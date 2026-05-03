package linker

import (
	"fmt"
	"net-fiddle/scanner/internal/collector"
)

type PairKey struct {
	NsInode uint64
	IfIndex int
}

type PairInfo struct {
	PairID string // "veth-pair-N" or "netkit-pair-N"
	Kind   string // "veth" or "netkit"
}

// Link matches veth/netkit interfaces across snapshots and returns a map
// from (nsInode, ifindex) → PairInfo for all paired interfaces.
func Link(snapshots []collector.NsSnapshot) map[PairKey]PairInfo {
	// Build index: (inode, ifindex) → InterfaceInfo + NsInfo
	type entry struct {
		iface collector.InterfaceInfo
		ns    collector.NsInfo
	}
	index := make(map[PairKey]entry)
	for _, snap := range snapshots {
		for _, iface := range snap.Interfaces {
			if iface.Kind == "veth" || iface.Kind == "netkit" {
				index[PairKey{snap.Ns.Inode, iface.IfIndex}] = entry{iface, snap.Ns}
			}
		}
	}

	result := make(map[PairKey]PairInfo)
	pairN := 0
	paired := make(map[PairKey]bool)

	for k, e := range index {
		if paired[k] {
			continue
		}
		if e.iface.PeerIfIndex == 0 {
			continue
		}

		// Search all namespaces for the peer by ifindex
		var peerKey PairKey
		found := false
		for k2 := range index {
			if k2 == k {
				continue
			}
			if k2.IfIndex == e.iface.PeerIfIndex {
				peerKey = k2
				found = true
				break
			}
		}
		if !found {
			continue
		}

		pairN++
		kind := e.iface.Kind
		pairID := fmt.Sprintf("%s-pair-%d", kind, pairN)
		info := PairInfo{PairID: pairID, Kind: kind}
		result[k] = info
		result[peerKey] = info
		paired[k] = true
		paired[peerKey] = true
	}

	return result
}
