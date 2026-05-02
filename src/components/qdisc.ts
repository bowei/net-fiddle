import { Layers } from 'lucide-react';
import { ComponentDef } from './base';

export class QdiscComponent extends ComponentDef {
  readonly type = 'qdisc';
  readonly label = 'Qdisc';
  readonly typeLabel = 'qdisc';
  readonly color = '#0891b2';
  readonly bgColor = '#ecfeff';
  readonly borderColor = '#67e8f9';
  readonly icon = Layers;
  readonly configTitle = 'Qdisc Config';
  readonly configItems = [
    'Egress-only; after TC egress classifier',
    'HTB, FQ, pfifo_fast, TBF, FQ-CoDel',
    'Bandwidth shaping and scheduling',
  ] as const;
  readonly configFields = [
    {
      key: 'type',
      label: 'Discipline',
      options: ['fq', 'htb', 'fq_codel', 'tbf', 'pfifo_fast'] as const,
      default: 'fq',
    },
  ] as const;
  // Qdisc is strictly on the egress path between TC egress and the NIC driver.
  readonly anchors = [
    { side: 'N', count: 1, flow: 'egress' as const, connector: 'in' as const },
    { side: 'S', count: 1, flow: 'egress' as const, connector: 'out' as const },
  ] as const;
}

export const qdisc = new QdiscComponent();
