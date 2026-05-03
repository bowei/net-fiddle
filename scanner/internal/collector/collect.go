package collector

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"
)

// runInNs runs a command inside the given network namespace.
func runInNs(nsPath string, args ...string) ([]byte, error) {
	argv := append([]string{"--net=" + nsPath, "--preserve-credentials", "--"}, args...)
	cmd := exec.Command("nsenter", argv...)
	return cmd.Output()
}

// CollectAll gathers all data for one namespace.
func CollectAll(ns NsInfo, noSockets bool) NsSnapshot {
	snap := NsSnapshot{Ns: ns}
	snap.Interfaces = collectInterfaces(ns.Path)
	snap.BpfAttach = collectBpf(ns.Path, snap.Interfaces)
	snap.NftHooks = collectNft(ns.Path)
	snap.Qdiscs = collectQdiscs(ns.Path, snap.Interfaces)
	snap.HasRoutes = collectRoutes(ns.Path)
	snap.NsIDMap = collectNsIDMap(ns.Path)
	if !noSockets {
		snap.Sockets = collectSockets(ns.Path)
	}
	return snap
}

// ---- interface collection ----

type ipLinkEntry struct {
	IfIndex     int      `json:"ifindex"`
	IfName      string   `json:"ifname"`
	LinkType    string   `json:"link_type"`
	Flags       []string `json:"flags"`
	LinkNetNsID *int     `json:"link_netnsid"`
	LinkInfo    *struct {
		InfoKind string `json:"info_kind"`
		InfoData *struct {
			PeerIfIndex int    `json:"peer_ifindex"`
			Mode        string `json:"mode"`
			Peer        *struct {
				IfIndex int `json:"ifindex"`
			} `json:"peer"`
		} `json:"info_data"`
	} `json:"linkinfo"`
}

func collectInterfaces(nsPath string) []InterfaceInfo {
	out, err := runInNs(nsPath, "ip", "-j", "-d", "link", "show")
	if err != nil {
		log.Printf("ip link show in %s: %v", nsPath, err)
		return nil
	}

	var entries []ipLinkEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		log.Printf("ip link parse in %s: %v", nsPath, err)
		return nil
	}

	var result []InterfaceInfo
	for _, e := range entries {
		lt := e.LinkType
		if lt == "loopback" || lt == "dummy" || lt == "none" {
			continue
		}

		kind := lt
		switch lt {
		case "ether":
			kind = "ether"
		case "veth":
			kind = "veth"
		}

		var peerIfIndex int
		var netkitMode string
		if e.LinkInfo != nil {
			if e.LinkInfo.InfoKind == "netkit" {
				kind = "netkit"
			}
			if e.LinkInfo.InfoData != nil {
				if e.LinkInfo.InfoData.PeerIfIndex != 0 {
					peerIfIndex = e.LinkInfo.InfoData.PeerIfIndex
				} else if e.LinkInfo.InfoData.Peer != nil {
					peerIfIndex = e.LinkInfo.InfoData.Peer.IfIndex
				}
				netkitMode = e.LinkInfo.InfoData.Mode
			}
		}

		result = append(result, InterfaceInfo{
			IfIndex:     e.IfIndex,
			IfName:      e.IfName,
			Kind:        kind,
			PeerIfIndex: peerIfIndex,
			LinkNetNsID: e.LinkNetNsID,
			NetkitMode:  netkitMode,
			Flags:       e.Flags,
		})
	}
	return result
}

// ---- BPF collection ----

type bpftoolNetEntry struct {
	IfIndex int    `json:"ifindex"`
	IfName  string `json:"ifname"`
	XDP     []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"xdp"`
	TC []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		AttachType string `json:"attach_type"`
	} `json:"tc"`
}

func collectBpf(nsPath string, ifaces []InterfaceInfo) []BpfAttachment {
	if _, err := exec.LookPath("bpftool"); err != nil {
		return nil
	}

	out, err := runInNs(nsPath, "bpftool", "-j", "net", "show")
	if err != nil {
		log.Printf("bpftool net show in %s: %v", nsPath, err)
		return nil
	}

	var entries []bpftoolNetEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		log.Printf("bpftool parse in %s: %v", nsPath, err)
		return nil
	}

	var result []BpfAttachment
	for _, e := range entries {
		for _, x := range e.XDP {
			result = append(result, BpfAttachment{
				IfName:     e.IfName,
				ProgID:     x.ID,
				ProgName:   x.Name,
				AttachType: "xdp",
			})
		}
		for _, t := range e.TC {
			result = append(result, BpfAttachment{
				IfName:     e.IfName,
				ProgID:     t.ID,
				ProgName:   t.Name,
				AttachType: t.AttachType,
			})
		}
	}
	return result
}

// ---- nftables collection ----

type nftChain struct {
	Family string `json:"family"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	Hook   string `json:"hook"`
	Type   string `json:"type"`
}

func collectNft(nsPath string) []NftHook {
	out, err := runInNs(nsPath, "nft", "-j", "list", "ruleset")
	if err != nil {
		return nil
	}

	var root struct {
		Nftables []json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(out, &root); err != nil {
		log.Printf("nft parse in %s: %v", nsPath, err)
		return nil
	}

	// hook → set of table names
	hookTables := make(map[string]map[string]bool)

	for _, elem := range root.Nftables {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(elem, &obj); err != nil {
			continue
		}
		raw, ok := obj["chain"]
		if !ok {
			continue
		}
		var chain nftChain
		if err := json.Unmarshal(raw, &chain); err != nil {
			continue
		}
		// Only include ip, ip6, inet families
		switch chain.Family {
		case "ip", "ip6", "inet":
		default:
			continue
		}
		// Only base chains have a hook
		if chain.Hook == "" {
			continue
		}
		if hookTables[chain.Hook] == nil {
			hookTables[chain.Hook] = make(map[string]bool)
		}
		hookTables[chain.Hook][chain.Table] = true
	}

	var result []NftHook
	for hook, tables := range hookTables {
		var tableList []string
		for t := range tables {
			tableList = append(tableList, t)
		}
		result = append(result, NftHook{Hook: hook, Tables: tableList})
	}
	return result
}

// ---- TC qdisc collection ----

func collectQdiscs(nsPath string, ifaces []InterfaceInfo) []QdiscInfo {
	var result []QdiscInfo
	for _, iface := range ifaces {
		out, err := runInNs(nsPath, "tc", "-j", "qdisc", "show", "dev", iface.IfName)
		if err != nil {
			log.Printf("tc qdisc show dev %s in %s: %v", iface.IfName, nsPath, err)
			continue
		}

		var qdiscs []struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(out, &qdiscs); err != nil {
			log.Printf("tc qdisc parse for %s in %s: %v", iface.IfName, nsPath, err)
			continue
		}

		for _, q := range qdiscs {
			if q.Kind == "noqueue" || q.Kind == "mq" {
				continue
			}
			result = append(result, QdiscInfo{IfName: iface.IfName, Kind: q.Kind})
		}
	}
	return result
}

// ---- route collection ----

func collectRoutes(nsPath string) bool {
	out, err := runInNs(nsPath, "ip", "-j", "route", "show")
	if err != nil {
		return false
	}

	var routes []map[string]json.RawMessage
	if err := json.Unmarshal(out, &routes); err != nil {
		return false
	}

	for _, r := range routes {
		// Skip local table routes
		if tableRaw, ok := r["table"]; ok {
			var table string
			if json.Unmarshal(tableRaw, &table) == nil && table == "local" {
				continue
			}
		}
		// Skip local dst
		if dstRaw, ok := r["dst"]; ok {
			var dst string
			if json.Unmarshal(dstRaw, &dst) == nil && dst == "local" {
				continue
			}
		}
		return true
	}
	return false
}

// ---- socket collection ----

type ssEntry struct {
	State     string   `json:"state"`
	Process   *ssProc  `json:"process"`
	Processes []ssProc `json:"processes"`
}

type ssProc struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

func collectSockets(nsPath string) []SocketInfo {
	out, err := runInNs(nsPath, "ss", "-j", "-tunap")
	if err != nil {
		log.Printf("ss in %s: %v", nsPath, err)
		return nil
	}

	var entries []ssEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		log.Printf("ss parse in %s: %v", nsPath, err)
		return nil
	}

	type pidComm struct {
		pid  int
		comm string
	}
	seen := make(map[pidComm]bool)
	var result []SocketInfo

	for _, e := range entries {
		state := strings.ToUpper(e.State)
		switch state {
		case "LISTEN", "ESTABLISHED", "CONNECTED":
		default:
			continue
		}

		var procs []ssProc
		if e.Process != nil {
			procs = append(procs, *e.Process)
		}
		procs = append(procs, e.Processes...)

		for _, p := range procs {
			key := pidComm{pid: p.PID, comm: p.Name}
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, SocketInfo{Comm: p.Name, PID: p.PID})
		}
	}
	return result
}

// ---- nsid → inode map ----

func collectNsIDMap(nsPath string) map[int]uint64 {
	out, err := runInNs(nsPath, "ip", "-j", "netns", "list-id")
	if err != nil {
		return nil
	}

	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		log.Printf("ip netns list-id parse in %s: %v", nsPath, err)
		return nil
	}

	result := make(map[int]uint64)
	for _, obj := range raw {
		var nsid int
		var inode uint64
		if v, ok := obj["nsid"]; ok {
			_ = json.Unmarshal(v, &nsid)
		}
		// iproute2 >= 5.9 uses "nsnsid"; try common variants
		for _, key := range []string{"nsnsid", "peer-ns", "peer_ns"} {
			if v, ok := obj[key]; ok {
				if json.Unmarshal(v, &inode) == nil && inode != 0 {
					break
				}
			}
		}
		if inode != 0 {
			result[nsid] = inode
		}
	}
	return result
}
