import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';

export class NetkitBpfEgressComponent extends BpfBase {
  readonly type = 'netkit-bpf-egress';
  readonly label = 'Netkit BPF Egress';
  readonly typeLabel = 'netkit-bpf-out';
  readonly icon = Zap;
  readonly configTitle = 'Netkit BPF Egress';
  readonly configItems = [
    'BPF_PROG_TYPE_NETKIT, egress hook',
    'Runs on packets departing toward the peer end',
    'Connect S → netkit E (egress hook anchor)',
    'TC_ACT_OK / SHOT / REDIRECT',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress' as const, connector: 'in'  as const },
    { side: 'S', count: 1, flow: 'egress' as const, connector: 'out' as const },
  ] as const;
}

export const netkitBpfEgress = new NetkitBpfEgressComponent();
