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
}

export const bpfProgram = new BpfProgramComponent();
