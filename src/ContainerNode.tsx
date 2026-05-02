import { memo, useContext } from 'react';
import { NodeResizer, type NodeProps } from 'reactflow';
import { REGISTRY } from './components/registry';
import { ContainerComponentDef } from './components/base';
import { DragContext } from './DragContext';
import type { NetNodeData } from './CustomNode';

function ContainerNode({ id, data, selected }: NodeProps<NetNodeData>) {
  const hoverContainerId = useContext(DragContext);
  const def = REGISTRY.get(data.nodeType);
  if (!def || !(def instanceof ContainerComponentDef)) return null;

  const Icon = def.icon;
  const isDropTarget = hoverContainerId === id;

  const classes = ['container-node', selected && 'selected', isDropTarget && 'drop-target']
    .filter(Boolean)
    .join(' ');

  return (
    <>
      <NodeResizer
        color={def.color}
        isVisible={selected ?? false}
        minWidth={def.minWidth}
        minHeight={def.minHeight}
      />
      <div
        className={classes}
        style={{
          borderColor: isDropTarget ? def.color : def.borderColor,
          backgroundColor: def.bgColor + (isDropTarget ? 'cc' : '55'),
          color: def.color,
        }}
      >
        <div className="container-title">
          <Icon size={14} color={def.color} />
          <span>{data.label}</span>
        </div>
      </div>
    </>
  );
}

export default memo(ContainerNode);
