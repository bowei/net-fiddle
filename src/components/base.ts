import type { LucideIcon } from 'lucide-react';

/**
 * Defines a draggable network component type.
 * Extend this class and register the instance in registry.ts to add a new type.
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
}
