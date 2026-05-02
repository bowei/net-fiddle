import { Terminal } from 'lucide-react';
import { ComponentDef } from './base';

export class SocketComponent extends ComponentDef {
  readonly type = 'socket';
  readonly label = 'Socket / Process';
  readonly typeLabel = 'socket';
  readonly color = '#16a34a';
  readonly bgColor = '#f0fdf4';
  readonly borderColor = '#86efac';
  readonly icon = Terminal;
  readonly configTitle = 'Socket Config';
  readonly configItems = [
    'TCP / UDP / RAW socket',
    'send() / recv() application boundary',
    'sk_buff placed in socket receive buffer',
  ] as const;
  // N: egress packets leave upward toward nft output.
  // S: ingress packets arrive from below after nft input.
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress' as const,  connector: 'out' as const },
    { side: 'S', count: 1, flow: 'ingress' as const, connector: 'in' as const },
  ] as const;
}

export const socket = new SocketComponent();
