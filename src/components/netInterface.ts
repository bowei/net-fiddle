import { Cable } from 'lucide-react';
import { ComponentDef } from './base';

export class NetInterfaceComponent extends ComponentDef {
  readonly type = 'interface';
  readonly label = 'Interface';
  readonly typeLabel = 'interface';
  readonly color = '#0d9488';
  readonly bgColor = '#f0fdfa';
  readonly borderColor = '#5eead4';
  readonly icon = Cable;
  readonly configTitle = 'Interface Config';
  readonly configItems = [
    'Physical/Virtual interface',
    'veth pairs, bridges, etc.',
    'MTU, MAC address settings',
  ] as const;
  // S: wire (physical connection to network or peer interface).
  // W: ingress path exits left toward XDP → TC-rx → nftables prerouting.
  // E: egress path enters right from TC-tx → qdisc.
  readonly anchors = [
    { side: 'S', count: 1, flow: 'any' as const,     connector: 'both' as const },
    { side: 'W', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'E', count: 1, flow: 'egress' as const,  connector: 'in' as const },
  ] as const;
}

export const netInterface = new NetInterfaceComponent();
