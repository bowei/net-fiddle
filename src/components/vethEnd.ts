import { Cable } from 'lucide-react';
import { ComponentDef } from './base';

export class VethEndComponent extends ComponentDef {
  readonly type = 'veth-end';
  readonly label = 'Veth End';
  readonly typeLabel = 'veth';
  readonly color = '#0d9488';
  readonly bgColor = '#f0fdfa';
  readonly borderColor = '#5eead4';
  readonly icon = Cable;
  readonly configTitle = 'Veth End Config';
  readonly configItems = [
    'One end of a virtual Ethernet pair',
    'Peer link is permanent (cannot be reconnected)',
    'Deleting either end removes both',
  ] as const;
  // S: peer link to the other veth end (locked edge).
  // W: ingress path exits left.
  // E: egress path enters right.
  readonly anchors = [
    { side: 'S', count: 1, flow: 'any' as const,     connector: 'both' as const },
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'N', count: 1, flow: 'egress' as const,  connector: 'in' as const },
  ] as const;
}

export const vethEnd = new VethEndComponent();
