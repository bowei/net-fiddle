import type { GraphRule, GraphViolation } from './types';

const NETKIT_ENDS = new Set(['netkit-primary', 'netkit-peer']);
const ALLOWED_NEIGHBORS = new Set(['netkit-primary', 'netkit-peer', 'xdp-program']);

export const netkitBpfPlacementRule: GraphRule = {
  id: 'netkit-bpf-placement',
  name: 'Netkit BPF Attachment',
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

      const srcIsNetkitBpf = srcType === 'netkit-bpf-ingress' || srcType === 'netkit-bpf-egress';
      const tgtIsNetkitBpf = tgtType === 'netkit-bpf-ingress' || tgtType === 'netkit-bpf-egress';

      if (srcIsNetkitBpf && !ALLOWED_NEIGHBORS.has(tgtType)) {
        violations.push({
          edgeId: edge.id,
          message: `"${src.data.label}" can only connect to a netkit interface or XDP, not "${tgt.data.label}"`,
          severity: 'error',
        });
      }
      if (tgtIsNetkitBpf && !ALLOWED_NEIGHBORS.has(srcType)) {
        violations.push({
          edgeId: edge.id,
          message: `"${tgt.data.label}" can only connect to a netkit interface or XDP, not "${src.data.label}"`,
          severity: 'error',
        });
      }

      // Netkit interface W/E hook anchors may only receive netkit BPF programs
      if (NETKIT_ENDS.has(srcType) && edge.sourceHandle) {
        const base = edge.sourceHandle.replace(/-[st]$/, '');
        if ((base === 'W-0' || base === 'E-0') && !tgtIsNetkitBpf) {
          violations.push({
            edgeId: edge.id,
            message: `Netkit hook anchor on "${src.data.label}" can only connect to a Netkit BPF program`,
            severity: 'error',
          });
        }
      }
      if (NETKIT_ENDS.has(tgtType) && edge.targetHandle) {
        const base = edge.targetHandle.replace(/-[st]$/, '');
        if ((base === 'W-0' || base === 'E-0') && !srcIsNetkitBpf) {
          violations.push({
            edgeId: edge.id,
            message: `Netkit hook anchor on "${tgt.data.label}" can only connect to a Netkit BPF program`,
            severity: 'error',
          });
        }
      }
    }

    return violations;
  },
};
