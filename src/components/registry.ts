import type { ComponentDef } from './base';
import { namespace } from './namespace';
import { netInterface } from './netInterface';
import { routingTable } from './routingTable';
import { trafficControl } from './trafficControl';
import { xdpProgram } from './xdpProgram';
import { tcBpfProgram } from './tcBpfProgram';
import { schedBpf } from './schedBpf';
import { nftablesPrerouting } from './nftablesPrerouting';
import { nftablesInput } from './nftablesInput';
import { nftablesForward } from './nftablesForward';
import { nftablesOutput } from './nftablesOutput';
import { nftablesPostrouting } from './nftablesPostrouting';
import { qdisc } from './qdisc';
import { socket } from './socket';
import { vethEnd } from './vethEnd';
import { vethPair, type SidebarTemplate } from './vethPair';
import { netkitPrimary } from './netkitPrimary';
import { netkitPeer } from './netkitPeer';
import { netkitBpf } from './netkitBpf';
import { netkitPair } from './netkitPair';

export type { SidebarTemplate };

/** Sidebar item: either a real ComponentDef or a virtual template (e.g. Veth Pair). */
export type SidebarItem = ComponentDef | SidebarTemplate;

/**
 * A named group of component types shown together in the sidebar.
 * To add a new type: create its class file, import the singleton, and add it
 * to the appropriate group (or create a new group) here.
 */
export interface SidebarGroup {
  label: string;
  items: readonly SidebarItem[];
}

export const SIDEBAR_GROUPS: readonly SidebarGroup[] = [
  {
    label: 'OS',
    items: [namespace, socket],
  },
  {
    label: 'Interface',
    items: [netInterface, vethPair, netkitPair],
  },
  {
    label: 'Routing',
    items: [routingTable],
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
    label: 'TC',
    items: [trafficControl, qdisc, tcBpfProgram, schedBpf],
  },
  {
    label: 'BPF',
    items: [xdpProgram],
  },
  {
    label: 'Netkit',
    items: [netkitBpf],
  },
];

/** Flat ordered list of all registered ComponentDefs (SidebarTemplates excluded). */
export const SIDEBAR_ITEMS: readonly ComponentDef[] = SIDEBAR_GROUPS.flatMap((g) =>
  g.items.filter((item): item is ComponentDef => item instanceof Object && 'anchors' in item)
);

/** O(1) lookup by component type string. Includes veth-end (not in sidebar) for canvas rendering. */
export const REGISTRY: ReadonlyMap<string, ComponentDef> = new Map(
  [...SIDEBAR_ITEMS, vethEnd, netkitPrimary, netkitPeer].map((c) => [c.type, c])
);

export type { ComponentDef };
