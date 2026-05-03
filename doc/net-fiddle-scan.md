# net-fiddle-scan Design

A standalone command-line tool that inspects a live Linux environment and emits a Net Fiddle topology JSON file suitable for direct import into the webapp.

---

## Goals

- Enumerate every network namespace on the host
- Within each namespace, discover the active packet-processing hooks in the order the kernel visits them
- Produce a topology JSON that, when imported, visually represents the real dataplane without manual diagramming
- Require no kernel patches or eBPF programs of its own — read-only inspection only

Out of scope for v1: bridge/L2 paths, WireGuard/IPsec tunnels, multi-table routing policy, tc classful qdiscs (HTB class trees).

---

## Prerequisites

- Root (or `CAP_SYS_ADMIN` + `CAP_NET_ADMIN` + `CAP_SYS_PTRACE`)
- Kernel ≥ 5.10 (for `bpftool net` JSON output)
- Userspace tools present: `ip`, `bpftool`, `nft`, `tc`, `ss`
- Python ≥ 3.9 (implementation language)

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│ NsEnumerator                                                  │
│   discovers all network namespaces (named + per-process)      │
└────────────────────────────┬─────────────────────────────────┘
                             │  one NsInfo per namespace
                             ▼
┌──────────────────────────────────────────────────────────────┐
│ NsCollector  (runs once per namespace via nsenter)            │
│   • InterfaceCollector   — ip -j link show                   │
│   • BpfCollector         — bpftool -j net show               │
│   • NftCollector         — nft -j list ruleset               │
│   • TcCollector          — tc -j qdisc/filter show           │
│   • RouteCollector       — ip -j route show table all        │
│   • SocketCollector      — ss -j -tunap                      │
└────────────────────────────┬─────────────────────────────────┘
                             │  raw NsSnapshot per namespace
                             ▼
┌──────────────────────────────────────────────────────────────┐
│ PairLinker                                                    │
│   correlates veth/netkit peers across namespace snapshots     │
└────────────────────────────┬─────────────────────────────────┘
                             │  cross-ns peer map
                             ▼
┌──────────────────────────────────────────────────────────────┐
│ TopologyBuilder                                               │
│   converts snapshots + peer map into nodes and edges          │
└────────────────────────────┬─────────────────────────────────┘
                             │  nodes[], edges[] (logical graph)
                             ▼
┌──────────────────────────────────────────────────────────────┐
│ Layouter                                                      │
│   assigns x/y positions: one column per namespace,            │
│   nodes ordered top-to-bottom by packet processing step       │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────┐
│ Emitter                                                       │
│   serialises to topology JSON (stdout or --output FILE)       │
└──────────────────────────────────────────────────────────────┘
```

---

## Data Collection

### Namespace enumeration — `NsEnumerator`

Two sources are merged, de-duplicated by inode number (`/proc/self/ns/net` inode serves as the canonical namespace id):

1. **Named namespaces** — `ip netns list` (backed by bind-mounts under `/var/run/netns/`). These have a human-readable name.
2. **Process namespaces** — scan `/proc/*/ns/net` symlinks. For each unique inode not already found via (1), record the PID and derive a name from `/proc/<pid>/comm` (e.g. `ns-containerd-1234`).

The host default namespace (`/proc/1/ns/net` or the scanner's own `net` namespace) is always included, named `"host"`.

Execution model: all subsequent collectors are invoked via `nsenter --net=/proc/<pid>/ns/net` or `nsenter --net=/var/run/netns/<name>` to run inside the target namespace.

---

### Interface collection — `InterfaceCollector`

Command: `ip -j link show`

For each interface:

| Attribute | Source field | Notes |
|---|---|---|
| Name | `ifname` | |
| Type | `link_type` | `"ether"` → `interface`; `"veth"` → `veth-end`; detected separately for netkit |
| Peer ifindex | `linkinfo.info_slave_data.peer_ifindex` (for veth) | Used by `PairLinker` |
| Peer netns id | `link_netnsid` | Identifies which namespace holds the peer |
| Flags | `flags` | `UP`, `LOWER_UP`, etc. |

**Netkit detection**: `ip -j -d link show type netkit` — netkit interfaces report `link_type: "netkit"` in `-d` (details) output. The primary end has `mode: "l3"` or `mode: "l2"` and a `peer_ifindex`; the peer end has a matching `peer_ifindex` in the peer namespace.

Interfaces of type `loopback` and `dummy` are skipped.

---

### BPF attachment collection — `BpfCollector`

Command: `bpftool -j net show`

Produces a per-interface list of attached programs in three categories:

| bpftool key | Meaning | Net Fiddle node type |
|---|---|---|
| `xdp` | XDP program | `xdp-program` |
| `tc` with `attach_type: "ingress"` | TC clsact ingress | `tc-bpf-program` (config direction: ingress) |
| `tc` with `attach_type: "egress"` | TC clsact egress | `tc-bpf-program` (config direction: egress) |
| `flow_dissector` | Skipped in v1 | — |

Each entry includes the BPF program `id`; enrich with `bpftool -j prog show id <id>` to get the `name` for the node label.

---

### nftables collection — `NftCollector`

Command: `nft -j list ruleset`

nftables organises rules into tables → chains → rules. The scanner cares only about **base chains** (chains with a `hook` field), which are the points where the kernel calls into nftables. One net-fiddle node is emitted per distinct hook that has at least one base chain:

| nft hook | Net Fiddle node type |
|---|---|
| `prerouting` | `nftables-prerouting` |
| `input` | `nftables-input` |
| `forward` | `nftables-forward` |
| `output` | `nftables-output` |
| `postrouting` | `nftables-postrouting` |

If multiple tables have base chains for the same hook, they are collapsed into a single node. The node label is derived from the table names (e.g. `nft:filter+nat`).

Chains of type `ip`, `ip6`, and `inet` are included. `bridge` and `netdev` chains are skipped in v1.

---

### TC qdisc collection — `TcCollector`

Command: `tc -j qdisc show dev <ifname>` for each interface

Relevant qdiscs:

| qdisc kind | Action |
|---|---|
| `clsact` | Signals that TC ingress and/or egress hooks may exist; confirmed by `BpfCollector`. Emit a `traffic-control` node. |
| `ingress` | Legacy ingress-only qdisc; emit a `traffic-control` node with direction ingress. |
| `fq`, `htb`, `fq_codel`, `tbf`, `pfifo_fast`, `mq`, `noqueue` | Emit a `qdisc` node. `noqueue` and `mq` are skipped. |

The egress-path TC hook (sched_cls egress / `sched-bpf`) is detected via `BpfCollector` attach_type `egress`. If a `sched-bpf` attach_type is found without a `clsact` qdisc, still emit a `sched-bpf` node.

---

### Routing table collection — `RouteCollector`

Command: `ip -j route show table all`

If at least one route exists (beyond the local table auto-populated by the kernel), emit a single `routing-table` node per namespace. Label: `"routing"`.

The routing table is always present on any namespace that has active interfaces, so this node appears in almost every namespace.

---

### Socket collection — `SocketCollector`

Command: `ss -j -tunap`

Aggregate by process: one `socket` node per unique `(comm, pid)` pair that has at least one established or listening TCP/UDP socket. Node label: process `comm` name (e.g. `nginx`, `sshd`). If a pid has multiple processes sharing the same socket (socket-activated services), use the comm of pid 1's child.

Sockets in `TIME_WAIT` or `CLOSE_WAIT` are skipped; only `LISTEN`, `ESTABLISHED`, and `CONNECTED` states are included.

---

## Cross-Namespace Pair Linking — `PairLinker`

Veth and netkit interfaces exist in two namespaces. After all `NsCollector` runs complete:

1. Build a map: `(ns_inode, ifindex) → InterfaceInfo`
2. For each interface with a `peer_ifindex` and `link_netnsid`, resolve the peer namespace inode via `/proc/<pid>/net/if_inet6` or the nsid→inode table from `ip -j netns list` (which maps nsid to inode for the current namespace).
3. Link the two ends: assign a shared `vethPairId` (`"veth-pair-N"` or `"netkit-pair-N"`).

If a peer namespace is not enumerated (e.g. a container that exited between scans), the interface is treated as a plain `interface` node with a note in stderr.

---

## Topology Building — `TopologyBuilder`

### Nodes

For each namespace, emit a `containerNode` of type `namespace`. Then emit child `netNode`s for each discovered component, with `parentNode` set to the namespace node's id.

For components attached to a specific interface (XDP programs, TC hooks, qdiscs), the `ifname` determines which interface they belong to. They appear as separate nodes connected by edges.

Node id scheme (stable across re-scans for the same host):
- Namespace: `ns-<name>` (e.g. `ns-host`, `ns-ns0`)
- Interface: `<ns-name>-<ifname>` (e.g. `ns-host-eth0`)
- BPF program: `<ns-name>-<ifname>-xdp-<prog_id>`
- TC hook: `<ns-name>-<ifname>-tc-<direction>`
- nftables hook: `<ns-name>-nft-<hook>`
- Routing table: `<ns-name>-routing`
- Qdisc: `<ns-name>-<ifname>-qdisc`
- Socket: `<ns-name>-sock-<comm>-<pid>`

### Edges

Edges are inferred from the Linux packet processing order rather than any explicit configuration. For each interface, two chains are built:

**Ingress chain** (components present are connected in order):
```
interface → [xdp-program] → [tc-bpf-program:ingress] → [nftables-prerouting]
  → [routing-table] → [nftables-input | nftables-forward] → [socket]
```

**Egress chain**:
```
[socket] → [nftables-output] → [nftables-postrouting]
  → [tc-bpf-program:egress | sched-bpf] → [qdisc] → interface
```

Components in brackets are only included if discovered. Edges use the source (`-s`) handle of the upstream node and the target (`-t`) handle of the downstream node at their default sides (the handle IDs are assigned as if the nodes were freshly dropped onto the canvas, i.e., `N-0-s`, `S-0-t`, etc.).

nftables nodes are shared between ingress and egress within a namespace — a single `nftables-output` node can receive edges from multiple sockets if multiple sockets are discovered.

**Pair-link edges**: veth and netkit pairs get a `pairLinkEdge` (`data.vethLink: true`) connecting the two ends, matching the format created by the drag-and-drop UI.

---

## Layout — `Layouter`

Assigns `position` (x, y) to each node. No general-purpose graph layout algorithm is used; positions are computed from the topology structure directly.

### Per-namespace column

Each namespace occupies a vertical column. Column width: 300px. Horizontal spacing between columns: 120px.

Column x-offset: `col_index * (300 + 120) + 40`

The namespace `containerNode` encloses all its children. Its width is 280px. Its height is computed after child positions are assigned: `max_child_y + child_height + 40px`.

### Per-interface vertical stack

Within a namespace, each interface generates two vertical stacks side by side: ingress (left half, x ≈ 30) and egress (right half, x ≈ 150). Nodes in each stack are spaced 90px apart vertically.

If a namespace has multiple interfaces, each interface pair of stacks is offset further right, tiled horizontally.

Shared nodes (nftables hooks, routing table, socket) are placed in the centre between the ingress and egress columns and connected to both. Vertical position follows their ingress ordering step.

### Example layout for a single-interface namespace

```
 30px  ┌─ namespace ──────────────────────────┐
       │  [interface]          [socket]        │
       │  [xdp-program]        [nft-output]    │
       │  [tc:ingress]         [nft-postrouting]│
       │  [nft-prerouting]     [tc:egress]     │
       │  [routing-table]      [qdisc]         │
       │  [nft-input]          [interface] ←── tied together
       └──────────────────────────────────────-┘
```

---

## CLI Interface

```
Usage: net-fiddle-scan [OPTIONS]

Options:
  --output FILE       Write topology JSON to FILE instead of stdout
  --namespace NAME    Scan only the named namespace (can be repeated)
  --exclude-ns NAME   Skip a namespace by name (can be repeated)
  --no-sockets        Omit socket nodes (useful when many processes exist)
  --pretty            Pretty-print the JSON output (default: compact)
  --verbose           Print per-namespace collection summary to stderr
  --version           Show version
```

Example:
```bash
sudo net-fiddle-scan --pretty --output topology.json
# then import topology.json into the Net Fiddle webapp
```

---

## Output Conformance

The emitted JSON must validate against `doc/topology-spec.json`. Specifically:

- Every `netNode` must have a `nodeType` in the enum
- Every edge `sourceHandle` must end in `-s`; every `targetHandle` must end in `-t`
- `containerNode` children must follow their parent in the `nodes` array
- `pairLinkEdge` must include `data.vethLink: true` and `className: "veth-link"`

---

## Limitations and Known Gaps

| Scenario | v1 behaviour |
|---|---|
| Netkit pair with peer in exited namespace | Peer end emitted as plain `interface` |
| Multiple nftables tables at same hook | Collapsed to one node; individual table names in label |
| TC classful qdiscs (HTB class trees) | Emitted as single `qdisc` node; class hierarchy not represented |
| `sched-bpf` (BPF_PROG_TYPE_SCHED_ACT) | Detected via bpftool; emitted as `sched-bpf` node |
| XDP in offload or generic mode | Detected and emitted; mode is not shown |
| Interfaces in multiple VRFs | All routes collapsed to one `routing-table` node |
| Docker/Podman overlay networks | veth pairs are detected; overlay encapsulation not represented |
| IPv6-only namespaces | Fully supported; `ss` and `nft` are address-family-agnostic |
| Kernel < 5.10 (no bpftool net JSON) | Fall back to parsing `bpftool net show` text output; less reliable |
| Concurrent namespace creation during scan | Namespace may be missing from output; re-run to capture |
