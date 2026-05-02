import { NftablesBase } from './nftablesBase';

export class NftablesPreroutingComponent extends NftablesBase {
  readonly type = 'nftables-prerouting';
  readonly label = 'nft prerouting';
  readonly typeLabel = 'nft:pre';
  readonly configItems = [
    'NF_INET_PRE_ROUTING hook',
    'DNAT, conntrack init',
    'First netfilter hook on ingress',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'ingress' as const },
    { side: 'S', count: 1, flow: 'ingress' as const },
  ] as const;
}

export const nftablesPrerouting = new NftablesPreroutingComponent();
