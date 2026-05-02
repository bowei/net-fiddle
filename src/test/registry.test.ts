import { describe, it, expect } from 'vitest';
import { REGISTRY, SIDEBAR_ITEMS, SIDEBAR_GROUPS } from '../components/registry';
import { ComponentDef, ContainerComponentDef } from '../components/base';
import { namespace } from '../components/namespace';
import { netInterface } from '../components/netInterface';
import { routingTable } from '../components/routingTable';
import { trafficControl } from '../components/trafficControl';
import { xdpProgram } from '../components/xdpProgram';
import { tcBpfProgram } from '../components/tcBpfProgram';
import { nftablesPrerouting } from '../components/nftablesPrerouting';
import { nftablesInput } from '../components/nftablesInput';
import { nftablesForward } from '../components/nftablesForward';
import { nftablesOutput } from '../components/nftablesOutput';
import { nftablesPostrouting } from '../components/nftablesPostrouting';
import { qdisc } from '../components/qdisc';
import { socket } from '../components/socket';

const ALL_DEFS = [
  namespace, netInterface, routingTable, trafficControl,
  xdpProgram, tcBpfProgram,
  nftablesPrerouting, nftablesInput, nftablesForward, nftablesOutput, nftablesPostrouting,
  qdisc, socket,
];

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
    for (const def of ALL_DEFS) expect(def).toBeInstanceOf(ComponentDef);
  });

  it.each(ALL_DEFS)('$type anchors from getAnchors() have valid flow and side', (def) => {
    const anchors = def.getAnchors({});
    for (const { side, count, flow } of anchors) {
      expect(['N', 'E', 'S', 'W']).toContain(side);
      expect(count).toBeGreaterThan(0);
      expect(['ingress', 'egress', 'any']).toContain(flow);
    }
  });
});

describe('ContainerComponentDef', () => {
  it('namespace is a container', () => {
    expect(namespace).toBeInstanceOf(ContainerComponentDef);
  });

  it('non-namespace types are plain nodes', () => {
    for (const def of NODE_DEFS) expect(def).not.toBeInstanceOf(ContainerComponentDef);
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

describe('configFields', () => {
  it('TC and TC BPF have a direction field', () => {
    for (const def of [trafficControl, tcBpfProgram]) {
      const field = def.configFields.find((f) => f.key === 'direction');
      expect(field).toBeDefined();
      expect(field!.options).toContain('ingress');
      expect(field!.options).toContain('egress');
    }
  });

  it('getAnchors() reflects direction config for TC', () => {
    const rx = trafficControl.getAnchors({ direction: 'ingress' });
    const tx = trafficControl.getAnchors({ direction: 'egress' });
    expect(rx.every((a) => a.flow === 'ingress')).toBe(true);
    expect(tx.every((a) => a.flow === 'egress')).toBe(true);
  });

  it('getAnchors() reflects direction config for TC BPF', () => {
    const rx = tcBpfProgram.getAnchors({ direction: 'ingress' });
    const tx = tcBpfProgram.getAnchors({ direction: 'egress' });
    expect(rx.every((a) => a.flow === 'ingress')).toBe(true);
    expect(tx.every((a) => a.flow === 'egress')).toBe(true);
  });

  it('XDP is always ingress regardless of config', () => {
    const anchors = xdpProgram.getAnchors({ direction: 'egress' });
    expect(anchors.every((a) => a.flow === 'ingress')).toBe(true);
  });
});

describe('nftables hooks', () => {
  it('prerouting and input are ingress-only', () => {
    for (const def of [nftablesPrerouting, nftablesInput]) {
      expect(def.anchors.every((a) => a.flow === 'ingress')).toBe(true);
    }
  });

  it('output and postrouting are egress-only', () => {
    for (const def of [nftablesOutput, nftablesPostrouting]) {
      expect(def.anchors.every((a) => a.flow === 'egress')).toBe(true);
    }
  });

  it('forward is flow-agnostic', () => {
    expect(nftablesForward.anchors.every((a) => a.flow === 'any')).toBe(true);
  });
});

describe('interface anchors', () => {
  it('has a wire anchor (any), an ingress anchor (W), and an egress anchor (E)', () => {
    const wire = netInterface.anchors.find((a) => a.flow === 'any');
    const rx = netInterface.anchors.find((a) => a.flow === 'ingress');
    const tx = netInterface.anchors.find((a) => a.flow === 'egress');
    expect(wire).toBeDefined();
    expect(rx).toBeDefined();
    expect(tx).toBeDefined();
  });
});

describe('REGISTRY', () => {
  it('contains every registered component', () => {
    for (const def of ALL_DEFS) expect(REGISTRY.get(def.type)).toBe(def);
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
    for (const def of SIDEBAR_ITEMS) expect(REGISTRY.get(def.type)).toBe(def);
  });
});

describe('SIDEBAR_GROUPS', () => {
  it('every group has a label and at least one item', () => {
    for (const group of SIDEBAR_GROUPS) {
      expect(group.label).toBeTruthy();
      expect(group.items.length).toBeGreaterThan(0);
    }
  });

  it('all items across groups are registered', () => {
    for (const group of SIDEBAR_GROUPS) {
      for (const def of group.items) expect(REGISTRY.get(def.type)).toBe(def);
    }
  });
});
