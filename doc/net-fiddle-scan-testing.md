# net-fiddle-scan testing

We will create a test setup daemon that runs as root to setup and teardown
the test environments but the tests themselves do not have to be run as root.
This will make it easier to iterate on tests and also avoid issues with
accidentally running arbitrary commands as a root user.

## Test setup daemon

Test setup daemon will export an HTTP RESTful interface that allows for manipulation
of network namespaces, interfaces, routing, bpf, tc.

All mutating endpoints accept a JSON body and return a JSON response. On error,
the response body is `{"error": "<message>"}` with an appropriate 4xx/5xx status.

---

### Namespace management

`linux-host` is a special namespace name for the Linux host network namespace.
This resource always exists and cannot be deleted.

```
GET    /ns                          list all namespaces (name + inode)
POST   /ns/<name>                   create named namespace (ip netns add)
DELETE /ns/<name>                   delete named namespace (ip netns del)
```

---

### `ip` commands

Runs `ip <cmdline>` inside the named namespace via `nsenter`.

```
POST /ns/<name>/ip
```

```json
{ "cmdline": ["link", "add", "veth0", "type", "veth", "peer", "name", "veth1"] }
```

Covers all subcommands needed for test setup:

| Subcommand | Purpose |
|---|---|
| `link add ... type veth peer name ...` | Create a veth pair (both ends in same namespace initially) |
| `link add ... type netkit ...` | Create a netkit pair |
| `link set <iface> netns <name>` | Move one end of a pair to another namespace |
| `link set <iface> up` / `down` | Bring interface up or down |
| `link set <iface> xdp obj <path> sec <section>` | Attach an XDP program from an ELF object file |
| `link set <iface> xdp off` | Detach XDP |
| `link del <iface>` | Delete an interface |
| `addr add <prefix> dev <iface>` | Assign an IP address |
| `addr del <prefix> dev <iface>` | Remove an IP address |
| `route add <prefix> via <gw>` | Add a route |
| `route del <prefix>` | Remove a route |

---

### `tc` commands

Runs `tc <cmdline>` inside the named namespace.

```
POST /ns/<name>/tc
```

```json
{ "cmdline": ["qdisc", "add", "dev", "eth0", "clsact"] }
```

| Subcommand | Purpose |
|---|---|
| `qdisc add dev <iface> clsact` | Add clsact qdisc (enables TC ingress + egress hooks) |
| `qdisc add dev <iface> ingress` | Add legacy ingress-only qdisc |
| `qdisc add dev <iface> root fq` | Add an egress qdisc (fq, htb, fq_codel, tbf) |
| `qdisc del dev <iface> clsact` | Remove clsact qdisc |
| `filter add dev <iface> ingress bpf da obj <path> sec <section>` | Attach TC BPF program on ingress |
| `filter add dev <iface> egress bpf da obj <path> sec <section>` | Attach TC BPF program on egress |
| `filter del dev <iface> ingress` | Remove all ingress TC filters |
| `filter del dev <iface> egress` | Remove all egress TC filters |

---

### `nft` commands

Runs `nft <cmdline>` inside the named namespace. The most convenient form passes
a complete ruleset fragment via the `-f -` flag (reading from stdin), so the body
carries a `script` field instead of a raw cmdline.

```
POST /ns/<name>/nft
```

Two forms are supported:

**Cmdline form** — for simple one-shot operations:
```json
{ "cmdline": ["flush", "ruleset"] }
```

**Script form** — for multi-statement setup:
```json
{ "script": "table inet filter {\n  chain input { type filter hook input priority 0; }\n}" }
```

Common operations needed for tests:

| Operation | Purpose |
|---|---|
| `flush ruleset` | Remove all nftables state |
| `add table <family> <name>` | Create a table |
| `add chain ... { type filter hook <hook> priority 0; }` | Register a base chain at a hook |
| `add chain ... { type nat hook postrouting priority 100; }` | Register a nat chain |
| `delete table <family> <name>` | Remove a table and all its chains |

---

### BPF object management

Tests need pre-compiled BPF ELF objects to attach via `ip link set xdp` or
`tc filter add bpf`. The daemon serves a small library of built-in stub programs
that do nothing (pass-through) so tests can verify program nodes appear in the
topology without requiring a full BPF toolchain in the test environment.

```
GET /bpf/objects                    list available stub objects by name
GET /bpf/objects/<name>             download ELF object file (Content-Type: application/octet-stream)
```

Built-in stubs:

| Name | BPF type | Section | Behaviour |
|---|---|---|---|
| `xdp_pass.o` | XDP | `xdp` | Returns `XDP_PASS` |
| `tc_pass.o` | `sched_cls` | `tc` | Returns `TC_ACT_OK` |
| `sched_act_pass.o` | `sched_act` | `action` | Returns `TC_ACT_OK` |

The daemon builds these objects at startup from embedded C source using `clang`
if available. If `clang` is not available, the daemon falls back to pre-compiled
ELF blobs embedded in the binary at build time.

---

### Process management

Creates a long-running process inside a namespace that opens and holds sockets,
so that `SocketCollector` has something to discover.

```
POST   /ns/<name>/process           start a process, returns { "pid": <n> }
DELETE /ns/<name>/process/<pid>     kill a process by PID
GET    /ns/<name>/process           list running processes { "pids": [...] }
```

```json
POST /ns/<name>/process
{ "type": "tcp-listen", "port": 8080 }
```

Supported process types:

| Type | Fields | What it does |
|---|---|---|
| `tcp-listen` | `port` (int) | Opens a TCP socket, binds to `0.0.0.0:<port>`, calls `listen()`, holds it open |
| `udp-bind` | `port` (int) | Opens a UDP socket and binds to `0.0.0.0:<port>` |
| `tcp-connect` | `addr` (string), `port` (int) | Opens a TCP connection to `<addr>:<port>`; requires the target to be listening |

The daemon spawns a minimal helper binary (`netns-sockhold`) for each process
request. `comm` for these processes will be `netns-sockhold`, which is what
`SocketCollector` will see.

---

### Daemon lifecycle

```
GET /healthz                        returns 200 OK when daemon is ready
POST /reset                         kill all managed processes and flush all
                                    non-host namespaces; used in test teardown
```

---

## Client library

Tests import a thin Go package `internal/testdaemon` that wraps the HTTP API:

```go
type Client struct { BaseURL string }

func (c *Client) CreateNs(name string) error
func (c *Client) DeleteNs(name string) error
func (c *Client) IP(ns string, args ...string) error
func (c *Client) TC(ns string, args ...string) error
func (c *Client) NftScript(ns, script string) error
func (c *Client) NftCmd(ns string, args ...string) error
func (c *Client) AttachXDP(ns, iface, objectName, section string) error
func (c *Client) AttachTCBPF(ns, iface, direction, objectName, section string) error
func (c *Client) StartProcess(ns string, req ProcessRequest) (pid int, err error)
func (c *Client) KillProcess(ns string, pid int) error
func (c *Client) Reset() error
```

The daemon address defaults to `http://localhost:7777` and is overridden by the
`NETFIDDLE_DAEMON_ADDR` environment variable.

---

## Testing

Unit tests assume the test setup daemon is already running as root. Each test:

1. Calls `client.Reset()` in `TestMain` or a `t.Cleanup` to ensure a clean slate.
2. Uses `client.*` methods to build the desired network topology.
3. Runs the scanner collectors or linker directly (not via the CLI binary) against the constructed namespaces.
4. Asserts on the returned `NsSnapshot` or `BuildResult` structs.

Example test skeleton:

```go
func TestVethPairDetection(t *testing.T) {
    c := testdaemon.NewClient()
    t.Cleanup(func() { c.Reset() })

    c.CreateNs("ns-a")
    c.CreateNs("ns-b")
    c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
    c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b")
    c.IP("ns-a", "link", "set", "veth0", "up")
    c.IP("ns-b", "link", "set", "veth1", "up")

    snaps := collector.EnumerateAndCollect(false)
    pairs := linker.Link(snaps)

    // Expect exactly one pair linking ns-a/veth0 to ns-b/veth1
    ...
}
```
