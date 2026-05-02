import { Code2 } from 'lucide-react';
import { ComponentDef } from './base';

export class BpfProgramComponent extends ComponentDef {
  readonly type = 'bpf-program';
  readonly label = 'BPF Program';
  readonly typeLabel = 'bpf';
  readonly color = '#9333ea';
  readonly bgColor = '#faf5ff';
  readonly borderColor = '#d8b4fe';
  readonly icon = Code2;
  readonly configTitle = 'BPF Config';
  readonly configItems = [
    'XDP programs',
    'TC BPF hooks',
    'Socket filters',
  ] as const;
  // Attaches to an interface on one side, hooks into the stack on the other.
  readonly anchors = [
    { side: 'N', count: 1 },
    { side: 'S', count: 1 },
  ] as const;
}

export const bpfProgram = new BpfProgramComponent();
