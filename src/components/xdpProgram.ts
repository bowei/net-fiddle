import { Zap } from 'lucide-react';
import { BpfBase } from './bpfBase';

export class XdpProgramComponent extends BpfBase {
  readonly type = 'xdp-program';
  readonly label = 'XDP Program';
  readonly typeLabel = 'xdp';
  readonly icon = Zap;
  readonly configTitle = 'XDP Config';
  readonly configItems = [
    'BPF_PROG_TYPE_XDP — before skb allocation',
    'XDP_PASS / DROP / TX / REDIRECT',
    'Native, offloaded, or generic mode',
  ] as const;
  // XDP is strictly ingress-only; it runs before TC and before skb creation.
  readonly anchors = [
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'in' as const },
    { side: 'S', count: 1, flow: 'ingress' as const, connector: 'out' as const },
  ] as const;
}

export const xdpProgram = new XdpProgramComponent();
