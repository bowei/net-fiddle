import { Cable } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

/** Sidebar-only placeholder that drops a paired veth-end node duo. Not a ComponentDef. */
export interface SidebarTemplate {
  readonly type: string;
  readonly label: string;
  readonly color: string;
  readonly icon: LucideIcon;
}

export const vethPair: SidebarTemplate = {
  type: 'veth',
  label: 'veth',
  color: '#0d9488',
  icon: Cable,
};
