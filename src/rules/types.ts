import type { Edge, Node } from 'reactflow';
import type { NetNodeData } from '../CustomNode';

export interface GraphViolation {
  edgeId?: string;
  nodeId?: string;
  message: string;
  severity: 'error' | 'warning';
}

export interface GraphRule {
  readonly id: string;
  readonly name: string;
  check(nodes: Node<NetNodeData>[], edges: Edge[]): GraphViolation[];
}
