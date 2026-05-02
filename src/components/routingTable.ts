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
    'ip_route_input() / ip_route_output()',
    'Local delivery vs. forward decision',
    'Policy routing (ip rule)',
  ] as const;
  // Routing sits at the intersection of ingress and egress paths; 'any' on all
  // anchors because it handles both directions depending on context.
  // N: arrives from nftables prerouting (ingress) or local socket (egress).
  // S: two outputs — local delivery and forwarding/egress output.
  readonly anchors = [
    { side: 'N', count: 1, flow: 'any' as const, connector: 'in' as const },
    { side: 'S', count: 2, flow: 'any' as const, connector: 'out' as const },
  ] as const;
}

export const routingTable = new RoutingTableComponent();
