import { memo } from 'react';
import { Handle, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';
import { resolveHandles, anchorStyle, anchorFlowColor, toRFPosition } from './anchorUtils';

export interface NetNodeData {
  nodeType: string;
  label: string;
  config?: Record<string, string>;
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
      {handles.flatMap(({ id, side, flow, globalIndex, totalOnSide }) => {
        const color = anchorFlowColor(flow, def.color);
        const style = { ...HANDLE_SIZE, background: color, ...anchorStyle(side, globalIndex, totalOnSide) };
        return [
          <Handle key={`${id}-t`} id={`${id}-t`} type="target" position={toRFPosition(side)} style={style} />,
          <Handle key={`${id}-s`} id={`${id}-s`} type="source" position={toRFPosition(side)} style={style} />,
        ];
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
