package builder

import (
	"fmt"
	"strings"

	"net-fiddle/scanner/internal/collector"
	"net-fiddle/scanner/internal/linker"
	"net-fiddle/scanner/internal/topology"
)

type BuildResult struct {
	Nodes []topology.Node
	Edges []topology.Edge
}

func Build(snapshots []collector.NsSnapshot, pairs map[linker.PairKey]linker.PairInfo) BuildResult {
	var nodes []topology.Node
	var edges []topology.Edge
	edgeN := 0

	nextEdgeID := func(prefix string) string {
		edgeN++
		return fmt.Sprintf("e-%s-%d", prefix, edgeN)
	}

	addEdge := func(srcID, srcHandle, tgtID, tgtHandle string) {
		edges = append(edges, topology.Edge{
			ID:           nextEdgeID("flow"),
			Source:       srcID,
			SourceHandle: srcHandle,
			Target:       tgtID,
			TargetHandle: tgtHandle,
		})
	}

	netkitSeen := make(map[string]bool) // pairID → primary already assigned

	for _, snap := range snapshots {
		nsID := "ns-" + snap.Ns.Name

		// --- namespace container node (placeholder position; layouter fills it in) ---
		zIdx := -1
		nsNode := topology.Node{
			ID:     nsID,
			Type:   "containerNode",
			ZIndex: &zIdx,
			Style:  &topology.NodeStyle{Width: 300, Height: 200},
			Data:   topology.NodeData{NodeType: "namespace", Label: snap.Ns.Name},
		}
		nodes = append(nodes, nsNode)

		// --- per-namespace shared nodes ---

		// nftables hook nodes
		nftNodeID := make(map[string]string) // hook → nodeID
		for _, h := range snap.NftHooks {
			nid := fmt.Sprintf("%s-nft-%s", snap.Ns.Name, h.Hook)
			label := "nft:" + h.Hook
			if len(h.Tables) > 0 {
				label = "nft:" + strings.Join(h.Tables, "+")
			}
			nodes = append(nodes, topology.Node{
				ID: nid, Type: "netNode", ParentNode: nsID,
				Data: topology.NodeData{NodeType: "nftables-" + h.Hook, Label: label},
			})
			nftNodeID[h.Hook] = nid
		}

		// routing-table
		var routingID string
		if snap.HasRoutes {
			routingID = snap.Ns.Name + "-routing"
			nodes = append(nodes, topology.Node{
				ID: routingID, Type: "netNode", ParentNode: nsID,
				Data: topology.NodeData{NodeType: "routing-table", Label: "routing"},
			})
		}

		// socket nodes
		var socketIDs []string
		for _, s := range snap.Sockets {
			sid := fmt.Sprintf("%s-sock-%s-%d", snap.Ns.Name, s.Comm, s.PID)
			nodes = append(nodes, topology.Node{
				ID: sid, Type: "netNode", ParentNode: nsID,
				Data: topology.NodeData{NodeType: "socket", Label: s.Comm},
			})
			socketIDs = append(socketIDs, sid)
		}

		// --- per-interface nodes ---
		for _, iface := range snap.Interfaces {
			pfx := snap.Ns.Name + "-" + iface.IfName
			pairKey := linker.PairKey{NsInode: snap.Ns.Inode, IfIndex: iface.IfIndex}
			pairInfo := pairs[pairKey]

			// Determine nodeType
			nodeType := "interface"
			vethPairID := ""
			var config map[string]string

			switch iface.Kind {
			case "veth":
				nodeType = "veth-end"
				vethPairID = pairInfo.PairID
			case "netkit":
				vethPairID = pairInfo.PairID
				if !netkitSeen[pairInfo.PairID] {
					netkitSeen[pairInfo.PairID] = true
					nodeType = "netkit-primary"
				} else {
					nodeType = "netkit-peer"
				}
			}

			ifaceNodeID := pfx
			nodes = append(nodes, topology.Node{
				ID: ifaceNodeID, Type: "netNode", ParentNode: nsID,
				Data: topology.NodeData{
					NodeType:   nodeType,
					Label:      iface.IfName,
					Config:     config,
					VethPairID: vethPairID,
				},
			})

			// Pair-link edge (only emit once — when we have both ends mapped)
			if pairInfo.PairID != "" {
				// The linker ensures both ends are in the map. Only emit the edge
				// for the primary/first end to avoid duplicates.
				if nodeType == "veth-end" || nodeType == "netkit-primary" {
					// Find the peer node ID: scan all snapshots for peer
					if peerNodeID := findPeerNodeID(snap.Ns.Inode, iface, snapshots, pairs); peerNodeID != "" {
						edges = append(edges, topology.Edge{
							ID:           fmt.Sprintf("pair-link-%s", pairInfo.PairID),
							Source:       ifaceNodeID,
							SourceHandle: "S-0-s",
							Target:       peerNodeID,
							TargetHandle: "S-0-t",
							Data:         &topology.EdgeData{VethLink: true},
							ClassName:    "veth-link",
							Style:        &topology.EdgeStyle{Stroke: "#0d9488", StrokeWidth: 3, StrokeDashArray: "6 3"},
							Label:        "\u26d3",
						})
					}
				}
			}

			// --- BPF nodes for this interface ---
			var xdpNodes []string
			var tcBpfIngressNodes []string
			var tcBpfEgressNodes []string
			var schedBpfNodes []string

			for _, bpf := range snap.BpfAttach {
				if bpf.IfName != iface.IfName {
					continue
				}
				switch bpf.AttachType {
				case "xdp":
					nid := fmt.Sprintf("%s-xdp-%d", pfx, bpf.ProgID)
					label := bpf.ProgName
					if label == "" {
						label = fmt.Sprintf("xdp-%d", bpf.ProgID)
					}
					nodes = append(nodes, topology.Node{
						ID: nid, Type: "netNode", ParentNode: nsID,
						Data: topology.NodeData{NodeType: "xdp-program", Label: label},
					})
					xdpNodes = append(xdpNodes, nid)
				case "ingress":
					nid := fmt.Sprintf("%s-tcbpf-ingress-%d", pfx, bpf.ProgID)
					label := bpf.ProgName
					if label == "" {
						label = fmt.Sprintf("tc-bpf-%d", bpf.ProgID)
					}
					nodes = append(nodes, topology.Node{
						ID: nid, Type: "netNode", ParentNode: nsID,
						Data: topology.NodeData{NodeType: "tc-bpf-program", Label: label, Config: map[string]string{"direction": "ingress"}},
					})
					tcBpfIngressNodes = append(tcBpfIngressNodes, nid)
				case "egress":
					nid := fmt.Sprintf("%s-tcbpf-egress-%d", pfx, bpf.ProgID)
					label := bpf.ProgName
					if label == "" {
						label = fmt.Sprintf("tc-bpf-%d", bpf.ProgID)
					}
					nodes = append(nodes, topology.Node{
						ID: nid, Type: "netNode", ParentNode: nsID,
						Data: topology.NodeData{NodeType: "tc-bpf-program", Label: label, Config: map[string]string{"direction": "egress"}},
					})
					tcBpfEgressNodes = append(tcBpfEgressNodes, nid)
				}
			}

			// --- TC nodes ---
			var tcIngressID, tcEgressID string
			hasClsact := false
			for _, q := range snap.Qdiscs {
				if q.IfName != iface.IfName {
					continue
				}
				if q.Kind == "clsact" || q.Kind == "ingress" {
					hasClsact = true
				}
			}
			// Emit traffic-control nodes if clsact present OR tc-bpf attachments found
			if hasClsact || len(tcBpfIngressNodes) > 0 {
				tcIngressID = fmt.Sprintf("%s-tc-ingress", pfx)
				nodes = append(nodes, topology.Node{
					ID: tcIngressID, Type: "netNode", ParentNode: nsID,
					Data: topology.NodeData{NodeType: "traffic-control", Label: "tc-" + iface.IfName, Config: map[string]string{"direction": "ingress"}},
				})
			}
			if hasClsact || len(tcBpfEgressNodes) > 0 {
				tcEgressID = fmt.Sprintf("%s-tc-egress", pfx)
				nodes = append(nodes, topology.Node{
					ID: tcEgressID, Type: "netNode", ParentNode: nsID,
					Data: topology.NodeData{NodeType: "traffic-control", Label: "tc-" + iface.IfName, Config: map[string]string{"direction": "egress"}},
				})
			}

			// sched-bpf: any egress BPF without traffic-control → sched-bpf nodes
			if !hasClsact && len(tcBpfEgressNodes) > 0 {
				// Re-classify egress tc-bpf as sched-bpf
				for i, nid := range tcBpfEgressNodes {
					// Find and relabel the node
					for j := range nodes {
						if nodes[j].ID == nid {
							nodes[j].Data.NodeType = "sched-bpf"
							nodes[j].Data.Config = nil
							schedBpfNodes = append(schedBpfNodes, nid)
							tcBpfEgressNodes[i] = ""
						}
					}
				}
				// filter out re-classified
				var kept []string
				for _, n := range tcBpfEgressNodes {
					if n != "" {
						kept = append(kept, n)
					}
				}
				tcBpfEgressNodes = kept
			}

			// --- qdisc node ---
			var qdiscID string
			for _, q := range snap.Qdiscs {
				if q.IfName != iface.IfName {
					continue
				}
				if q.Kind == "clsact" || q.Kind == "ingress" || q.Kind == "noqueue" || q.Kind == "mq" {
					continue
				}
				qdiscID = fmt.Sprintf("%s-qdisc", pfx)
				qcfg := map[string]string{}
				// Map kind to config type value supported by net-fiddle
				switch q.Kind {
				case "fq", "htb", "fq_codel", "tbf", "pfifo_fast":
					qcfg["type"] = q.Kind
				default:
					qcfg["type"] = "fq" // fallback
				}
				nodes = append(nodes, topology.Node{
					ID: qdiscID, Type: "netNode", ParentNode: nsID,
					Data: topology.NodeData{NodeType: "qdisc", Label: q.Kind + "-" + iface.IfName, Config: qcfg},
				})
				break // only one qdisc node per interface
			}

			// --- Build ingress edge chain ---
			// Chain: iface → xdp... → tc-bpf-ingress... → tc-ingress → nft-prerouting → routing → nft-input → socket
			ingressChain := []chainNode{
				{id: ifaceNodeID, out: "N-0-s"},
			}
			for _, n := range xdpNodes {
				ingressChain = append(ingressChain, chainNode{id: n, in: "S-0-t", out: "N-0-s"})
			}
			for _, n := range tcBpfIngressNodes {
				ingressChain = append(ingressChain, chainNode{id: n, in: "S-0-t", out: "N-0-s"})
			}
			if tcIngressID != "" {
				ingressChain = append(ingressChain, chainNode{id: tcIngressID, in: "N-0-t", out: "S-0-s"})
			}
			if nid, ok := nftNodeID["prerouting"]; ok {
				ingressChain = append(ingressChain, chainNode{id: nid, in: "S-0-t", out: "N-0-s"})
			}
			if routingID != "" {
				ingressChain = append(ingressChain, chainNode{id: routingID, in: "N-0-t", out: "S-0-s"})
			}
			if nid, ok := nftNodeID["input"]; ok {
				ingressChain = append(ingressChain, chainNode{id: nid, in: "S-0-t", out: "N-0-s"})
			}
			if nid, ok := nftNodeID["forward"]; ok {
				ingressChain = append(ingressChain, chainNode{id: nid, in: "N-0-t", out: "S-0-s"})
			}
			// Connect each socket to the last nft-input node
			if len(socketIDs) > 0 {
				if nid, ok := nftNodeID["input"]; ok {
					for _, sid := range socketIDs {
						addEdge(nid, "N-0-s", sid, "S-0-t")
					}
				} else if routingID != "" {
					for _, sid := range socketIDs {
						addEdge(routingID, "S-0-s", sid, "S-0-t")
					}
				}
			}
			connectChain(ingressChain, addEdge)

			// --- Build egress edge chain ---
			// Chain: socket → nft-output → nft-postrouting → tc-egress → tc-bpf-egress... → sched-bpf... → qdisc → iface
			var egressChain []chainNode
			if len(socketIDs) > 0 {
				egressChain = append(egressChain, chainNode{id: socketIDs[0], out: "S-1-s"})
			}
			if nid, ok := nftNodeID["output"]; ok {
				egressChain = append(egressChain, chainNode{id: nid, in: "N-0-t", out: "S-0-s"})
			}
			if nid, ok := nftNodeID["postrouting"]; ok {
				egressChain = append(egressChain, chainNode{id: nid, in: "N-0-t", out: "S-0-s"})
			}
			if tcEgressID != "" {
				egressChain = append(egressChain, chainNode{id: tcEgressID, in: "N-0-t", out: "S-0-s"})
			}
			for _, n := range tcBpfEgressNodes {
				egressChain = append(egressChain, chainNode{id: n, in: "S-0-t", out: "N-0-s"})
			}
			for _, n := range schedBpfNodes {
				egressChain = append(egressChain, chainNode{id: n, in: "N-0-t", out: "S-0-s"})
			}
			if qdiscID != "" {
				egressChain = append(egressChain, chainNode{id: qdiscID, in: "N-0-t", out: "S-0-s"})
			}
			egressChain = append(egressChain, chainNode{id: ifaceNodeID, in: "N-1-t"})
			connectChain(egressChain, addEdge)
		}
	}

	return BuildResult{Nodes: nodes, Edges: edges}
}

type chainNode struct {
	id  string
	in  string // targetHandle (empty for first node)
	out string // sourceHandle (empty for last node)
}

func connectChain(chain []chainNode, addEdge func(string, string, string, string)) {
	for i := 0; i+1 < len(chain); i++ {
		src := chain[i]
		tgt := chain[i+1]
		if src.out == "" || tgt.in == "" {
			continue
		}
		addEdge(src.id, src.out, tgt.id, tgt.in)
	}
}

func findPeerNodeID(myNsInode uint64, iface collector.InterfaceInfo, snapshots []collector.NsSnapshot, pairs map[linker.PairKey]linker.PairInfo) string {
	myKey := linker.PairKey{NsInode: myNsInode, IfIndex: iface.IfIndex}
	myPair := pairs[myKey]
	if myPair.PairID == "" {
		return ""
	}

	for _, snap := range snapshots {
		for _, other := range snap.Interfaces {
			if snap.Ns.Inode == myNsInode && other.IfIndex == iface.IfIndex {
				continue
			}
			key := linker.PairKey{NsInode: snap.Ns.Inode, IfIndex: other.IfIndex}
			if pairs[key].PairID == myPair.PairID {
				return snap.Ns.Name + "-" + other.IfName
			}
		}
	}
	return ""
}
