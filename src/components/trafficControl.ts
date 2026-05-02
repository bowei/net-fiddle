import { Sliders } from 'lucide-react';
import { ComponentDef, type AnchorSpec } from './base';

export class TrafficControlComponent extends ComponentDef {
  readonly type = 'traffic-control';
  readonly label = 'Traffic Control';
  readonly typeLabel = 'tc';
  readonly color = '#d97706';
  readonly bgColor = '#fffbeb';
  readonly borderColor = '#fcd34d';
  readonly icon = Sliders;
  readonly configTitle = 'TC Config';
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
      { side: 'N', count: 1, flow },
      { side: 'S', count: 1, flow },
    ];
  }
}

export const trafficControl = new TrafficControlComponent();
