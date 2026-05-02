import { memo } from 'react';
import { Handle, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';
import { anchorStyle, anchorHandleId, toRFPosition } from './anchorUtils';

export interface NetNodeData {
  nodeType: string;
  label: string;
}

const HANDLE_BASE_STYLE = { width: 8, height: 8 };

function CustomNode({ data, selected }: NodeProps<NetNodeData>) {
  const def = REGISTRY.get(data.nodeType);
  if (!def) return null;

  const Icon = def.icon;
  const handleColor = { background: def.color };

  return (
    <div
      className={`custom-node${selected ? ' selected' : ''}`}
      style={{ borderColor: def.borderColor, backgroundColor: def.bgColor, color: def.color }}
    >
      {def.anchors.flatMap(({ side, count }) =>
        Array.from({ length: count }, (_, i) => {
          const id = anchorHandleId(side, i);
          const style = { ...HANDLE_BASE_STYLE, ...handleColor, ...anchorStyle(side, i, count) };
          return [
            <Handle key={`${id}-t`} id={`${id}-t`} type="target" position={toRFPosition(side)} style={style} />,
            <Handle key={`${id}-s`} id={`${id}-s`} type="source" position={toRFPosition(side)} style={style} />,
          ];
        })
      )}

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
