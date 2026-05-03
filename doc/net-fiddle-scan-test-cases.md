# net-fiddle-scan Test Cases

Each test uses the test setup daemon client to build a network topology, then
calls the scanner internals directly (`collector`, `linker`, `builder`) and
asserts on the returned structs. Tests call `client.Reset()` in cleanup.

Test helper shorthand used in assertions:

- `snap(name)` — the `NsSnapshot` for the namespace named `name`
- `iface(snap, name)` — the `InterfaceInfo` in `snap` with `IfName == name`
- `node(result, id)` — the `topology.Node` with `ID == id`
- `edge(result, src, tgt)` — the `topology.Edge` with `Source == src && Target == tgt`
- `pairOf(pairs, ns, iface)` — the `PairInfo` for `(ns.Inode, iface.IfIndex)`

---

## 1. Namespace Enumeration

### TC-NS-01: Host namespace always present

**Setup:** none (daemon is running)

**Assert:**
- `EnumerateNamespaces()` returns at least one entry
- Exactly one entry has `Name == "host"`
- `host` entry has a non-zero `Inode`
- `host` entry `Path` resolves to `/proc/1/ns/net`

---

### TC-NS-02: Single named namespace

**Setup:**
```
CreateNs("ns-a")
```

**Assert:**
- Result contains an entry with `Name == "ns-a"`
- `ns-a` has a different `Inode` than `host`
- `ns-a` `Path == "/var/run/netns/ns-a"`

---

### TC-NS-03: Multiple named namespaces, no duplicates

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
CreateNs("ns-c")
```

**Assert:**
- Result contains entries for `host`, `ns-a`, `ns-b`, `ns-c` (at minimum)
- All inodes are distinct

---

### TC-NS-04: Namespace deleted before scan

**Setup:**
```
CreateNs("ns-gone")
DeleteNs("ns-gone")
```

**Assert:**
- Result does not contain any entry with `Name == "ns-gone"`

---

## 2. Interface Collection

### TC-IF-01: Empty namespace has no interfaces

**Setup:**
```
CreateNs("ns-empty")
```

**Assert:**
- `snap("ns-empty").Interfaces` is empty (loopback is filtered out)

---

### TC-IF-02: Single regular (ether) interface

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
IP("ns-a", "link", "set", "dummy0", "up")
```

**Assert:**
- `snap("ns-a").Interfaces` has exactly one entry
- `iface(snap, "dummy0").Kind == "ether"` (or "dummy" — whichever ip reports)
- `iface(snap, "dummy0").PeerIfIndex == 0`

---

### TC-IF-03: Interface flags reflect UP state

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
```

**Assert:** `iface(snap, "dummy0")` does not have `"UP"` in `Flags`

**Then:**
```
IP("ns-a", "link", "set", "dummy0", "up")
```

**Assert:** `iface(snap, "dummy0")` has `"UP"` in `Flags`

---

### TC-IF-04: Multiple interfaces in one namespace

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
IP("ns-a", "link", "add", "dummy1", "type", "dummy")
IP("ns-a", "link", "add", "dummy2", "type", "dummy")
```

**Assert:**
- `snap("ns-a").Interfaces` has exactly 3 entries
- Names are `dummy0`, `dummy1`, `dummy2`

---

## 3. Veth Pair Detection — Same Namespace

### TC-VETH-01: Veth pair created in one namespace

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
```

**Assert:**
- `snap("ns-a").Interfaces` has exactly 2 entries
- `iface(snap, "veth0").Kind == "veth"`
- `iface(snap, "veth1").Kind == "veth"`
- `iface(snap, "veth0").PeerIfIndex == iface(snap, "veth1").IfIndex`
- `iface(snap, "veth1").PeerIfIndex == iface(snap, "veth0").IfIndex`
- `iface(snap, "veth0").LinkNetNsID == nil` (peer is in same ns)
- `linker.Link(snaps)` assigns the same `PairID` to both ends
- `PairID` starts with `"veth-pair-"`

---

### TC-VETH-02: Veth pair linking produces exactly one pair entry

**Setup:** same as TC-VETH-01

**Assert:**
- `pairs` map has exactly 2 entries (one per end)
- `pairOf(pairs, "ns-a", "veth0").PairID == pairOf(pairs, "ns-a", "veth1").PairID`
- `pairOf(pairs, "ns-a", "veth0").Kind == "veth"`

---

## 4. Veth Pair Detection — Cross-Namespace

### TC-VETH-CROSS-01: Veth pair with peer moved to another namespace

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
```

**Assert:**
- `snap("ns-a").Interfaces` has exactly 1 entry: `veth0`
- `snap("ns-b").Interfaces` has exactly 1 entry: `veth1`
- `iface(snap("ns-a"), "veth0").PeerIfIndex == iface(snap("ns-b"), "veth1").IfIndex`
- `iface(snap("ns-a"), "veth0").LinkNetNsID != nil`
- `linker.Link(snaps)` assigns the same `PairID` to both ends
- `pairOf(pairs, "ns-a", "veth0").PairID == pairOf(pairs, "ns-b", "veth1").PairID`

---

### TC-VETH-CROSS-02: Two independent veth pairs across namespaces

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
CreateNs("ns-c")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
IP("ns-a", "link", "add", "veth2", "type", "veth", "peer", "name", "veth3")
IP("ns-a", "link", "set", "veth3", "netns", "ns-c")
```

**Assert:**
- `linker.Link(snaps)` produces exactly 4 pair-map entries (2 pairs × 2 ends each)
- `pairOf(pairs, "ns-a", "veth0").PairID != pairOf(pairs, "ns-a", "veth2").PairID`
- `pairOf(pairs, "ns-a", "veth0").PairID == pairOf(pairs, "ns-b", "veth1").PairID`
- `pairOf(pairs, "ns-a", "veth2").PairID == pairOf(pairs, "ns-c", "veth3").PairID`

---

### TC-VETH-CROSS-03: Veth pair between two non-host namespaces, both peers up

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "eth0", "type", "veth", "peer", "name", "eth0")
IP("ns-a", "link", "set", "eth0", "netns", "ns-b")  // moves the peer named eth0
IP("ns-a", "link", "set", "eth0", "up")
IP("ns-b", "link", "set", "eth0", "up")
IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "eth0")
IP("ns-b", "addr", "add", "10.0.0.2/24", "dev", "eth0")
```

**Assert:**
- Linker correctly pairs `ns-a/eth0` with `ns-b/eth0` despite both having the same interface name
- `builder.Build` emits a `pairLinkEdge` with `data.vethLink == true` connecting the two nodes
- The edge `className == "veth-link"`

---

### TC-VETH-CROSS-04: Ifindex collision — two unrelated namespaces share same ifindex values

This tests that the linker does not incorrectly pair interfaces just because they
have the same ifindex number in different namespaces.

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
CreateNs("ns-c")
// Create a veth pair spanning ns-a and ns-b
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
// Create an unrelated interface in ns-c that may have an overlapping ifindex
IP("ns-c", "link", "add", "dummy0", "type", "dummy")
```

**Assert:**
- Linker produces exactly 2 pair-map entries (one pair, two ends)
- `ns-c/dummy0` is not in the pairs map
- `pairOf(pairs, "ns-a", "veth0")` and `pairOf(pairs, "ns-b", "veth1")` have the same `PairID`

---

### TC-VETH-CROSS-05: Three-namespace chain (ns-a ↔ ns-b ↔ ns-c)

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
CreateNs("ns-c")
// ns-a ↔ ns-b
IP("ns-a", "link", "add", "veth-ab", "type", "veth", "peer", "name", "veth-ba")
IP("ns-a", "link", "set", "veth-ba", "netns", "ns-b")
// ns-b ↔ ns-c
IP("ns-b", "link", "add", "veth-bc", "type", "veth", "peer", "name", "veth-cb")
IP("ns-b", "link", "set", "veth-cb", "netns", "ns-c")
```

**Assert:**
- Linker produces 4 pair-map entries (2 pairs)
- `ns-b` has 2 interfaces: `veth-ba` and `veth-bc`
- Each is paired independently with its respective counterpart
- `builder.Build` emits 2 `pairLinkEdge` edges

---

## 5. Netkit Pair Detection — Cross-Namespace

### TC-NK-01: Netkit pair basic detection

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
IP("ns-a", "link", "set", "nk1", "netns", "ns-b")
```

**Assert:**
- `iface(snap("ns-a"), "nk0").Kind == "netkit"`
- `iface(snap("ns-b"), "nk1").Kind == "netkit"`
- `iface(snap("ns-a"), "nk0").NetkitMode != ""` (primary end has mode set)
- `iface(snap("ns-b"), "nk1").NetkitMode == ""` (peer end has no mode)
- Linker assigns the same `PairID` to both ends
- `PairID` starts with `"netkit-pair-"`
- `pairOf(pairs, "ns-a", "nk0").Kind == "netkit"`

---

### TC-NK-02: Netkit primary vs peer node type in builder

**Setup:** same as TC-NK-01

**Assert:**
- `node(result, "ns-a-nk0").Data.NodeType == "netkit-primary"`
- `node(result, "ns-b-nk1").Data.NodeType == "netkit-peer"`

This confirms that primary/peer assignment uses the `NetkitMode` field and not
iteration order, which is non-deterministic.

---

### TC-NK-03: Two netkit pairs, primary/peer not confused across pairs

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
CreateNs("ns-c")
IP("ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
IP("ns-a", "link", "set", "nk1", "netns", "ns-b")
IP("ns-a", "link", "add", "nk2", "type", "netkit", "mode", "l3", "peer", "name", "nk3")
IP("ns-a", "link", "set", "nk3", "netns", "ns-c")
```

**Assert:**
- `node(result, "ns-a-nk0").Data.NodeType == "netkit-primary"`
- `node(result, "ns-a-nk2").Data.NodeType == "netkit-primary"`
- `node(result, "ns-b-nk1").Data.NodeType == "netkit-peer"`
- `node(result, "ns-c-nk3").Data.NodeType == "netkit-peer"`
- The two pairs have different `PairID` values

---

### TC-NK-04: Mixed veth and netkit pairs in the same topology

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
IP("ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
IP("ns-a", "link", "set", "nk1", "netns", "ns-b")
```

**Assert:**
- Linker produces 4 entries: 2 pairs, each with 2 ends
- Veth pair `PairID` starts with `"veth-pair-"`, netkit pair starts with `"netkit-pair-"`
- `node(result, "ns-a-veth0").Data.NodeType == "veth-end"`
- `node(result, "ns-a-nk0").Data.NodeType == "netkit-primary"`
- Builder emits exactly 2 `pairLinkEdge` edges

---

## 6. nftables Hook Detection

### TC-NFT-01: Namespace with no nftables rules

**Setup:**
```
CreateNs("ns-a")
```

**Assert:**
- `snap("ns-a").NftHooks` is empty

---

### TC-NFT-02: Single hook registered

**Setup:**
```
CreateNs("ns-a")
NftScript("ns-a", `
  table inet filter {
    chain input { type filter hook input priority 0; }
  }
`)
```

**Assert:**
- `snap("ns-a").NftHooks` has exactly 1 entry
- Entry has `Hook == "input"` and `Tables` contains `"filter"`

---

### TC-NFT-03: Multiple hooks from one table

**Setup:**
```
CreateNs("ns-a")
NftScript("ns-a", `
  table inet filter {
    chain pre  { type filter hook prerouting  priority 0; }
    chain in   { type filter hook input       priority 0; }
    chain fwd  { type filter hook forward     priority 0; }
    chain out  { type filter hook output      priority 0; }
    chain post { type filter hook postrouting priority 0; }
  }
`)
```

**Assert:**
- `snap("ns-a").NftHooks` has exactly 5 entries
- Hooks present: `prerouting`, `input`, `forward`, `output`, `postrouting`

---

### TC-NFT-04: Multiple tables at the same hook collapsed to one node

**Setup:**
```
CreateNs("ns-a")
NftScript("ns-a", `
  table inet filter { chain in { type filter hook input priority 0; } }
  table inet mangle { chain in { type filter hook input priority -50; } }
`)
```

**Assert:**
- `snap("ns-a").NftHooks` has exactly 1 entry (hook `"input"`)
- Entry `Tables` contains both `"filter"` and `"mangle"`
- `builder.Build` emits exactly one `nftables-input` node for `ns-a`

---

### TC-NFT-05: Bridge and netdev chains are ignored

**Setup:**
```
CreateNs("ns-a")
NftScript("ns-a", `
  table inet filter { chain in { type filter hook input priority 0; } }
  table bridge btable { chain pre { type filter hook prerouting priority 0; } }
`)
```

**Assert:**
- `snap("ns-a").NftHooks` has exactly 1 entry: `input`
- No `bridge`-family hook appears in the result

---

### TC-NFT-06: NAT table at postrouting and prerouting

**Setup:**
```
CreateNs("ns-a")
NftScript("ns-a", `
  table inet nat {
    chain pre  { type nat hook prerouting  priority -100; }
    chain post { type nat hook postrouting priority  100; }
  }
`)
```

**Assert:**
- `snap("ns-a").NftHooks` has 2 entries: `prerouting` and `postrouting`
- Both have `Tables == ["nat"]`

---

## 7. TC Qdisc Detection

### TC-TC-01: No qdiscs (beyond default noqueue)

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
```

**Assert:**
- `snap("ns-a").Qdiscs` is empty (noqueue is filtered)

---

### TC-TC-02: clsact qdisc signals TC hooks

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
```

**Assert:**
- `snap("ns-a").Qdiscs` has exactly 1 entry: `{IfName: "veth0", Kind: "clsact"}`
- `builder.Build` emits a `traffic-control` node with `config.direction == "ingress"` for `veth0`
- `builder.Build` emits a `traffic-control` node with `config.direction == "egress"` for `veth0`

---

### TC-TC-03: Egress qdisc (fq)

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "fq")
```

**Assert:**
- `snap("ns-a").Qdiscs` contains `{IfName: "veth0", Kind: "fq"}`
- `builder.Build` emits a `qdisc` node with `config.type == "fq"`

---

### TC-TC-04: Multiple interfaces, qdiscs attributed correctly

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "add", "veth2", "type", "veth", "peer", "name", "veth3")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
TC("ns-a", "qdisc", "add", "dev", "veth2", "root", "fq")
```

**Assert:**
- `snap("ns-a").Qdiscs` has 2 entries, each attributed to the correct interface
- `qdisc` node for `veth2` has `config.type == "fq"`
- `traffic-control` nodes are only emitted for `veth0`, not `veth2`

---

## 8. BPF Program Detection

*Tests in this section require clang to be present. If `ListBpfObjects()` returns an empty
list, skip with `t.Skip("clang not available; BPF stubs not built")`.*

### TC-BPF-01: XDP program attached to interface

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp")
```

**Assert:**
- `snap("ns-a").BpfAttach` has 1 entry with `IfName == "veth0"` and `AttachType == "xdp"`
- `builder.Build` emits an `xdp-program` node as a child of `ns-a`
- The ingress chain for `veth0` includes `veth0 → xdp-program → ...`

---

### TC-BPF-02: TC BPF filter on ingress

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc")
```

**Assert:**
- `snap("ns-a").BpfAttach` has 1 entry with `AttachType == "ingress"`
- `builder.Build` emits a `tc-bpf-program` node with `config.direction == "ingress"`
- The node appears between the `traffic-control` ingress node and `nftables-prerouting` in the chain

---

### TC-BPF-03: TC BPF filter on egress

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
AttachTCBPF("ns-a", "veth0", "egress", "tc_pass.o", "tc")
```

**Assert:**
- `snap("ns-a").BpfAttach` has 1 entry with `AttachType == "egress"`
- `builder.Build` emits a `tc-bpf-program` node with `config.direction == "egress"`

---

### TC-BPF-04: BPF program without clsact → sched-bpf node

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
// Attach TC BPF without adding clsact first (sched_cls on the root qdisc)
TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "handle", "1:", "fq")
TC("ns-a", "filter", "add", "dev", "veth0", "parent", "1:", "bpf", "da", "obj", "<tc_pass_path>", "sec", "tc")
```

*Note: this scenario requires the TC filter to be attached without a clsact qdisc.
If the kernel reports this as an egress TC attachment without clsact, the builder
re-classifies it as `sched-bpf`.*

**Assert:**
- `builder.Build` emits a `sched-bpf` node (not `tc-bpf-program`) for this attachment
- No `traffic-control` egress node is emitted for `veth0`

---

### TC-BPF-05: XDP and TC BPF on the same interface

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp")
AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc")
```

**Assert:**
- `snap("ns-a").BpfAttach` has 2 entries
- Builder emits both an `xdp-program` node and a `tc-bpf-program` ingress node
- Ingress chain order: `veth0 → xdp-program → tc-bpf-program → traffic-control → ...`

---

## 9. Route Detection

### TC-RT-01: Namespace with no routes has no routing-table node

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
```

**Assert:**
- `snap("ns-a").HasRoutes == false`
- `builder.Build` does not emit a `routing-table` node for `ns-a`

---

### TC-RT-02: Namespace with a route has routing-table node

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
IP("ns-a", "link", "set", "dummy0", "up")
IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "dummy0")
IP("ns-a", "route", "add", "10.1.0.0/24", "via", "10.0.0.2")
```

**Assert:**
- `snap("ns-a").HasRoutes == true`
- `builder.Build` emits a `routing-table` node for `ns-a`
- The routing node appears in the ingress chain: `... → nft-prerouting → routing-table → nft-input → ...`

---

## 10. Socket Detection

### TC-SOCK-01: No sockets in namespace

**Setup:**
```
CreateNs("ns-a")
```

**Assert:**
- `snap("ns-a").Sockets` is empty

---

### TC-SOCK-02: TCP listening socket

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
IP("ns-a", "link", "set", "dummy0", "up")
IP("ns-a", "addr", "add", "127.0.0.1/8", "dev", "dummy0")
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 9000})
```

**Assert:**
- `snap("ns-a").Sockets` has exactly 1 entry
- Entry `PID > 0`
- `builder.Build` emits a `socket` node with `label == "testdaemon"` (the comm of the helper)

---

### TC-SOCK-03: UDP bound socket

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "dummy0", "type", "dummy")
IP("ns-a", "link", "set", "dummy0", "up")
StartProcess("ns-a", ProcessRequest{Type: "udp-bind", Port: 9001})
```

**Assert:**
- `snap("ns-a").Sockets` has 1 entry (UDP socket in ESTABLISHED/CONNECTED state)

---

### TC-SOCK-04: Two processes with sockets produce two socket nodes

**Setup:**
```
CreateNs("ns-a")
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 9000})
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 9001})
```

**Assert:**
- `snap("ns-a").Sockets` has 2 entries with distinct `PID` values
- `builder.Build` emits 2 `socket` nodes

---

### TC-SOCK-05: --no-sockets flag suppresses socket collection

**Setup:**
```
CreateNs("ns-a")
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 9000})
```

**Collect with** `noSockets = true`

**Assert:**
- `snap("ns-a").Sockets` is empty
- `builder.Build` emits no `socket` nodes

---

### TC-SOCK-06: TCP connection between two namespaces

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
IP("ns-a", "link", "set", "veth0", "up")
IP("ns-b", "link", "set", "veth1", "up")
IP("ns-a", "addr", "add", "192.168.1.1/24", "dev", "veth0")
IP("ns-b", "addr", "add", "192.168.1.2/24", "dev", "veth1")
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 8080})
StartProcess("ns-b", ProcessRequest{Type: "tcp-connect", Addr: "192.168.1.1", Port: 8080})
```

**Assert:**
- `snap("ns-a").Sockets` has at least 1 entry (LISTEN or ESTABLISHED)
- `snap("ns-b").Sockets` has at least 1 entry (ESTABLISHED)
- Builder emits `socket` nodes in both namespaces

---

## 11. Edge Chain Correctness

### TC-EDGE-01: Ingress chain ordering without BPF or nftables

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0")
// ip addr add creates a route automatically
```

**Assert:**
- Ingress edges exist: `ns-a-veth0 → ns-a-routing`
- No edge to `nftables-prerouting` (not configured)

---

### TC-EDGE-02: Full ingress chain with nftables and routing

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0")
NftScript("ns-a", `
  table inet filter {
    chain pre  { type filter hook prerouting priority 0; }
    chain in   { type filter hook input      priority 0; }
  }
`)
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 8080})
```

**Assert:**
- Edge sequence exists: `veth0 → nft-prerouting → routing-table → nft-input → socket`
- Source/target handle IDs follow the convention: source handles end in `-s`, target handles end in `-t`

---

### TC-EDGE-03: Egress chain with TC and qdisc

**Setup:**
```
CreateNs("ns-a")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth0", "up")
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "fq")
NftScript("ns-a", `
  table inet filter {
    chain out  { type filter hook output      priority 0; }
    chain post { type filter hook postrouting priority 100; }
  }
`)
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 8080})
```

**Assert:**
- Egress edges: `socket → nft-output → nft-postrouting → tc-egress → qdisc → veth0`

---

### TC-EDGE-04: Pair-link edge attributes

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
```

**Assert:**
- `builder.Build` emits exactly 1 edge with `Data.VethLink == true`
- That edge has `ClassName == "veth-link"`
- `SourceHandle == "S-0-s"` and `TargetHandle == "S-0-t"`

---

## 12. Complex / Combined Scenarios

### TC-COMPLEX-01: Two-namespace routing topology (ns-a ↔ host ↔ ns-b)

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
// ns-a ↔ host
IP("linux-host", "link", "add", "veth-ha", "type", "veth", "peer", "name", "veth-ah")
IP("linux-host", "link", "set", "veth-ah", "netns", "ns-a")
IP("linux-host", "link", "set", "veth-ha", "up")
IP("ns-a", "link", "set", "veth-ah", "up")
IP("linux-host", "addr", "add", "10.0.1.1/24", "dev", "veth-ha")
IP("ns-a", "addr", "add", "10.0.1.2/24", "dev", "veth-ah")
// ns-b ↔ host
IP("linux-host", "link", "add", "veth-hb", "type", "veth", "peer", "name", "veth-bh")
IP("linux-host", "link", "set", "veth-bh", "netns", "ns-b")
IP("linux-host", "link", "set", "veth-hb", "up")
IP("ns-b", "link", "set", "veth-bh", "up")
IP("linux-host", "addr", "add", "10.0.2.1/24", "dev", "veth-hb")
IP("ns-b", "addr", "add", "10.0.2.2/24", "dev", "veth-bh")
// routes
IP("ns-a", "route", "add", "default", "via", "10.0.1.1")
IP("ns-b", "route", "add", "default", "via", "10.0.2.1")
```

**Assert:**
- `linker.Link` identifies 2 pairs: `host/veth-ha ↔ ns-a/veth-ah` and `host/veth-hb ↔ ns-b/veth-bh`
- All 3 namespaces have `HasRoutes == true`
- `builder.Build` emits 3 namespace container nodes and 2 pair-link edges
- Layout assigns each namespace to a distinct column (non-overlapping x positions)

---

### TC-COMPLEX-02: Full stack — nftables + TC + BPF + socket + routing + veth pair

*Requires clang.*

**Setup:**
```
CreateNs("ns-a")
CreateNs("ns-b")
// veth pair
IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
IP("ns-a", "link", "set", "veth0", "up")
IP("ns-b", "link", "set", "veth1", "up")
IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0")
IP("ns-b", "addr", "add", "10.0.0.2/24", "dev", "veth1")
// nftables in ns-a
NftScript("ns-a", `
  table inet filter {
    chain pre  { type filter hook prerouting  priority 0; }
    chain in   { type filter hook input       priority 0; }
    chain out  { type filter hook output      priority 0; }
    chain post { type filter hook postrouting priority 100; }
  }
`)
// TC in ns-a
TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
// XDP on ns-a/veth0
AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp")
// TC BPF ingress on ns-a/veth0
AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc")
// socket in ns-a
StartProcess("ns-a", ProcessRequest{Type: "tcp-listen", Port: 8080})
```

**Assert:**
- `snap("ns-a").Interfaces` has 1 entry: `veth0`
- `snap("ns-b").Interfaces` has 1 entry: `veth1`
- `snap("ns-a").BpfAttach` has 2 entries: XDP and TC ingress
- `snap("ns-a").NftHooks` has 4 entries
- `snap("ns-a").HasRoutes == true`
- `snap("ns-a").Sockets` has 1 entry
- Builder node types in ns-a: `veth-end`, `xdp-program`, `tc-bpf-program` (ingress), `traffic-control` (ingress), `traffic-control` (egress), `nftables-prerouting`, `nftables-input`, `nftables-output`, `nftables-postrouting`, `routing-table`, `socket`
- Builder node types in ns-b: `veth-end` (as netkit-peer if netkit, else veth-end)
- 1 pair-link edge connects `ns-a/veth0` to `ns-b/veth1`
- Ingress chain for ns-a/veth0: `veth0 → xdp-program → tc-bpf-program → tc-ingress → nft-prerouting → routing → nft-input → socket`
- Egress chain for ns-a/veth0: `socket → nft-output → nft-postrouting → tc-egress → veth0`
- JSON output validates against `doc/topology-spec.json`

---

### TC-COMPLEX-03: Large topology — 5 namespaces, mixed pair types

**Setup:**
```
CreateNs("ns-a") through CreateNs("ns-e")
// ns-a ↔ ns-b via veth
// ns-b ↔ ns-c via netkit
// ns-c ↔ ns-d via veth
// ns-d ↔ ns-e via netkit
// Each namespace has nftables input hook
// ns-a and ns-e each have a listening socket
```

**Assert:**
- Linker identifies exactly 4 pairs with correct types (veth/netkit alternating)
- No cross-contamination: each pair contains exactly the two expected endpoints
- Builder emits 5 namespace container nodes
- All 8 pair ends have the correct `nodeType` (`veth-end`, `netkit-primary`, or `netkit-peer`)
- Layout: all 5 namespace containers have distinct, non-overlapping x positions
- No node has a position that places it outside its parent container bounds

---

### TC-COMPLEX-04: Scan output round-trips through topology spec

**Setup:** any topology with at least one of each node type

**Assert:**
- The JSON output of `net-fiddle-scan --pretty` parses without error
- The parsed JSON satisfies all constraints in `doc/topology-spec.json`:
  - Every `netNode.data.nodeType` is in the allowed enum
  - Every edge `sourceHandle` ends in `-s`
  - Every edge `targetHandle` ends in `-t`
  - `containerNode` children appear after their parent in the `nodes` array
  - Every `pairLinkEdge` has `data.vethLink == true` and `className == "veth-link"`
