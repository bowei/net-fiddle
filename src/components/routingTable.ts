import { GitFork } from 'lucide-react';
import { ComponentDef } from './base';

export class RoutingTableComponent extends ComponentDef {
  readonly type = 'routing-table';
  readonly label = 'Routing Table';
  readonly typeLabel = 'routing';
  readonly color = '#6366f1';
  readonly bgColor = '#eef2ff';
  readonly borderColor = '#a5b4fc';
  readonly icon = GitFork;
  readonly configTitle = 'Routing Config';
  readonly configItems = [
    'Static routes',
    'Policy routing rules',
    'Default gateway',
  ] as const;
}

export const routingTable = new RoutingTableComponent();
