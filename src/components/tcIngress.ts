import { Layers } from 'lucide-react';
import { ComponentDef, type AnchorSpec } from './base';

export class TrafficControlComponent extends ComponentDef {
  readonly type = 'traffic-control';
  readonly label = 'tc ingress';
  readonly typeLabel = 'tc';
  readonly color = '#d97706';
  readonly bgColor = '#fffbeb';
  readonly borderColor = '#fcd34d';
  readonly icon = Layers;
  readonly configTitle = 'tc ingress';
  readonly configItems = [
    'clsact qdisc (ingress or egress)',
    'BPF_PROG_TYPE_SCHED_CLS classifiers',
    'TC_ACT_OK / SHOT / REDIRECT',
  ] as const;
  readonly configFields = [
    {
      key: 'direction',
      label: 'Direction',
      options: ['ingress', 'egress'] as const,
      default: 'ingress',
    },
  ] as const;
  // Anchors are declared as a placeholder; getAnchors() always takes precedence.
  readonly anchors = [] as const;

  getAnchors(config: Record<string, string>): readonly AnchorSpec[] {
    const flow = config.direction === 'egress' ? 'egress' : 'ingress';
    return [
      { side: 'N', count: 1, flow, connector: 'in' as const },
      { side: 'S', count: 1, flow, connector: 'out' as const },
    ];
  }
}

export const trafficControl = new TrafficControlComponent();
