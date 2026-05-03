import { Server } from 'lucide-react';
import { ComponentDef } from './base';

export class NetkitPrimaryComponent extends ComponentDef {
  readonly type = 'netkit-primary';
  readonly label = 'Netkit Host';
  readonly typeLabel = 'netkit-host';
  readonly color = '#0d9488';
  readonly bgColor = '#f0fdfa';
  readonly borderColor = '#5eead4';
  readonly icon = Server;
  readonly configTitle = 'Netkit Host Side';
  readonly configItems = [
    'Primary (host) end of a netkit pair',
    'Peer link to container side is permanent',
    'W: ingress BPF hook — packets arriving from peer',
    'E: egress BPF hook — packets departing to peer',
    'BPF_PROG_TYPE_NETKIT or SCHED_CLS',
  ] as const;
  // N: host network stack. S: locked peer link. W/E: BPF hook attachment points.
  readonly anchors = [
    { side: 'S', count: 1, flow: 'any' as const, connector: 'both' as const },
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'N', count: 1, flow: 'egress' as const, connector: 'in' as const },
  ] as const;
}

export const netkitPrimary = new NetkitPrimaryComponent();
