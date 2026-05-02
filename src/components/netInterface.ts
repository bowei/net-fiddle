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
}

export const netInterface = new NetInterfaceComponent();
