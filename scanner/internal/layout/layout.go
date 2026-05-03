package layout

import (
	"net-fiddle/scanner/internal/builder"
	"net-fiddle/scanner/internal/topology"
)

const (
	nsWidth      = 280.0
	nsColSpacing = 420.0
	nodeHeight   = 44.0
	rowSpacing   = 90.0
	leftX        = 20.0
	centreX      = 90.0
	rightX       = 155.0
	startY       = 36.0
	nsTopPad     = 20.0
	nsSidePad    = 20.0
)

func Layout(result *builder.BuildResult) {
	// Group children by parent
	children := make(map[string][]*topology.Node)
	var containers []*topology.Node
	for i := range result.Nodes {
		n := &result.Nodes[i]
		if n.Type == "containerNode" {
			containers = append(containers, n)
		} else if n.ParentNode != "" {
			children[n.ParentNode] = append(children[n.ParentNode], n)
		}
	}

	// Assign namespace column positions
	for colIdx, ns := range containers {
		nsX := float64(colIdx)*nsColSpacing + 30.0
		ns.Position = topology.Position{X: nsX, Y: 30}

		kids := children[ns.ID]

		// Sort kids into columns
		leftCol, centreCol, rightCol := categorise(kids)

		maxY := assignColumn(leftCol, nsX+leftX, ns.Position.Y+startY)
		cy := assignColumn(centreCol, nsX+centreX, ns.Position.Y+startY)
		ry := assignColumn(rightCol, nsX+rightX, ns.Position.Y+startY)
		if cy > maxY {
			maxY = cy
		}
		if ry > maxY {
			maxY = ry
		}

		// Now make positions relative to container
		for _, kid := range kids {
			kid.Position.X -= nsX
			kid.Position.Y -= ns.Position.Y
		}

		// Size container
		h := maxY - ns.Position.Y + nodeHeight + 30.0
		if h < 120 {
			h = 120
		}
		ns.Style = &topology.NodeStyle{Width: nsWidth, Height: h}
	}
}

func categorise(nodes []*topology.Node) (left, centre, right []*topology.Node) {
	for _, n := range nodes {
		switch n.Data.NodeType {
		case "interface", "veth-end", "netkit-primary", "netkit-peer",
			"xdp-program":
			left = append(left, n)
		case "tc-bpf-program":
			if n.Data.Config["direction"] == "egress" {
				right = append(right, n)
			} else {
				left = append(left, n)
			}
		case "traffic-control":
			if n.Data.Config["direction"] == "egress" {
				right = append(right, n)
			} else {
				left = append(left, n)
			}
		case "sched-bpf", "qdisc":
			right = append(right, n)
		case "nftables-output", "nftables-postrouting":
			right = append(right, n)
		case "socket":
			right = append(right, n)
		default: // nftables-prerouting, nftables-input, nftables-forward, routing-table, network
			centre = append(centre, n)
		}
	}
	return
}

func assignColumn(nodes []*topology.Node, x, startY float64) float64 {
	y := startY
	for _, n := range nodes {
		n.Position = topology.Position{X: x, Y: y}
		y += rowSpacing
	}
	if len(nodes) == 0 {
		return startY
	}
	return y - rowSpacing
}
