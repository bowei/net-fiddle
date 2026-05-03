import type { GraphRule, GraphViolation } from './types';

const INTERFACE_TYPES = new Set(['interface', 'veth-end', 'netkit-primary', 'netkit-peer']);

export const xdpPlacementRule: GraphRule = {
  id: 'xdp-placement',
  name: 'XDP Placement',
  check(nodes, edges) {
    const violations: GraphViolation[] = [];
    const nodeById = new Map(nodes.map((n) => [n.id, n]));

    for (const edge of edges) {
      if (edge.data?.vethLink) continue;
      const src = nodeById.get(edge.source);
      const tgt = nodeById.get(edge.target);
      if (!src || !tgt) continue;

      if (tgt.data.nodeType === 'xdp-program' && !INTERFACE_TYPES.has(src.data.nodeType)) {
        violations.push({
          edgeId: edge.id,
          message: `XDP must be placed immediately after an interface, not "${src.data.label}"`,
          severity: 'error',
        });
      }
    }

    return violations;
  },
};
