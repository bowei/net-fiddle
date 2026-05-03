import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';

export class NetkitBpfIngressComponent extends BpfBase {
  readonly type = 'netkit-bpf-ingress';
  readonly label = 'Netkit BPF Ingress';
  readonly typeLabel = 'netkit-bpf-in';
  readonly icon = Zap;
  readonly configTitle = 'Netkit BPF Ingress';
  readonly configItems = [
    'BPF_PROG_TYPE_NETKIT, ingress hook',
    'Runs on packets arriving from the peer end',
    'Connect S → netkit W (ingress hook anchor)',
    'TC_ACT_OK / SHOT / REDIRECT',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'in'  as const },
    { side: 'S', count: 1, flow: 'ingress' as const, connector: 'out' as const },
  ] as const;
}

export const netkitBpfIngress = new NetkitBpfIngressComponent();
