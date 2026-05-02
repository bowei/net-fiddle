import { Globe2 } from 'lucide-react';
import { ComponentDef } from './base';

export class NamespaceComponent extends ComponentDef {
  readonly type = 'namespace';
  readonly label = 'Namespace';
  readonly typeLabel = 'namespace';
  readonly color = '#3b82f6';
  readonly bgColor = '#eff6ff';
  readonly borderColor = '#93c5fd';
  readonly icon = Globe2;
  readonly configTitle = 'Namespace Config';
  readonly configItems = [
    'Network namespace isolation',
    'Separate routing table',
    'Independent interfaces',
  ] as const;
}

export const namespace = new NamespaceComponent();
