import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';
import type { AnchorSpec } from './base';

export class TcBpfProgramComponent extends BpfBase {
  readonly type = 'tc-bpf-program';
  readonly label = 'tc ingress';
  readonly typeLabel = 'tc-bpf';
  readonly icon = Zap;
  readonly configTitle = 'TC BPF';
  readonly configItems = [
    'BPF_PROG_TYPE_SCHED_CLS via clsact',
    'TC_ACT_OK / SHOT / REDIRECT',
    'Full sk_buff access; ingress or egress',
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
      { side: 'N', count: 1, flow, connector: 'out' as const },
      { side: 'S', count: 1, flow, connector: 'in' as const },
    ];
  }
}

export const tcBpfProgram = new TcBpfProgramComponent();
