import { NftablesBase } from './nftablesBase';

export class NftablesInputComponent extends NftablesBase {
  readonly type = 'nftables-input';
  readonly label = 'nftables input';
  readonly typeLabel = 'nft:input';
  readonly configItems = [
    'NF_INET_LOCAL_IN hook',
    'Firewall for locally-destined traffic',
    'Rate limiting, port filtering',
  ] as const;
  readonly anchors = [
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'S', count: 1, flow: 'ingress' as const, connector: 'in' as const },
  ] as const;
}

export const nftablesInput = new NftablesInputComponent();
