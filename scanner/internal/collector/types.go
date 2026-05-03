package collector

// NsInfo describes a discovered network namespace.
type NsInfo struct {
	Name  string // human-readable
	Path  string // /var/run/netns/<name> or /proc/<pid>/ns/net
	Inode uint64
	PID   int // 0 for named namespaces
}

// InterfaceInfo holds per-interface data from ip -j link show.
type InterfaceInfo struct {
	IfIndex     int
	IfName      string
	Kind        string // "ether", "veth", "netkit", "loopback", etc.
	PeerIfIndex int    // 0 if not a pair type
	Flags       []string
}

// BpfAttachment holds one BPF program attached to an interface.
type BpfAttachment struct {
	IfName     string
	ProgID     int
	ProgName   string
	AttachType string // "xdp", "ingress", "egress"
}

// NftHook is one active nftables hook in the namespace.
type NftHook struct {
	Hook   string   // "prerouting", "input", "forward", "output", "postrouting"
	Tables []string // table names that register chains at this hook
}

// QdiscInfo holds a TC qdisc for one interface.
type QdiscInfo struct {
	IfName string
	Kind   string // "clsact", "ingress", "fq", "htb", etc.
}

// SocketInfo is one process with active sockets.
type SocketInfo struct {
	Comm string
	PID  int
}

// NsSnapshot is the complete collected data for one namespace.
type NsSnapshot struct {
	Ns         NsInfo
	Interfaces []InterfaceInfo
	BpfAttach  []BpfAttachment
	NftHooks   []NftHook
	Qdiscs     []QdiscInfo
	HasRoutes  bool
	Sockets    []SocketInfo
}
