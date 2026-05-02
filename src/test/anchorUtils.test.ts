import { describe, it, expect } from 'vitest';
import { Position } from 'reactflow';
import {
  toRFPosition,
  anchorStyle,
  anchorHandleId,
  anchorFlowColor,
  resolveHandles,
} from '../anchorUtils';
import type { CardinalSide, AnchorSpec } from '../components/base';

describe('toRFPosition', () => {
  it.each([
    ['N', Position.Top],
    ['E', Position.Right],
    ['S', Position.Bottom],
    ['W', Position.Left],
  ] as [CardinalSide, Position][])('%s → %s', (side, expected) => {
    expect(toRFPosition(side)).toBe(expected);
  });
});

describe('anchorStyle', () => {
  it('single handle on N/S sits at 50% left', () => {
    expect(anchorStyle('N', 0, 1)).toEqual({ left: '50%' });
    expect(anchorStyle('S', 0, 1)).toEqual({ left: '50%' });
  });

  it('single handle on E/W sits at 50% top', () => {
    expect(anchorStyle('E', 0, 1)).toEqual({ top: '50%' });
    expect(anchorStyle('W', 0, 1)).toEqual({ top: '50%' });
  });

  it('two handles on N are at ~33% and ~67%', () => {
    expect(anchorStyle('N', 0, 2)).toEqual({ left: `${(1 / 3) * 100}%` });
    expect(anchorStyle('N', 1, 2)).toEqual({ left: `${(2 / 3) * 100}%` });
  });

  it('three handles are evenly spaced at 25%, 50%, 75%', () => {
    expect(anchorStyle('S', 0, 3)).toEqual({ left: '25%' });
    expect(anchorStyle('S', 1, 3)).toEqual({ left: '50%' });
    expect(anchorStyle('S', 2, 3)).toEqual({ left: '75%' });
  });

  it('handles never land exactly at 0% or 100%', () => {
    for (const side of ['N', 'E', 'S', 'W'] as CardinalSide[]) {
      for (let count = 1; count <= 5; count++) {
        for (let i = 0; i < count; i++) {
          const style = anchorStyle(side, i, count);
          const pct = parseFloat(Object.values(style)[0] as string);
          expect(pct).toBeGreaterThan(0);
          expect(pct).toBeLessThan(100);
        }
      }
    }
  });
});

describe('anchorHandleId', () => {
  it('produces stable IDs from side and index', () => {
    expect(anchorHandleId('N', 0)).toBe('N-0');
    expect(anchorHandleId('S', 1)).toBe('S-1');
    expect(anchorHandleId('E', 0)).toBe('E-0');
    expect(anchorHandleId('W', 2)).toBe('W-2');
  });

  it('IDs are unique across sides and indices', () => {
    const ids = (['N', 'E', 'S', 'W'] as CardinalSide[]).flatMap(
      (side) => [0, 1, 2].map((i) => anchorHandleId(side, i))
    );
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('anchorFlowColor', () => {
  it('ingress → blue', () => expect(anchorFlowColor('ingress', '#000')).toBe('#3b82f6'));
  it('egress → amber', () => expect(anchorFlowColor('egress', '#000')).toBe('#f59e0b'));
  it('any → fallback color', () => expect(anchorFlowColor('any', '#aabbcc')).toBe('#aabbcc'));
});

describe('resolveHandles', () => {
  it('single-spec single handle returns globalIndex 0 of 1', () => {
    const specs: AnchorSpec[] = [{ side: 'N', count: 1, flow: 'ingress' }];
    const [h] = resolveHandles(specs);
    expect(h).toMatchObject({ id: 'N-0', side: 'N', flow: 'ingress', globalIndex: 0, totalOnSide: 1 });
  });

  it('two specs on same side are merged with global indices', () => {
    const specs: AnchorSpec[] = [
      { side: 'N', count: 1, flow: 'ingress' },
      { side: 'N', count: 1, flow: 'egress' },
    ];
    const handles = resolveHandles(specs);
    expect(handles).toHaveLength(2);
    expect(handles[0]).toMatchObject({ globalIndex: 0, totalOnSide: 2, flow: 'ingress' });
    expect(handles[1]).toMatchObject({ globalIndex: 1, totalOnSide: 2, flow: 'egress' });
  });

  it('handles on different sides do not share global indices', () => {
    const specs: AnchorSpec[] = [
      { side: 'N', count: 2, flow: 'any' },
      { side: 'S', count: 1, flow: 'any' },
    ];
    const handles = resolveHandles(specs);
    const north = handles.filter((h) => h.side === 'N');
    const south = handles.filter((h) => h.side === 'S');
    expect(north).toHaveLength(2);
    expect(south).toHaveLength(1);
    expect(south[0]).toMatchObject({ globalIndex: 0, totalOnSide: 1 });
  });

  it('empty anchors returns empty array', () => {
    expect(resolveHandles([])).toEqual([]);
  });

  it('connector is passed through to resolved handles', () => {
    const specs: AnchorSpec[] = [
      { side: 'N', count: 1, flow: 'ingress', connector: 'in' },
      { side: 'S', count: 1, flow: 'egress',  connector: 'out' },
    ];
    const [n, s] = resolveHandles(specs);
    expect(n.connector).toBe('in');
    expect(s.connector).toBe('out');
  });

  it('connector defaults to both when omitted', () => {
    const specs: AnchorSpec[] = [{ side: 'N', count: 1, flow: 'any' }];
    const [h] = resolveHandles(specs);
    expect(h.connector).toBe('both');
  });

  it('handle IDs are unique across all sides', () => {
    const specs: AnchorSpec[] = [
      { side: 'N', count: 2, flow: 'ingress' },
      { side: 'S', count: 2, flow: 'egress' },
      { side: 'E', count: 1, flow: 'any' },
    ];
    const ids = resolveHandles(specs).map((h) => h.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});
