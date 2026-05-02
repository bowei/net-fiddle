import { Globe2 } from 'lucide-react';
import { ContainerComponentDef } from './base';

export class NamespaceComponent extends ContainerComponentDef {
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
  // Containers are connected by nesting, not by edges.
  readonly anchors = [] as const;
  readonly defaultWidth = 320;
  readonly defaultHeight = 240;
  readonly minWidth = 160;
  readonly minHeight = 120;
}

export const namespace = new NamespaceComponent();
