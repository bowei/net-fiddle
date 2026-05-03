import type { GraphRule, GraphViolation } from './types';
import { resolveEdgeHandleFlows } from './utils';
import type { AnchorFlow } from '../components/base';

function ingressOrder(nodeType: string, config?: Record<string, string>): number | null {
  switch (nodeType) {
    case 'interface':
    case 'veth-end':             return 0;
    case 'xdp-program':          return 1;
    case 'traffic-control':
    case 'tc-bpf-program':       return config?.direction === 'ingress' ? 2 : null;
    case 'nftables-prerouting':  return 3;
    case 'routing-table':        return 4;
    case 'nftables-input':
    case 'nftables-forward':     return 5;
    case 'socket':               return 6;
    default:                     return null;
  }
}

function egressOrder(nodeType: string, config?: Record<string, string>): number | null {
  switch (nodeType) {
    case 'socket':                  return 0;
    case 'nftables-output':         return 1;
    case 'nftables-postrouting':    return 2;
    case 'traffic-control':
    case 'tc-bpf-program':          return config?.direction === 'egress' ? 3 : null;
    case 'sched-bpf':               return 3;
    case 'qdisc':                   return 4;
    case 'interface':
    case 'veth-end':                return 5;
    default:                        return null;
  }
}

export const linuxOrderRule: GraphRule = {
  id: 'linux-order',
  name: 'Linux Network Stack Ordering',
  check(nodes, edges) {
    const violations: GraphViolation[] = [];
    const nodeById = new Map(nodes.map((n) => [n.id, n]));

    for (const edge of edges) {
      if (edge.data?.vethLink) continue;
      const flows = resolveEdgeHandleFlows(edge, nodes);
      if (!flows) continue;

      const flow: AnchorFlow =
        flows.src !== 'any' ? flows.src :
        flows.tgt !== 'any' ? flows.tgt : 'any';
      if (flow === 'any') continue;

      const srcNode = nodeById.get(edge.source);
      const tgtNode = nodeById.get(edge.target);
      if (!srcNode || !tgtNode) continue;

      const orderFn = flow === 'ingress' ? ingressOrder : egressOrder;
      const srcOrd = orderFn(srcNode.data.nodeType, srcNode.data.config);
      const tgtOrd = orderFn(tgtNode.data.nodeType, tgtNode.data.config);

      if (srcOrd === null || tgtOrd === null) continue;

      if (srcOrd >= tgtOrd) {
        violations.push({
          edgeId: edge.id,
          message: `Linux ${flow} ordering violation: "${srcNode.data.label}" (step ${srcOrd}) cannot feed "${tgtNode.data.label}" (step ${tgtOrd})`,
          severity: 'error',
        });
      }
    }
    return violations;
  },
};
