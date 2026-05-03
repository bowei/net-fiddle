# UI Functionality Reference

## Overview

Net Fiddle is a visual designer for Linux network topologies. You build diagrams by dragging components onto a canvas, connecting them with edges, and the tool validates whether the resulting graph is a valid Linux packet path.

---

## Layout

The interface has four regions:

- **Sidebar** (left) — component palette, organized into groups
- **Canvas** (center) — the diagram workspace
- **Properties panel** (right) — appears when a node is selected
- **Header** (top) — toolbar with load/import/export/clear actions
- **Status bar** (bottom) — keyboard shortcut reference and violation count

---

## Building a Diagram

### Adding components

Drag any component from the sidebar onto the canvas. Releasing the drag creates a node at that position.

If you drag onto a namespace box, the node is automatically nested inside it — it will move with the namespace if the namespace is repositioned.

### Namespaces

A namespace is a resizable container box. Drag its border handles to resize it. Any component dropped inside its bounding box becomes a child of that namespace.

### Veth pairs

Dragging a **Veth Pair** from the sidebar drops two linked veth-end nodes side by side. They are permanently connected by a dashed edge representing the virtual wire between the two ends. This link cannot be removed or reconnected — to remove it, delete one of the veth-end nodes (both ends are deleted together).

### Connecting components

Drag from any connector dot on a node to a connector dot on another node to create an edge. Edges can be reconnected by dragging from the midpoint of an existing edge to a new target. Dragging an edge off into empty space removes it.

### Connector dots

Each component has one or more connector points on its perimeter:

- **Solid dot** — source (outgoing connection)
- **Hollow dot** — target (incoming connection)
- **Blue** — ingress (wire → process) traffic
- **Amber** — egress (process → wire) traffic

### Dynamic connector positioning

Connector dots move to the side of the node that faces their connected peer. When two nodes are connected, each end's connector dot migrates to whichever face (top, bottom, left, or right) is geometrically closest to the other node. This keeps edges short and avoids routing them through the node's body.

When the connecting edge is removed, the dot returns to its default position.

If a node has multiple connectors on the same side, they are sorted by their destination's position along that side to avoid crossing lines.

---

## Selecting and Editing

Click any node to select it and open the properties panel. Click the canvas background to deselect.

### Properties panel

The properties panel shows:

- **Type** — read-only component type name
- **Label** — editable; the name shown on the node
- **Config fields** — dropdowns for configurable properties (e.g. TC direction: ingress/egress; qdisc type: fq/htb/etc.)
- **Position** — X/Y coordinates (for plain nodes); or Width/Height (for container nodes, read-only)
- **Description** — bullet-point notes about what the component does in the Linux kernel

### Moving nodes

Drag any node to reposition it. Dragging a node onto a namespace box reparents it into that namespace. Dragging it off a namespace un-parents it back to the root canvas.

Dragging a namespace moves all its children with it.

### Deleting nodes

Select a node, then press **Delete** or **Backspace**, or click the trash icon in the properties panel.

- Deleting a namespace also deletes all nodes nested inside it.
- Deleting a veth-end node deletes both ends of the pair.

---

## Edge Validation

Edges are colored to show flow direction and validation status:

| Color | Meaning |
|---|---|
| Blue | Ingress traffic (wire → process) |
| Amber | Egress traffic (process → wire) |
| Red + ⚠ | Validation error |

Two kinds of errors are detected:

**Flow mismatch** — the source connector is typed for one direction (e.g. egress) but the target connector expects the other (e.g. ingress). This catches connecting components in a direction that makes no physical sense.

**Linux ordering violation** — the source and target are in the wrong order for the Linux kernel's actual packet processing pipeline. For example, connecting a qdisc before an nftables postrouting hook on an egress path would be flagged, because in Linux qdisc comes after postrouting.

The expected Linux ordering is:

**Ingress:** interface / veth-end → XDP → TC ingress → nftables prerouting → routing table → nftables input / forward → socket

**Egress:** socket → nftables output → nftables postrouting → TC egress → qdisc → interface / veth-end

TC and TC-BPF position depends on the configured direction (ingress or egress).

When violations exist, a red badge in the status bar shows the count. Hovering over it shows all violation messages.

---

## Toolbar Actions

| Button | Action |
|---|---|
| Load Sample | Loads a built-in example diagram |
| Import | Opens a file picker to load a JSON diagram file |
| Export | Downloads the current diagram as a JSON file |
| Clear All | Removes all nodes and edges |

---

## Keyboard Shortcuts

| Key | Action |
|---|---|
| Delete / Backspace | Delete selected node (when no text input is focused) |
| Escape | Deselect current node |

---

## Import / Export

Diagrams are stored as JSON containing a `nodes` array and an `edges` array in ReactFlow's standard format. The `examples/` directory contains ready-made diagrams:

- `linux-ingress.json` — complete ingress path (NIC → socket)
- `linux-egress.json` — complete egress path (socket → NIC)
- `linux-io.json` — combined ingress and egress sharing a socket and interface

To load an example, use **Import** and select the file, or use **Load Sample** for the built-in demo.
