import { Globe } from 'lucide-react';
import { ComponentDef } from './base';

export class NetworkComponent extends ComponentDef {
  readonly type = 'network';
  readonly label = 'Network';
  readonly typeLabel = 'network';
  readonly color = '#64748b';
  readonly bgColor = '#f8fafc';
  readonly borderColor = '#cbd5e1';
  readonly icon = Globe;
  readonly configTitle = 'External Network';
  readonly configItems = [
    'Represents an external network entity (internet, upstream router, etc.)',
    'Top-left connector: source of ingress traffic',
    'Top-right connector: destination of egress traffic',
  ] as const;
  // Two handles on the N side: ingress source (solid) and egress destination (hollow).
  readonly anchors = [
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'N', count: 1, flow: 'egress'  as const, connector: 'in'  as const },
  ] as const;
}

export const network = new NetworkComponent();
