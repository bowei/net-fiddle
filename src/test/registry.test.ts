import { describe, it, expect } from 'vitest';
import { REGISTRY, SIDEBAR_ITEMS } from '../components/registry';
import { ComponentDef, ContainerComponentDef } from '../components/base';
import { namespace } from '../components/namespace';
import { netInterface } from '../components/netInterface';
import { routingTable } from '../components/routingTable';
import { nftables } from '../components/nftables';
import { trafficControl } from '../components/trafficControl';
import { bpfProgram } from '../components/bpfProgram';

const ALL_DEFS = [namespace, netInterface, routingTable, nftables, trafficControl, bpfProgram];
const CONTAINER_DEFS = ALL_DEFS.filter((d) => d instanceof ContainerComponentDef);
const NODE_DEFS = ALL_DEFS.filter((d) => !(d instanceof ContainerComponentDef));

describe('ComponentDef subclasses', () => {
  it.each(ALL_DEFS)('$type has all required ComponentDef fields', (def) => {
    expect(def.type).toBeTruthy();
    expect(def.label).toBeTruthy();
    expect(def.typeLabel).toBeTruthy();
    expect(def.color).toMatch(/^#[0-9a-f]{6}$/i);
    expect(def.bgColor).toMatch(/^#[0-9a-f]{6}$/i);
    expect(def.borderColor).toMatch(/^#[0-9a-f]{6}$/i);
    expect(def.icon).toBeDefined();
    expect(def.configTitle).toBeTruthy();
    expect(def.configItems.length).toBeGreaterThan(0);
  });

  it('each type string is unique', () => {
    const types = ALL_DEFS.map((d) => d.type);
    expect(new Set(types).size).toBe(types.length);
  });

  it('all defs are instances of ComponentDef', () => {
    for (const def of ALL_DEFS) {
      expect(def).toBeInstanceOf(ComponentDef);
    }
  });
});

describe('ContainerComponentDef', () => {
  it('namespace is a container', () => {
    expect(namespace).toBeInstanceOf(ContainerComponentDef);
  });

  it('non-namespace types are plain nodes', () => {
    for (const def of NODE_DEFS) {
      expect(def).not.toBeInstanceOf(ContainerComponentDef);
    }
  });

  it.each(CONTAINER_DEFS)('$type has valid container dimensions', (def) => {
    const c = def as ContainerComponentDef;
    expect(c.defaultWidth).toBeGreaterThan(0);
    expect(c.defaultHeight).toBeGreaterThan(0);
    expect(c.minWidth).toBeGreaterThan(0);
    expect(c.minHeight).toBeGreaterThan(0);
    expect(c.minWidth).toBeLessThanOrEqual(c.defaultWidth);
    expect(c.minHeight).toBeLessThanOrEqual(c.defaultHeight);
  });
});

describe('REGISTRY', () => {
  it('contains every registered component', () => {
    for (const def of ALL_DEFS) {
      expect(REGISTRY.get(def.type)).toBe(def);
    }
  });

  it('returns undefined for unknown types', () => {
    expect(REGISTRY.get('does-not-exist')).toBeUndefined();
  });
});

describe('SIDEBAR_ITEMS', () => {
  it('lists every registered component exactly once', () => {
    expect(SIDEBAR_ITEMS).toHaveLength(ALL_DEFS.length);
    expect(new Set(SIDEBAR_ITEMS).size).toBe(ALL_DEFS.length);
  });

  it('matches REGISTRY entries', () => {
    for (const def of SIDEBAR_ITEMS) {
      expect(REGISTRY.get(def.type)).toBe(def);
    }
  });
});
