import { Code2 } from 'lucide-react';
import { BpfBase } from './bpfBase';
import type { AnchorSpec } from './base';

export class TcBpfProgramComponent extends BpfBase {
  readonly type = 'tc-bpf-program';
  readonly label = 'TC BPF Program';
  readonly typeLabel = 'tc-bpf';
  readonly icon = Code2;
  readonly configTitle = 'TC BPF Config';
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
      { side: 'N', count: 1, flow, connector: 'in' as const },
      { side: 'S', count: 1, flow, connector: 'out' as const },
    ];
  }
}

export const tcBpfProgram = new TcBpfProgramComponent();
