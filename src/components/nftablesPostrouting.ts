import { NftablesBase } from './nftablesBase';

export class NftablesPostroutingComponent extends NftablesBase {
  readonly type = 'nftables-postrouting';
  readonly label = 'nftables postrouting';
  readonly typeLabel = 'nft:post';
  readonly configItems = [
    'NF_INET_POST_ROUTING hook',
    'Masquerade, SNAT',
    'Last netfilter hook before qdisc',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress' as const, connector: 'in' as const },
    { side: 'S', count: 1, flow: 'egress' as const, connector: 'out' as const },
  ] as const;
}

export const nftablesPostrouting = new NftablesPostroutingComponent();
