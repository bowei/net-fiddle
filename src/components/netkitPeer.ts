import { Box } from 'lucide-react';
import { ComponentDef } from './base';

export class NetkitPeerComponent extends ComponentDef {
  readonly type = 'netkit-peer';
  readonly label = 'Netkit Container';
  readonly typeLabel = 'netkit-peer';
  readonly color = '#0d9488';
  readonly bgColor = '#f0fdfa';
  readonly borderColor = '#5eead4';
  readonly icon = Box;
  readonly configTitle = 'Netkit Container Side';
  readonly configItems = [
    'Peer (container) end of a netkit pair',
    'Peer link to host side is permanent',
    'W: ingress BPF hook — packets arriving from host',
    'E: egress BPF hook — packets departing to host',
    'BPF_PROG_TYPE_NETKIT or SCHED_CLS',
  ] as const;
  // N: container network stack. S: locked peer link. W/E: BPF hook attachment points.
  readonly anchors = [
    { side: 'S', count: 1, flow: 'any' as const, connector: 'both' as const },
    { side: 'N', count: 1, flow: 'ingress' as const, connector: 'out' as const },
    { side: 'N', count: 1, flow: 'egress' as const, connector: 'in' as const },
  ] as const;
}

export const netkitPeer = new NetkitPeerComponent();
