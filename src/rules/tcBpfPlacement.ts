import type { GraphRule, GraphViolation } from './types';

const TC_BPF_TYPES = new Set(['tc-bpf-program', 'sched-bpf']);

export const tcBpfPlacementRule: GraphRule = {
  id: 'tc-bpf-placement',
  name: 'TC BPF Attachment',
  check(nodes, edges) {
    const violations: GraphViolation[] = [];
    const nodeById = new Map(nodes.map((n) => [n.id, n]));

    for (const edge of edges) {
      if (edge.data?.vethLink) continue;
      const src = nodeById.get(edge.source);
      const tgt = nodeById.get(edge.target);
      if (!src || !tgt) continue;

      const srcType = src.data.nodeType;
      const tgtType = tgt.data.nodeType;

      if (TC_BPF_TYPES.has(srcType) && tgtType !== 'traffic-control') {
        violations.push({
          edgeId: edge.id,
          message: `"${src.data.label}" can only connect to a Traffic Control node, not "${tgt.data.label}"`,
          severity: 'error',
        });
      }
      if (TC_BPF_TYPES.has(tgtType) && srcType !== 'traffic-control') {
        violations.push({
          edgeId: edge.id,
          message: `"${tgt.data.label}" can only connect to a Traffic Control node, not "${src.data.label}"`,
          severity: 'error',
        });
      }
    }

    return violations;
  },
};
