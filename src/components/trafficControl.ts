import { Zap } from 'lucide-react';
import { ComponentDef } from './base';

export class TrafficControlComponent extends ComponentDef {
  readonly type = 'traffic-control';
  readonly label = 'Traffic Control';
  readonly typeLabel = 'tc';
  readonly color = '#d97706';
  readonly bgColor = '#fffbeb';
  readonly borderColor = '#fcd34d';
  readonly icon = Zap;
  readonly configTitle = 'TC Config';
  readonly configItems = [
    'Rate limiting (tbf, htb)',
    'Queue disciplines',
    'Packet scheduling',
  ] as const;
  // Single ingress, single egress.
  readonly anchors = [
    { side: 'N', count: 1 },
    { side: 'S', count: 1 },
  ] as const;
}

export const trafficControl = new TrafficControlComponent();
