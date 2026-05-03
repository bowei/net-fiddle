import type { GraphRule, GraphViolation } from './types';
import { resolveEdgeHandleFlows } from './utils';

export const flowMismatchRule: GraphRule = {
  id: 'flow-mismatch',
  name: 'Flow Direction Mismatch',
  check(nodes, edges) {
    const violations: GraphViolation[] = [];
    for (const edge of edges) {
      if (edge.data?.vethLink) continue;
      const flows = resolveEdgeHandleFlows(edge, nodes);
      if (!flows) continue;
      if (flows.src !== 'any' && flows.tgt !== 'any' && flows.src !== flows.tgt) {
        violations.push({
          edgeId: edge.id,
          message: `Flow mismatch: source is ${flows.src} but target expects ${flows.tgt}`,
          severity: 'error',
        });
      }
    }
    return violations;
  },
};
