import { NftablesBase } from './nftablesBase';

export class NftablesForwardComponent extends NftablesBase {
  readonly type = 'nftables-forward';
  readonly label = 'nftables forward';
  readonly typeLabel = 'nft:fwd';
  readonly configItems = [
    'NF_INET_FORWARD hook',
    'Forwarding policy, conntrack matching',
    'Transit traffic (not locally delivered)',
  ] as const;
  // Forward sits between ingress and egress paths; 'any' because it processes
  // packets that entered as ingress and leave as egress.
  readonly anchors = [
    { side: 'N', count: 1, flow: 'any' as const, connector: 'in' as const },
    { side: 'S', count: 1, flow: 'any' as const, connector: 'out' as const },
  ] as const;
}

export const nftablesForward = new NftablesForwardComponent();
