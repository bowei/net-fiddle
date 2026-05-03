import { Position } from 'reactflow';
import type { CSSProperties } from 'react';
import type { CardinalSide, AnchorFlow, AnchorConnector, AnchorSpec } from './components/base';

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
 * Returns the inline style that positions a handle at slot `globalIndex` out of
 * `totalOnSide` handles evenly distributed along `side`.
 *
 * Formula: offset = (globalIndex + 1) / (totalOnSide + 1) → never 0% or 100%.
 *
 * Only the axis running *along* the side is set here (left for N/S, top for E/W).
 * React Flow's `position` prop already pins the perpendicular axis.
 */
export function anchorStyle(
  side: CardinalSide,
  globalIndex: number,
  totalOnSide: number
): CSSProperties {
  const pct = `${((globalIndex + 1) / (totalOnSide + 1)) * 100}%`;
  return side === 'N' || side === 'S' ? { left: pct } : { top: pct };
}

/**
 * Produces a stable handle ID for use in React Flow edge records.
 * Format: "{side}-{globalIndex}", e.g. "N-0", "S-1".
 */
export function anchorHandleId(side: CardinalSide, globalIndex: number): string {
  return `${side}-${globalIndex}`;
}

/** Returns the color for a handle based on its flow direction. */
export function anchorFlowColor(flow: AnchorFlow, fallback: string): string {
  if (flow === 'ingress') return '#3b82f6'; // blue
  if (flow === 'egress') return '#f59e0b'; // amber
  return fallback;
}

/**
 * A fully resolved handle ready for React Flow rendering.
 * Multiple AnchorSpecs on the same side are merged so handles are spaced
 * across the full side width without overlap.
 */
export interface ResolvedHandle {
  id: string;
  side: CardinalSide;
  flow: AnchorFlow;
  connector: AnchorConnector;
  globalIndex: number;
  totalOnSide: number;
}

/**
 * Converts a list of AnchorSpecs into ResolvedHandles, assigning each handle
 * a global index within its side so positions never overlap.
 */
export function resolveHandles(anchors: readonly AnchorSpec[]): ResolvedHandle[] {
  // Collect all handles per side in declaration order.
  const bySide = new Map<CardinalSide, Array<{ flow: AnchorFlow; connector: AnchorConnector }>>();
  for (const { side, count, flow, connector = 'both' } of anchors) {
    if (!bySide.has(side)) bySide.set(side, []);
    const arr = bySide.get(side)!;
    for (let i = 0; i < count; i++) arr.push({ flow, connector });
  }

  const result: ResolvedHandle[] = [];
  for (const [side, handles] of bySide) {
    const total = handles.length;
    handles.forEach(({ flow, connector }, globalIndex) => {
      result.push({
        id: anchorHandleId(side, globalIndex),
        side,
        flow,
        connector,
        globalIndex,
        totalOnSide: total,
      });
    });
  }
  return result;
}
