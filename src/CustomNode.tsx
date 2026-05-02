import { memo } from 'react';
import { Handle, Position, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';

export interface NetNodeData {
  nodeType: string;
  label: string;
}

function CustomNode({ data, selected }: NodeProps<NetNodeData>) {
  const def = REGISTRY.get(data.nodeType);
  if (!def) return null;

  const Icon = def.icon;
  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{
        borderColor: def.borderColor,
        backgroundColor: def.bgColor,
        color: def.color,
      }}
    >
      <Handle type="target" position={Position.Top} style={{ background: def.color, width: 8, height: 8 }} />
      <div className="node-header">
        <div className="node-icon">
          <Icon size={16} color={def.color} />
        </div>
        <span className="node-label">{data.label}</span>
      </div>
      <div className="node-type-label">{def.typeLabel}</div>
      <Handle type="source" position={Position.Bottom} style={{ background: def.color, width: 8, height: 8 }} />
    </div>
  );
}

export default memo(CustomNode);
