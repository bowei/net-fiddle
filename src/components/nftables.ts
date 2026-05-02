import { Shield } from 'lucide-react';
import { ComponentDef } from './base';

export class NftablesComponent extends ComponentDef {
  readonly type = 'nftables';
  readonly label = 'nftables';
  readonly typeLabel = 'nftables';
  readonly color = '#f97316';
  readonly bgColor = '#fff7ed';
  readonly borderColor = '#fdba74';
  readonly icon = Shield;
  readonly configTitle = 'nftables Config';
  readonly configItems = [
    'Firewall rules',
    'NAT configuration',
    'Packet filtering',
  ] as const;
  // Packets enter from the top, exit from the bottom.
  readonly anchors = [
    { side: 'N', count: 1 },
    { side: 'S', count: 1 },
  ] as const;
}

export const nftables = new NftablesComponent();
