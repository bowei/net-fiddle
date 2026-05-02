# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make dev        # start Vite dev server (http://localhost:5173)
make build      # tsc type-check + Vite production build → dist/
make test       # run Vitest test suite once
make typecheck  # tsc --noEmit only
make clean      # remove dist/ and node_modules/
```

Run a single test file:
```bash
npx vitest run src/test/registry.test.ts
```

## Architecture

### Component type system

Every draggable element on the canvas is defined by a class in `src/components/`:

- `base.ts` — two abstract base classes:
  - `ComponentDef` — plain node (small card on canvas)
  - `ContainerComponentDef extends ComponentDef` — resizable namespace-like box that other nodes can be dropped into; adds `defaultWidth/Height` and `minWidth/Height`
- One file per type (e.g. `namespace.ts`, `netInterface.ts`) — concrete class + exported singleton
- `registry.ts` — imports all singletons, exports `REGISTRY: ReadonlyMap<string, ComponentDef>` and `SIDEBAR_ITEMS: readonly ComponentDef[]`

**To add a new component type:** create a class file, export a singleton, add it to the `COMPONENTS` array in `registry.ts`. Nothing else needs to change.

**To add a new container type:** extend `ContainerComponentDef` instead of `ComponentDef`.

### React Flow node types

`App.tsx` registers two ReactFlow node renderers:

| ReactFlow type | Renderer | Used for |
|---|---|---|
| `netNode` | `CustomNode.tsx` | All plain `ComponentDef` types |
| `containerNode` | `ContainerNode.tsx` | All `ContainerComponentDef` types |

The correct renderer is selected at drop-time via `instanceof ContainerComponentDef`.

### Drag-and-drop and reparenting

`App.tsx` owns all canvas state (`nodes`, `edges`, `selectedNode`, `hoverContainerId`).

- **Sidebar → canvas drop** (`onDrop`): creates a new node; if the drop point is inside a container's bounding box, sets `parentNode` and makes the position parent-relative.
- **Canvas node drag** (`onNodeDrag` / `onNodeDragStop`): uses `node.positionAbsolute` (set by React Flow) for hit-testing against container bounds; reparents or un-parents after drag stops.
- **Drop-target highlight**: `hoverContainerId` is shared to container nodes via `DragContext` (a React context in `DragContext.ts`) so `ContainerNode` can apply a highlight CSS class without touching node state on every mouse-move.

### Test coverage

`src/test/registry.test.ts` covers the component registry: field validation, `instanceof` hierarchy, `REGISTRY` lookup, and `SIDEBAR_ITEMS` consistency.
