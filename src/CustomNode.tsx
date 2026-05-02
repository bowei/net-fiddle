import { memo } from 'react';
import { Handle, Position, type NodeProps } from 'reactflow';
import { NODE_CONFIG, type NodeType } from './nodeConfig';

export interface NetNodeData {
  nodeType: NodeType;
  label: string;
  selected?: boolean;
}

function CustomNode({ data, selected }: NodeProps<NetNodeData>) {
  const cfg = NODE_CONFIG[data.nodeType];
  const { Icon } = cfg;

  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{
        borderColor: cfg.borderColor,
        backgroundColor: cfg.bgColor,
        color: cfg.color,
      }}
    >
      <Handle type="target" position={Position.Top} style={{ background: cfg.color, width: 8, height: 8 }} />
      <div className="node-header">
        <div className="node-icon">
          <Icon size={16} color={cfg.color} />
        </div>
        <span className="node-label">{data.label}</span>
      </div>
      <div className="node-type-label">{cfg.typeLabel}</div>
      <Handle type="source" position={Position.Bottom} style={{ background: cfg.color, width: 8, height: 8 }} />
    </div>
  );
}

export default memo(CustomNode);
