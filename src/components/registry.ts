import type { ComponentDef } from './base';
import { namespace } from './namespace';
import { netInterface } from './netInterface';
import { routingTable } from './routingTable';
import { nftables } from './nftables';
import { trafficControl } from './trafficControl';
import { bpfProgram } from './bpfProgram';

/**
 * Ordered list of all registered component types.
 * To add a new type: create its class file, import the singleton here,
 * and append it to this array.
 */
const COMPONENTS: ComponentDef[] = [
  namespace,
  netInterface,
  routingTable,
  nftables,
  trafficControl,
  bpfProgram,
];

/** Ordered definitions for the sidebar (preserves COMPONENTS order). */
export const SIDEBAR_ITEMS: readonly ComponentDef[] = COMPONENTS;

/** O(1) lookup by component type string. */
export const REGISTRY: ReadonlyMap<string, ComponentDef> = new Map(
  COMPONENTS.map((c) => [c.type, c])
);

export type { ComponentDef };
