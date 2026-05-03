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

  // Group handles by dynamic side.
  const bySide = new Map<CardinalSide, typeof withDynSide>();
  for (const h of withDynSide) {
    if (!bySide.has(h.dynSide)) bySide.set(h.dynSide, []);
    bySide.get(h.dynSide)!.push(h);
  }

  // Sort each side's handles by the connected peer's coordinate along that side
  // (x for N/S, y for E/W) to minimise line crossings. Handles with no connection
  // use this node's own centre coordinate so stable sort preserves their order.
  for (const [side, group] of bySide) {
    const isNS = side === 'N' || side === 'S';
    group.sort((a, b) => {
      const ca = connectedCenter.get(a.id);
      const cb = connectedCenter.get(b.id);
      const ka = ca ? (isNS ? ca.x : ca.y) : isNS ? thisCx : thisCy;
      const kb = cb ? (isNS ? cb.x : cb.y) : isNS ? thisCx : thisCy;
      return ka - kb;
    });
  }

  // Build dynIdx from sorted position within each side group.
  const dynIdxMap = new Map<string, number>();
  for (const group of bySide.values()) group.forEach((h, i) => dynIdxMap.set(h.id, i));

  const positioned = withDynSide.map((h) => ({
    ...h,
    dynIdx: dynIdxMap.get(h.id)!,
    dynTotal: bySide.get(h.dynSide)!.length,
  }));

  // Tell ReactFlow to re-measure handle DOM positions whenever side OR position-along-side changes.
  // dynIdx/dynTotal affect the CSS left/top %, which moves the dot without remounting the Handle,
  // so we must also retrigger here — not just when dynSide changes.
  const updateNodeInternals = useUpdateNodeInternals();
  const dynKey = positioned.map((h) => `${h.id}:${h.dynSide}:${h.dynIdx}:${h.dynTotal}`).join(',');
  useEffect(() => {
    updateNodeInternals(id);
  }, [dynKey]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{ borderColor: def.borderColor, backgroundColor: def.bgColor, color: def.color }}
    >
      {positioned.flatMap(({ id: hid, flow, connector, dynSide, dynIdx, dynTotal }) => {
        const color = anchorFlowColor(flow, def.color);
        const base = { ...HANDLE_SIZE, ...anchorStyle(dynSide, dynIdx, dynTotal) };
        const solidStyle = { ...base, background: color, border: `2px solid ${color}` };
        const hollowStyle = { ...base, background: 'transparent', border: `2px solid ${color}` };
        const pos = toRFPosition(dynSide);
        const els: ReactElement[] = [];
        // Include dynSide in key so React remounts the Handle when the side changes,
        // forcing ReactFlow to re-register the handle position in its internal store.
        if (connector !== 'out')
          els.push(
            <Handle
              key={`${hid}-${dynSide}-t`}
              id={`${hid}-t`}
              type="target"
              position={pos}
              style={hollowStyle}
              isConnectableStart={false}
            />
          );
        if (connector !== 'in')
          els.push(
            <Handle
              key={`${hid}-${dynSide}-s`}
              id={`${hid}-s`}
              type="source"
              position={pos}
              style={solidStyle}
            />
          );
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
