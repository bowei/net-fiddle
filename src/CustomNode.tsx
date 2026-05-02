import { memo, type ReactElement } from 'react';
import { Handle, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';
import { resolveHandles, anchorStyle, anchorFlowColor, toRFPosition } from './anchorUtils';

export interface NetNodeData {
  nodeType: string;
  label: string;
  config?: Record<string, string>;
  /** Set on veth-end nodes to identify the paired partner. */
  vethPairId?: string;
}

const HANDLE_SIZE = { width: 10, height: 10 };

function CustomNode({ data, selected }: NodeProps<NetNodeData>) {
  const def = REGISTRY.get(data.nodeType);
  if (!def) return null;

  const Icon = def.icon;
  const handles = resolveHandles(def.getAnchors(data.config ?? {}));

  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{ borderColor: def.borderColor, backgroundColor: def.bgColor, color: def.color }}
    >
      {handles.flatMap(({ id, side, flow, connector, globalIndex, totalOnSide }) => {
        const color = anchorFlowColor(flow, def.color);
        const base = { ...HANDLE_SIZE, ...anchorStyle(side, globalIndex, totalOnSide) };
        const solidStyle  = { ...base, background: color,         border: `2px solid ${color}` };
        const hollowStyle = { ...base, background: 'transparent', border: `2px solid ${color}` };
        const pos = toRFPosition(side);
        const els: ReactElement[] = [];
        if (connector !== 'out') els.push(<Handle key={`${id}-t`} id={`${id}-t`} type="target" position={pos} style={hollowStyle} />);
        if (connector !== 'in')  els.push(<Handle key={`${id}-s`} id={`${id}-s`} type="source" position={pos} style={solidStyle} />);
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
