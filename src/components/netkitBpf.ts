import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';
import type { AnchorSpec } from './base';

export class NetkitBpfComponent extends BpfBase {
  readonly type = 'netkit-bpf';
  readonly label = 'Netkit BPF';
  readonly typeLabel = 'netkit-bpf';
  readonly icon = Zap;
  readonly configTitle = 'Netkit BPF Config';
  readonly configItems = [
    'BPF_PROG_TYPE_NETKIT (kernel ≥ 6.7)',
    'Attaches to W (ingress) or E (egress) hook on a netkit end',
    'TC_ACT_OK / SHOT / REDIRECT',
    'Full sk_buff access; can modify headers',
  ] as const;
  readonly configFields = [
    {
      key: 'direction',
      label: 'Direction',
      options: ['ingress', 'egress'] as const,
      default: 'ingress',
    },
  ] as const;
  readonly anchors = [] as const;

  getAnchors(config: Record<string, string>): readonly AnchorSpec[] {
    const flow = config.direction === 'egress' ? 'egress' : 'ingress';
    return [
      { side: 'S', count: 1, flow, connector: 'out' as const },
    ];
  }
}

export const netkitBpf = new NetkitBpfComponent();
