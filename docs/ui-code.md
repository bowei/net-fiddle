# UI Code Reference

## High-level layout

The app is a single React component tree rooted at `App.tsx`. The visible layout is three vertical panels inside a flex row:

```
┌─ header ──────────────────────────────────────┐
├─ sidebar ─┬─ canvas (ReactFlow) ─┬─ properties ┤
└─ status bar ──────────────────────────────────┘
```

The canvas is managed entirely by ReactFlow. All node/edge state lives in `App.tsx` and is passed down.

---

## App.tsx

Root component. Owns all state and event wiring.

**State:**
- `nodes` / `edges` — ReactFlow node and edge arrays (via `useNodesState` / `useEdgesState`)
- `selectedNode` — drives the properties panel; updated on node click and kept in sync with `nodes` via a `useEffect`
- `hoverContainerId` — shared through `DragContext` to highlight the active drop target container during drags
- `rfInstance` — ReactFlow instance ref; used to project screen coordinates to canvas coordinates on drop
- `counters` — ref tracking per-type auto-increment counters for label generation (`socket-1`, `socket-2`, …)

**Derived state (useMemo):**
- `violations` — runs `ALL_RULES` over current nodes/edges
- `violationsByEdge` — indexes violations by `edgeId` for O(1) lookup
- `styledEdges` — applies stroke color (`#3b82f6` ingress, `#f59e0b` egress, `#ef4444` error), arrowheads, and `⚠` labels based on violations and flow type

**Drop handling (`onDrop`):**

The drop type comes from `dataTransfer` key `application/netfiddle`, set in `onDragStart`. Two code paths:

1. `nodeType === 'veth'` — special case: creates two `veth-end` nodes offset ±90px from the drop point, plus a permanent dashed edge (`data.vethLink: true`). Both nodes share a `vethPairId` so they can be co-deleted.
2. `nodeType === 'netkit'` — similar special case: creates `netkit-primary` and `netkit-peer` nodes offset ±90px apart, plus a permanent dashed edge. Both share a `vethPairId` for co-deletion (the same field is reused since the behavior is identical).
3. Everything else — looks up the def in REGISTRY, creates either a `containerNode` or `netNode`. If dropped inside a container's bounding box (`findContainerAt`), sets `parentNode` and converts the position to container-relative coordinates.

**Container collision (`findContainerAt`):**
Simple AABB test against all `containerNode` nodes. Uses `node.width/height` or falls back to `style.width/height`.

**Reparenting on drag stop (`onNodeDragStop`):**
Uses `node.positionAbsolute` (set by ReactFlow during drag) to test which container the node lands in, then updates `parentNode` and converts position to be parent-relative.

**Deletion (`deleteSelected`):**
- Container: also deletes all children (`parentNode === id`)
- Veth end: also deletes the other end (matched via `vethPairId`)
- Triggered by Delete/Backspace keydown (skipped when an input is focused)

**Edge reconnect protection:**
The `onEdgeReconnect*` callbacks check `edge.data?.vethLink` and skip reconnection/deletion for permanent veth edges. An `edgeReconnectSuccessful` ref tracks whether the reconnect gesture completed; if not, the edge is deleted on `onEdgeReconnectEnd`.

**`edgeFlowType(edge, nodes)`:**
Returns the dominant `AnchorFlow` of an edge for coloring. Calls `resolveEdgeHandleFlows()` (from `src/rules/utils.ts`) and returns whichever side is non-`'any'`, preferring source.

---

## CustomNode.tsx

Renders individual network nodes. Exported type `NetNodeData`:

```ts
interface NetNodeData {
  nodeType: string;                  // key into REGISTRY
  label: string;
  config?: Record<string, string>;   // e.g. { direction: 'ingress' }
  vethPairId?: string;               // set on veth-end nodes
}
```

**Dynamic anchor positioning** — runs on every render using live `useNodes()` and `useEdges()`:

1. Build `connectedCenter` map: for each edge touching this node, record the absolute center `(x + w/2, y + h/2)` of the peer node, keyed by base handle ID (with `-s`/`-t` suffix stripped).
2. For each resolved handle, call `closestSide(dx, dy, hw, hh)` with the vector from this node's center to the peer's center. This uses `|dx/hw| vs |dy/hh|` (normalized by half-dimensions) to pick the nearest face. Disconnected handles use their declared default side.
3. Group handles by their assigned side, then sort within each group by the peer's x-coordinate (for N/S sides) or y-coordinate (for E/W sides) to minimize edge crossings.
4. Assign `dynIdx` (position within the sorted group) and `dynTotal` (group size) for spacing calculations.

**Handle rendering:**
Each anchor produces up to two ReactFlow `Handle` components:
- `connector !== 'out'` → hollow target handle (transparent fill, colored border)
- `connector !== 'in'` → solid source handle (filled with flow color)

The React key includes `dynSide` (e.g. `"N-0-t"` becomes `"S-0-t"` when the handle migrates). This forces React to remount the Handle, which forces ReactFlow to re-register its DOM position.

**`useUpdateNodeInternals()`** is called in a `useEffect` whenever `dynKey` changes. This is necessary because handle DOM positions change without the node resizing, so ReactFlow's ResizeObserver never fires — without this call the edge routing store retains stale coordinates.

---

## ContainerNode.tsx

Renders namespace/container boxes. Uses ReactFlow's `NodeResizer` component with `minWidth`/`minHeight` from the def.

Reads `DragContext` to apply the `.drop-target` CSS class when `hoverContainerId === id`. Background and border colors intensify to signal the active drop zone.

---

## anchorUtils.ts

Pure utility functions with no React dependencies.

| Function | Purpose |
|---|---|
| `toRFPosition(side)` | Maps `CardinalSide` → ReactFlow `Position` enum |
| `anchorStyle(side, idx, total)` | Returns `{ left: '33%' }` or `{ top: '50%' }` for inline handle positioning. Formula: `(idx + 1) / (total + 1) * 100%` — handles are always between 0% and 100% exclusive |
| `anchorHandleId(side, idx)` | Produces stable IDs: `'N-0'`, `'S-1'`, etc. |
| `anchorFlowColor(flow, fallback)` | `'ingress'` → `#3b82f6`, `'egress'` → `#f59e0b`, else fallback |
| `resolveHandles(anchors)` | Converts `AnchorSpec[]` into `ResolvedHandle[]` with global indices and totals per side |

`resolveHandles` merges all specs declaring the same side before assigning indices, so components can declare anchors in separate logical groups without tracking global positions.

---

## DragContext.ts

A single `React.createContext<string | null>(null)` holding the ID of the container being hovered during a drag, or `null`. Set by `App`'s `onDragOver`/`onDragLeave`, read by `ContainerNode`. Avoids prop-drilling through ReactFlow's node renderer boundary.

---

## Component definition system

### base.ts

Two abstract base classes:

**`ComponentDef`** — for plain nodes. Required abstract members: `type`, `label`, `typeLabel`, `color`, `bgColor`, `borderColor`, `icon`, `configTitle`, `configItems`, `anchors`. Optional overrides:
- `configFields: ConfigField[]` — dropdown fields shown in the properties panel
- `getAnchors(config): AnchorSpec[]` — override for config-dependent anchors (e.g. TC direction changes which flow the handles advertise); default returns `this.anchors`

**`ContainerComponentDef extends ComponentDef`** — adds `defaultWidth`, `defaultHeight`, `minWidth`, `minHeight`. The `instanceof` check in App and the registry uses this to decide which ReactFlow node type to assign.

**`AnchorSpec`:**
```ts
{ side: CardinalSide; count: number; flow: AnchorFlow; connector?: AnchorConnector }
```
`connector` defaults to `'both'`; use `'in'` for handles that only accept incoming edges, `'out'` for handles that only emit.

### registry.ts

Defines `SIDEBAR_GROUPS` (the rendered sidebar), `SIDEBAR_ITEMS` (all ComponentDefs, excluding SidebarTemplates), and `REGISTRY` (the lookup map).

`vethEnd`, `netkitPrimary`, and `netkitPeer` are in `REGISTRY` but not in `SIDEBAR_ITEMS` — they are created programmatically when the corresponding pair template is dropped. `vethPair` and `netkitPair` are `SidebarTemplate` objects (have `type`/`label`/`color`/`icon` but no anchors) — they appear in the sidebar but are not ComponentDefs and do not enter the registry.

To add a new component type: create a class file extending `ComponentDef`, export a singleton, add it to the `COMPONENTS` array in `registry.ts`.

---

## Graph validation (src/rules/)

See the extensible rule system:

| File | Contents |
|---|---|
| `types.ts` | `GraphViolation`, `GraphRule` interfaces |
| `utils.ts` | `resolveEdgeHandleFlows(edge, nodes)` — resolves both handle flows for an edge; returns `null` if either side is unresolvable |
| `flowMismatch.ts` | Flags edges where source and target have incompatible non-`'any'` flows |
| `linuxOrder.ts` | Validates that connected components follow the Linux kernel's actual packet processing order (separate tables for ingress and egress; TC/TC-BPF position is config-dependent) |
| `netkitBpfPlacement.ts` | Netkit BPF ingress/egress nodes may only connect to netkit-primary, netkit-peer, or xdp-program |
| `tcBpfPlacement.ts` | tc-bpf-program and sched-bpf nodes may only connect to traffic-control nodes |
| `xdpPlacement.ts` | xdp-program source must be an interface, veth-end, or netkit node |
| `index.ts` | `ALL_RULES` export; re-exports everything |

Violations are computed in `App.tsx` via `useMemo` and used to color edges red and show a count badge in the status bar. `vethLink` edges are skipped by all rules.

---

## CSS (index.css)

Key layout classes and their roles:

| Class | Role |
|---|---|
| `.app` | `flex column; height: 100vh` root |
| `.body` | `flex row; flex: 1` below header |
| `.canvas-wrapper` | `flex: 1` ReactFlow host; `.drag-over` adds faint indigo tint |
| `.sidebar` | Fixed 160px, scrollable |
| `.properties-panel` | Fixed 240px, scrollable |
| `.custom-node` | Node card — 130px min-width, 10px padding, 2px border, shadow on hover |
| `.custom-node.selected` | Box-shadow ring using `currentColor` (def.color) |
| `.container-node` | 100% width/height, dashed border, transitions on color |
| `.container-node.drop-target` | Brighter border + stronger shadow while a node is dragged over it |
| `.status-violations` | Red pill badge in footer showing violation count |
| `.veth-link .react-flow__edgeupdater` | `display: none` — hides the reconnect drag handles on permanent veth edges |
