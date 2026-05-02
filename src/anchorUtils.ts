import { Position } from 'reactflow';
import type { CSSProperties } from 'react';
import type { CardinalSide } from './components/base';

const SIDE_TO_RF_POSITION: Record<CardinalSide, Position> = {
  N: Position.Top,
  E: Position.Right,
  S: Position.Bottom,
  W: Position.Left,
};

export function toRFPosition(side: CardinalSide): Position {
  return SIDE_TO_RF_POSITION[side];
}

/**
 * Returns the inline style that positions a handle at slot `index` out of
 * `count` handles evenly distributed along `side`.
 *
 * Formula: offset = (index + 1) / (count + 1)  →  never 0% or 100%.
 *
 * Only the axis running *along* the side is set here (left for N/S, top for
 * E/W). React Flow's `position` prop already pins the perpendicular axis.
 */
export function anchorStyle(
  side: CardinalSide,
  index: number,
  count: number
): CSSProperties {
  const pct = `${((index + 1) / (count + 1)) * 100}%`;
  return side === 'N' || side === 'S' ? { left: pct } : { top: pct };
}

/**
 * Produces a stable handle ID for use in React Flow edge records.
 * Format: "{side}-{index}", e.g. "N-0", "S-1".
 */
export function anchorHandleId(side: CardinalSide, index: number): string {
  return `${side}-${index}`;
}
