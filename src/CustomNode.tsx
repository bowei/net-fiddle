import { memo, type ReactElement, useEffect } from 'react';
import { Handle, useNodes, useEdges, useUpdateNodeInternals, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';
import { resolveHandles, anchorStyle, anchorFlowColor, toRFPosition } from './anchorUtils';
import type { CardinalSide } from './components/base';

export interface NetNodeData {
  nodeType: string;
  label: string;
  config?: Record<string, string>;
  /** Set on veth-end nodes to identify the paired partner. */
  vethPairId?: string;
}

const HANDLE_SIZE = { width: 10, height: 10 };
const FALLBACK_W = 140;
const FALLBACK_H = 60;

/** Returns the side of a rectangle that a ray from its center toward (dx, dy) exits through. */
function closestSide(dx: number, dy: number, hw: number, hh: number): CardinalSide {
  if (Math.abs(dx / hw) > Math.abs(dy / hh)) return dx > 0 ? 'E' : 'W';
  return dy > 0 ? 'S' : 'N';
}

function CustomNode({ id, data, selected }: NodeProps<NetNodeData>) {
  const def = REGISTRY.get(data.nodeType);
  const allNodes = useNodes<NetNodeData>();
  const allEdges = useEdges();

  if (!def) return null;

  const Icon = def.icon;
  const handles = resolveHandles(def.getAnchors(data.config ?? {}));

  // Locate this node's absolute center.
  const thisNode = allNodes.find((n) => n.id === id);
  const thisPos = thisNode?.positionAbsolute ?? thisNode?.position ?? { x: 0, y: 0 };
  const thisW = thisNode?.width ?? FALLBACK_W;
  const thisH = thisNode?.height ?? FALLBACK_H;
  const thisCx = thisPos.x + thisW / 2;
  const thisCy = thisPos.y + thisH / 2;

  // Build map: base handle id → absolute center of the connected node.
  const connectedCenter = new Map<string, { x: number; y: number }>();
  for (const edge of allEdges) {
    const center = (nodeId: string) => {
      const n = allNodes.find((nd) => nd.id === nodeId);
      if (!n) return null;
      const p = n.positionAbsolute ?? n.position;
      return { x: p.x + (n.width ?? FALLBACK_W) / 2, y: p.y + (n.height ?? FALLBACK_H) / 2 };
    };
    if (edge.source === id && edge.sourceHandle) {
      const c = center(edge.target);
      if (c) connectedCenter.set(edge.sourceHandle.replace(/-[st]$/, ''), c);
    }
    if (edge.target === id && edge.targetHandle) {
      const c = center(edge.source);
      if (c) connectedCenter.set(edge.targetHandle.replace(/-[st]$/, ''), c);
    }
  }

  // Assign each handle a dynamic side (closest to its connected peer, or default).
  const withDynSide = handles.map((h) => {
    const cc = connectedCenter.get(h.id);
    const dynSide: CardinalSide = cc
      ? closestSide(cc.x - thisCx, cc.y - thisCy, thisW / 2, thisH / 2)
      : h.side;
    return { ...h, dynSide };
  });

  // Count how many handles land on each dynamic side for even spacing.
  const sideTotals = new Map<CardinalSide, number>();
  for (const h of withDynSide) sideTotals.set(h.dynSide, (sideTotals.get(h.dynSide) ?? 0) + 1);

  // Assign per-side index in declaration order.
  const sideIdx = new Map<CardinalSide, number>();
  const positioned = withDynSide.map((h) => {
    const idx = sideIdx.get(h.dynSide) ?? 0;
    sideIdx.set(h.dynSide, idx + 1);
    return { ...h, dynIdx: idx, dynTotal: sideTotals.get(h.dynSide)! };
  });

  // Tell ReactFlow to re-measure handle DOM positions whenever side assignments change.
  // Without this the edge-routing store retains stale coordinates.
  const updateNodeInternals = useUpdateNodeInternals();
  const dynSidesKey = positioned.map((h) => `${h.id}:${h.dynSide}`).join(',');
  useEffect(() => { updateNodeInternals(id); }, [dynSidesKey]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{ borderColor: def.borderColor, backgroundColor: def.bgColor, color: def.color }}
    >
      {positioned.flatMap(({ id: hid, flow, connector, dynSide, dynIdx, dynTotal }) => {
        const color = anchorFlowColor(flow, def.color);
        const base = { ...HANDLE_SIZE, ...anchorStyle(dynSide, dynIdx, dynTotal) };
        const solidStyle  = { ...base, background: color,         border: `2px solid ${color}` };
        const hollowStyle = { ...base, background: 'transparent', border: `2px solid ${color}` };
        const pos = toRFPosition(dynSide);
        const els: ReactElement[] = [];
        // Include dynSide in key so React remounts the Handle when the side changes,
        // forcing ReactFlow to re-register the handle position in its internal store.
        if (connector !== 'out') els.push(<Handle key={`${hid}-${dynSide}-t`} id={`${hid}-t`} type="target" position={pos} style={hollowStyle} />);
        if (connector !== 'in')  els.push(<Handle key={`${hid}-${dynSide}-s`} id={`${hid}-s`} type="source" position={pos} style={solidStyle} />);
        return els;
      })}

      <div className="node-header">
        <div className="node-icon">
          <Icon size={16} color={def.color} />
        </div>
        <span className="node-label">{data.label}</span>
      </div>
      <div className="node-type-label">{def.typeLabel}</div>
    </div>
  );
}

export default memo(CustomNode);
