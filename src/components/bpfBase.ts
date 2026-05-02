import { ComponentDef } from './base';

/** Shared visual identity for all BPF program variants. */
export abstract class BpfBase extends ComponentDef {
  readonly color = '#9333ea';
  readonly bgColor = '#faf5ff';
  readonly borderColor = '#d8b4fe';
}
