import type { LucideIcon } from 'lucide-react';

/** One of the four cardinal sides of a component box. */
export type CardinalSide = 'N' | 'E' | 'S' | 'W';

/**
 * Declares one group of anchor points on a single side of the component box.
 * Handles are distributed evenly along that side: for `count` handles,
 * handle i sits at position (i+1)/(count+1) of the side length.
 *
 * NOTE: changing `anchors` on a type is a breaking change for saved topologies,
 * because React Flow edge records store the handle IDs derived from side+index.
 */
export interface AnchorSpec {
  side: CardinalSide;
  count: number;
}

/**
 * Defines a draggable network component type.
 * Extend this class and register the instance in registry.ts to add a new type.
 *
 * For components that act as resizable containers for other nodes,
 * extend ContainerComponentDef instead.
 */
export abstract class ComponentDef {
  /** Unique identifier used as the node's data.nodeType field. */
  abstract readonly type: string;
  /** Human-readable name shown in the sidebar and Properties panel. */
  abstract readonly label: string;
  /** Short tag rendered beneath the node label on the canvas. */
  abstract readonly typeLabel: string;
  /** Primary brand color (icon, handles, selection ring). */
  abstract readonly color: string;
  /** Node card background color. */
  abstract readonly bgColor: string;
  /** Node card border color. */
  abstract readonly borderColor: string;
  /** Lucide icon displayed in the sidebar and on the canvas node. */
  abstract readonly icon: LucideIcon;
  /** Section heading in the Properties panel config block. */
  abstract readonly configTitle: string;
  /** Bullet-point descriptions in the Properties panel config block. */
  abstract readonly configItems: readonly string[];
  /**
   * Connection anchor points for this component type.
   * Each entry places `count` handles evenly distributed along the named side.
   * Use an empty array for types that should not be connectable.
   */
  abstract readonly anchors: readonly AnchorSpec[];
}

/**
 * Defines a component type that acts as a resizable container on the canvas.
 * Child nodes placed inside it move with it and are positioned relative to it.
 * Extend this class when adding a new container type.
 */
export abstract class ContainerComponentDef extends ComponentDef {
  readonly isContainer = true as const;
  /** Initial width in canvas pixels when the node is first dropped. */
  abstract readonly defaultWidth: number;
  /** Initial height in canvas pixels when the node is first dropped. */
  abstract readonly defaultHeight: number;
  /** Minimum width enforced by the resize handle. */
  abstract readonly minWidth: number;
  /** Minimum height enforced by the resize handle. */
  abstract readonly minHeight: number;
}
