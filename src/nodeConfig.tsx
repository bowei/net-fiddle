import {
  Globe2,
  Cable,
  GitFork,
  Shield,
  Zap,
  Code2,
  type LucideIcon,
} from 'lucide-react';

export type NodeType =
  | 'namespace'
  | 'interface'
  | 'routing-table'
  | 'nftables'
  | 'traffic-control'
  | 'bpf-program';

interface NodeTypeConfig {
  label: string;
  typeLabel: string;
  color: string;
  bgColor: string;
  borderColor: string;
  Icon: LucideIcon;
  configTitle: string;
  configItems: string[];
}

export const NODE_CONFIG: Record<NodeType, NodeTypeConfig> = {
  namespace: {
    label: 'Namespace',
    typeLabel: 'namespace',
    color: '#3b82f6',
    bgColor: '#eff6ff',
    borderColor: '#93c5fd',
    Icon: Globe2,
    configTitle: 'Namespace Config',
    configItems: [
      'Network namespace isolation',
      'Separate routing table',
      'Independent interfaces',
    ],
  },
  interface: {
    label: 'Interface',
    typeLabel: 'interface',
    color: '#0d9488',
    bgColor: '#f0fdfa',
    borderColor: '#5eead4',
    Icon: Cable,
    configTitle: 'Interface Config',
    configItems: [
      'Physical/Virtual interface',
      'veth pairs, bridges, etc.',
      'MTU, MAC address settings',
    ],
  },
  'routing-table': {
    label: 'Routing Table',
    typeLabel: 'routing',
    color: '#6366f1',
    bgColor: '#eef2ff',
    borderColor: '#a5b4fc',
    Icon: GitFork,
    configTitle: 'Routing Config',
    configItems: [
      'Static routes',
      'Policy routing rules',
      'Default gateway',
    ],
  },
  nftables: {
    label: 'nftables',
    typeLabel: 'nftables',
    color: '#f97316',
    bgColor: '#fff7ed',
    borderColor: '#fdba74',
    Icon: Shield,
    configTitle: 'nftables Config',
    configItems: [
      'Firewall rules',
      'NAT configuration',
      'Packet filtering',
    ],
  },
  'traffic-control': {
    label: 'Traffic Control',
    typeLabel: 'tc',
    color: '#d97706',
    bgColor: '#fffbeb',
    borderColor: '#fcd34d',
    Icon: Zap,
    configTitle: 'TC Config',
    configItems: [
      'Rate limiting (tbf, htb)',
      'Queue disciplines',
      'Packet scheduling',
    ],
  },
  'bpf-program': {
    label: 'BPF Program',
    typeLabel: 'bpf',
    color: '#9333ea',
    bgColor: '#faf5ff',
    borderColor: '#d8b4fe',
    Icon: Code2,
    configTitle: 'BPF Config',
    configItems: [
      'XDP programs',
      'TC BPF hooks',
      'Socket filters',
    ],
  },
};

export const SIDEBAR_ITEMS: NodeType[] = [
  'namespace',
  'interface',
  'routing-table',
  'nftables',
  'traffic-control',
  'bpf-program',
];
