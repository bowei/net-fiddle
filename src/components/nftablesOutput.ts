import { NftablesBase } from './nftablesBase';

export class NftablesOutputComponent extends NftablesBase {
  readonly type = 'nftables-output';
  readonly label = 'nft output';
  readonly typeLabel = 'nft:out';
  readonly configItems = [
    'NF_INET_LOCAL_OUT hook',
    'Output policy, SNAT',
    'First netfilter hook on egress path',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress' as const },
    { side: 'S', count: 1, flow: 'egress' as const },
  ] as const;
}

export const nftablesOutput = new NftablesOutputComponent();
