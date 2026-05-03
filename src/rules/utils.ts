import type { Edge, Node } from 'reactflow';
import { REGISTRY } from '../components/registry';
import { resolveHandles } from '../anchorUtils';
import type { AnchorFlow } from '../components/base';
import type { NetNodeData } from '../CustomNode';

export function resolveEdgeHandleFlows(
  edge: Edge,
  nodes: Node<NetNodeData>[]
): { src: AnchorFlow; tgt: AnchorFlow } | null {
  if (!edge.sourceHandle || !edge.targetHandle) return null;
  const src = nodes.find((n) => n.id === edge.source);
  const tgt = nodes.find((n) => n.id === edge.target);
  if (!src || !tgt) return null;
  const srcDef = REGISTRY.get(src.data.nodeType);
  const tgtDef = REGISTRY.get(tgt.data.nodeType);
  if (!srcDef || !tgtDef) return null;
  const srcHandleId = edge.sourceHandle.replace(/-[st]$/, '');
  const tgtHandleId = edge.targetHandle.replace(/-[st]$/, '');
  const srcHandle = resolveHandles(srcDef.getAnchors(src.data.config ?? {})).find(
    (h) => h.id === srcHandleId
  );
  const tgtHandle = resolveHandles(tgtDef.getAnchors(tgt.data.config ?? {})).find(
    (h) => h.id === tgtHandleId
  );
  if (!srcHandle || !tgtHandle) return null;
  return { src: srcHandle.flow, tgt: tgtHandle.flow };
}
