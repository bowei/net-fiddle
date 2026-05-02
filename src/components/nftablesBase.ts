import { Shield } from 'lucide-react';
import { ComponentDef } from './base';

/** Shared visual identity for all nftables hook variants. */
export abstract class NftablesBase extends ComponentDef {
  readonly color = '#f97316';
  readonly bgColor = '#fff7ed';
  readonly borderColor = '#fdba74';
  readonly icon = Shield;
  readonly configTitle = 'nftables Config';
}
