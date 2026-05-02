import { describe, it, expect } from 'vitest';
import { NODE_CONFIG, SIDEBAR_ITEMS, type NodeType } from '../nodeConfig';

const ALL_TYPES: NodeType[] = [
  'namespace',
  'interface',
  'routing-table',
  'nftables',
  'traffic-control',
  'bpf-program',
];

describe('NODE_CONFIG', () => {
  it('has an entry for every node type', () => {
    for (const t of ALL_TYPES) {
      expect(NODE_CONFIG[t]).toBeDefined();
    }
  });

  it('every entry has required fields', () => {
    for (const t of ALL_TYPES) {
      const cfg = NODE_CONFIG[t];
      expect(cfg.label).toBeTruthy();
      expect(cfg.typeLabel).toBeTruthy();
      expect(cfg.color).toMatch(/^#[0-9a-f]{6}$/i);
      expect(cfg.bgColor).toMatch(/^#[0-9a-f]{6}$/i);
      expect(cfg.borderColor).toMatch(/^#[0-9a-f]{6}$/i);
      expect(cfg.Icon).toBeDefined();
      expect(cfg.configTitle).toBeTruthy();
      expect(cfg.configItems.length).toBeGreaterThan(0);
    }
  });
});

describe('SIDEBAR_ITEMS', () => {
  it('lists every node type exactly once', () => {
    expect(SIDEBAR_ITEMS).toHaveLength(ALL_TYPES.length);
    expect(new Set(SIDEBAR_ITEMS).size).toBe(ALL_TYPES.length);
  });
});
