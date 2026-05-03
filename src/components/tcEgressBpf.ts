import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';

export class SchedBpfComponent extends BpfBase {
  readonly type = 'sched-bpf';
  readonly label = 'tc egress';
  readonly typeLabel = 'sched-bpf';
  readonly icon = Zap;
  readonly configTitle = 'TC Sched BPF Config';
  readonly configItems = [
    'BPF_PROG_TYPE_SCHED_CLS or SCHED_ACT',
    'Attaches via clsact qdisc, egress hook',
    'Runs before qdisc enqueue (sch_handle_egress)',
    'TC_ACT_OK / SHOT / REDIRECT / PIPE',
    'Full sk_buff access; can modify headers',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress', connector: 'in' },
    { side: 'S', count: 1, flow: 'egress', connector: 'out' },
  ] as const;
}

export const schedBpf = new SchedBpfComponent();
