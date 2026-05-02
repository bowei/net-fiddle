import { describe, it, expect } from 'vitest';
import { Position } from 'reactflow';
import { toRFPosition, anchorStyle, anchorHandleId } from '../anchorUtils';
import type { CardinalSide } from '../components/base';

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
          const val = Object.values(style)[0] as string;
          const pct = parseFloat(val);
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
    const sides: CardinalSide[] = ['N', 'E', 'S', 'W'];
    const ids = sides.flatMap((side) => [0, 1, 2].map((i) => anchorHandleId(side, i)));
    expect(new Set(ids).size).toBe(ids.length);
  });
});
