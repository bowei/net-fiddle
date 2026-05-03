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
	// Build index: (inode, ifindex) → (InterfaceInfo, NsSnapshot)
	type entry struct {
		iface collector.InterfaceInfo
		snap  *collector.NsSnapshot
	}
	index := make(map[PairKey]entry)
	for i := range snapshots {
		snap := &snapshots[i]
		for _, iface := range snap.Interfaces {
			if iface.Kind == "veth" || iface.Kind == "netkit" {
				index[PairKey{snap.Ns.Inode, iface.IfIndex}] = entry{iface, snap}
			}
		}
	}

	// Build inode → snapshot for fast lookup
	inodeToSnap := make(map[uint64]*collector.NsSnapshot)
	for i := range snapshots {
		inodeToSnap[snapshots[i].Ns.Inode] = &snapshots[i]
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

		// Resolve which snapshot holds the peer
		peerSnap := resolvePeerSnap(e.iface, e.snap, inodeToSnap)
		if peerSnap == nil {
			continue
		}

		peerKey := PairKey{NsInode: peerSnap.Ns.Inode, IfIndex: e.iface.PeerIfIndex}
		peerEntry, ok := index[peerKey]
		if !ok {
			continue
		}

		// Bidirectional confirmation
		if peerEntry.iface.PeerIfIndex != e.iface.IfIndex {
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

// resolvePeerSnap returns the snapshot that contains the peer interface.
// It uses link_netnsid + the source namespace's NsIDMap to identify the peer
// namespace by inode. Falls back to a full scan when nsid is unavailable.
func resolvePeerSnap(iface collector.InterfaceInfo, src *collector.NsSnapshot, inodeToSnap map[uint64]*collector.NsSnapshot) *collector.NsSnapshot {
	if iface.LinkNetNsID != nil && src.NsIDMap != nil {
		if inode, ok := src.NsIDMap[*iface.LinkNetNsID]; ok {
			return inodeToSnap[inode]
		}
	}

	// Peer is in the same namespace when link_netnsid is nil
	if iface.LinkNetNsID == nil {
		return src
	}

	// Last resort: scan all snapshots for the peer ifindex
	for _, snap := range inodeToSnap {
		if snap.Ns.Inode == src.Ns.Inode {
			continue
		}
		for _, other := range snap.Interfaces {
			if other.IfIndex == iface.PeerIfIndex && other.PeerIfIndex == iface.IfIndex {
				return snap
			}
		}
	}
	return nil
}
