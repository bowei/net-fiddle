import type { ComponentDef } from './base';
import { namespace } from './namespace';
import { netInterface } from './netInterface';
import { routingTable } from './routingTable';
import { trafficControl } from './trafficControl';
import { xdpProgram } from './xdpProgram';
import { tcBpfProgram } from './tcBpfProgram';
import { nftablesPrerouting } from './nftablesPrerouting';
import { nftablesInput } from './nftablesInput';
import { nftablesForward } from './nftablesForward';
import { nftablesOutput } from './nftablesOutput';
import { nftablesPostrouting } from './nftablesPostrouting';
import { qdisc } from './qdisc';
import { socket } from './socket';

/**
 * A named group of component types shown together in the sidebar.
 * To add a new type: create its class file, import the singleton, and add it
 * to the appropriate group (or create a new group) here.
 */
export interface SidebarGroup {
  label: string;
  items: readonly ComponentDef[];
}

export const SIDEBAR_GROUPS: readonly SidebarGroup[] = [
  {
    label: 'Network',
    items: [namespace, netInterface, socket, routingTable, qdisc],
  },
  {
    label: 'nftables',
    items: [
      nftablesPrerouting,
      nftablesInput,
      nftablesForward,
      nftablesOutput,
      nftablesPostrouting,
    ],
  },
  {
    label: 'BPF / TC',
    items: [xdpProgram, trafficControl, tcBpfProgram],
  },
];

/** Flat ordered list of all registered components (sidebar order preserved). */
export const SIDEBAR_ITEMS: readonly ComponentDef[] = SIDEBAR_GROUPS.flatMap(
  (g) => g.items
);

/** O(1) lookup by component type string. */
export const REGISTRY: ReadonlyMap<string, ComponentDef> = new Map(
  SIDEBAR_ITEMS.map((c) => [c.type, c])
);

export type { ComponentDef };
