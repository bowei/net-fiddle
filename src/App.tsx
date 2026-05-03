import {
  useState,
  useCallback,
  useRef,
  useEffect,
  useMemo,
  DragEvent,
} from 'react';
import ReactFlow, {
  addEdge,
  updateEdge,
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
  type XYPosition,
  type ReactFlowInstance,
} from 'reactflow';
import 'reactflow/dist/style.css';

import CustomNode, { type NetNodeData } from './CustomNode';
import ContainerNode from './ContainerNode';
import { REGISTRY, SIDEBAR_GROUPS } from './components/registry';
import { ContainerComponentDef, type AnchorFlow } from './components/base';
import { DragContext } from './DragContext';
import { ALL_RULES, resolveEdgeHandleFlows, type GraphViolation } from './rules';
import {
  Upload,
  Download,
  Trash2,
  X,
  FlaskConical,
} from 'lucide-react';

const nodeTypes = {
  netNode: CustomNode,
  containerNode: ContainerNode,
};

function absolutePosition(node: Node): XYPosition {
  return node.positionAbsolute ?? node.position;
}

function findContainerAt(pos: XYPosition, containers: Node[]): Node | undefined {
  return containers.find((c) => {
    const w = c.width ?? (c.style?.width as number) ?? 300;
    const h = c.height ?? (c.style?.height as number) ?? 200;
    return pos.x >= c.position.x && pos.x <= c.position.x + w &&
           pos.y >= c.position.y && pos.y <= c.position.y + h;
  });
}

/** Returns the dominant flow type of an edge for coloring/arrow purposes. */
function edgeFlowType(edge: Edge, nodes: Node<NetNodeData>[]): AnchorFlow {
  const flows = resolveEdgeHandleFlows(edge, nodes);
  if (!flows) return 'any';
  if (flows.src !== 'any') return flows.src;
  if (flows.tgt !== 'any') return flows.tgt;
  return 'any';
}

const SAMPLE_NODES: Node<NetNodeData>[] = [
  {
    id: 'ns-1',
    type: 'containerNode',
    position: { x: 30, y: 30 },
    style: { width: 360, height: 400 },
    zIndex: -1,
    data: { nodeType: 'namespace', label: 'ns-1' },
  },
  {
    id: 'socket-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 110, y: 30 },
    data: { nodeType: 'socket', label: 'socket-1' },
  },
  {
    id: 'nft-out-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 100, y: 120 },
    data: { nodeType: 'nftables-output', label: 'nft-out-1' },
  },
  {
    id: 'iface-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 110, y: 220 },
    data: { nodeType: 'interface', label: 'eth0' },
  },
  {
    id: 'xdp-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 20, y: 310 },
    data: { nodeType: 'xdp-program', label: 'xdp-1' },
  },
  {
    id: 'tc-rx-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 220, y: 310 },
    data: { nodeType: 'traffic-control', label: 'tc-rx', config: { direction: 'ingress' } },
  },
];

const SAMPLE_EDGES = [
  { id: 'e-sock-out', source: 'socket-1', sourceHandle: 'N-0-s', target: 'nft-out-1', targetHandle: 'N-0-t' },
  { id: 'e-out-iface', source: 'nft-out-1', sourceHandle: 'S-0-s', target: 'iface-1', targetHandle: 'E-0-t' },
  { id: 'e-iface-xdp', source: 'iface-1', sourceHandle: 'W-0-s', target: 'xdp-1', targetHandle: 'N-0-t' },
  { id: 'e-iface-tc', source: 'iface-1', sourceHandle: 'W-0-s', target: 'tc-rx-1', targetHandle: 'N-0-t' },
];

export default function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState<NetNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNode, setSelectedNode] = useState<Node<NetNodeData> | null>(null);
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const [hoverContainerId, setHoverContainerId] = useState<string | null>(null);
  const [edgeTooltip, setEdgeTooltip] = useState<{ messages: string[]; x: number; y: number } | null>(null);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const counters = useRef<Record<string, number>>({});
  const edgeReconnectSuccessful = useRef(true);

  const violations = useMemo<GraphViolation[]>(
    () => ALL_RULES.flatMap((r) => r.check(nodes, edges)),
    [nodes, edges]
  );

  const violationsByEdge = useMemo(() => {
    const map = new Map<string, GraphViolation[]>();
    for (const v of violations) {
      if (!v.edgeId) continue;
      if (!map.has(v.edgeId)) map.set(v.edgeId, []);
      map.get(v.edgeId)!.push(v);
    }
    return map;
  }, [violations]);

  // Derive edges with flow-colored strokes, directional arrowheads, and violation warnings.
  const styledEdges = useMemo(
    () =>
      edges.map((e) => {
        if (e.data?.vethLink) return e;
        const hasError = (violationsByEdge.get(e.id) ?? []).some((v) => v.severity === 'error');
        const flow = edgeFlowType(e, nodes);
        const color = hasError
          ? '#ef4444'
          : flow === 'ingress'
          ? '#3b82f6'
          : flow === 'egress'
          ? '#f59e0b'
          : undefined;
        const marker = color
          ? { type: MarkerType.ArrowClosed, color, width: 16, height: 16 }
          : undefined;
        return {
          ...e,
          style: color ? { ...e.style, stroke: color, strokeWidth: hasError ? 2 : 1.5 } : e.style,
          markerEnd: marker,
          label: hasError ? '⚠' : e.label,
        };
      }),
    [edges, nodes, violationsByEdge]
  );

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  const onEdgeMouseEnter = useCallback(
    (event: React.MouseEvent, edge: Edge) => {
      const msgs = (violationsByEdge.get(edge.id) ?? []).map((v) => v.message);
      if (msgs.length === 0) return;
      const rect = wrapperRef.current?.getBoundingClientRect();
      setEdgeTooltip({ messages: msgs, x: event.clientX - (rect?.left ?? 0), y: event.clientY - (rect?.top ?? 0) });
    },
    [violationsByEdge]
  );

  const onEdgeMouseMove = useCallback((event: React.MouseEvent) => {
    const rect = wrapperRef.current?.getBoundingClientRect();
    setEdgeTooltip((t) => t ? { ...t, x: event.clientX - (rect?.left ?? 0), y: event.clientY - (rect?.top ?? 0) } : null);
  }, []);

  const onEdgeMouseLeave = useCallback(() => setEdgeTooltip(null), []);

  const onEdgeReconnectStart = useCallback((_event: unknown, edge: Edge) => {
    if (edge.data?.vethLink) { edgeReconnectSuccessful.current = true; return; }
    edgeReconnectSuccessful.current = false;
  }, []);

  const onEdgeReconnect = useCallback(
    (oldEdge: Edge, newConnection: Connection) => {
      if (oldEdge.data?.vethLink) return;
      edgeReconnectSuccessful.current = true;
      setEdges((eds) => updateEdge(oldEdge, newConnection, eds));
    },
    [setEdges]
  );

  const onEdgeReconnectEnd = useCallback(
    (_: MouseEvent | TouchEvent, edge: Edge) => {
      if (edge.data?.vethLink) return;
      if (!edgeReconnectSuccessful.current) {
        setEdges((eds) => eds.filter((e) => e.id !== edge.id));
      }
      edgeReconnectSuccessful.current = true;
    },
    [setEdges]
  );

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node<NetNodeData>) => {
    setSelectedNode(node);
  }, []);

  const onPaneClick = useCallback(() => setSelectedNode(null), []);

  const nextLabel = (nodeType: string): string => {
    const def = REGISTRY.get(nodeType);
    const prefix = def?.typeLabel ?? nodeType;
    const count = (counters.current[nodeType] ?? 0) + 1;
    counters.current[nodeType] = count;
    return `${prefix}-${count}`;
  };

  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: string) => {
    event.dataTransfer.setData('application/netfiddle', nodeType);
    event.dataTransfer.effectAllowed = 'move';
  };

  const onDragOver = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      event.dataTransfer.dropEffect = 'move';
      setIsDragOver(true);
      if (!rfInstance || !wrapperRef.current) return;
      const bounds = wrapperRef.current.getBoundingClientRect();
      const pos = rfInstance.project({ x: event.clientX - bounds.left, y: event.clientY - bounds.top });
      const found = findContainerAt(pos, nodes.filter((n) => n.type === 'containerNode'));
      setHoverContainerId(found?.id ?? null);
    },
    [nodes, rfInstance]
  );

  const onDragLeave = useCallback(() => {
    setIsDragOver(false);
    setHoverContainerId(null);
  }, []);

  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      setIsDragOver(false);
      setHoverContainerId(null);
      const nodeType = event.dataTransfer.getData('application/netfiddle');
      if (!nodeType || !rfInstance || !wrapperRef.current) return;

      const bounds = wrapperRef.current.getBoundingClientRect();
      const canvasPos = rfInstance.project({ x: event.clientX - bounds.left, y: event.clientY - bounds.top });

      if (nodeType === 'veth') {
        const pairN = (counters.current['veth'] ?? 0) + 1;
        counters.current['veth'] = pairN;
        const pairId = `veth-pair-${pairN}`;
        const aId = `veth-${pairN}a`;
        const bId = `veth-${pairN}b`;
        const parent = findContainerAt(canvasPos, nodes.filter((n) => n.type === 'containerNode'));
        const makeVethNode = (id: string, label: string, dx: number) => {
          const pos = parent
            ? { x: canvasPos.x - parent.position.x + dx, y: canvasPos.y - parent.position.y }
            : { x: canvasPos.x + dx, y: canvasPos.y };
          return {
            id, type: 'netNode' as const,
            position: pos,
            ...(parent ? { parentNode: parent.id } : {}),
            data: { nodeType: 'veth-end', label, vethPairId: pairId },
          };
        };
        setNodes((nds) => [...nds, makeVethNode(aId, `veth-${pairN}a`, -90), makeVethNode(bId, `veth-${pairN}b`, 90)]);
        setEdges((eds) => [...eds, {
          id: `veth-link-${pairId}`,
          source: aId, sourceHandle: 'S-0-s',
          target: bId, targetHandle: 'S-0-t',
          data: { vethLink: true },
          className: 'veth-link',
          style: { stroke: '#0d9488', strokeWidth: 3, strokeDasharray: '6 3' },
          label: '⛓',
        }]);
        return;
      }

      if (nodeType === 'netkit') {
        const pairN = (counters.current['netkit'] ?? 0) + 1;
        counters.current['netkit'] = pairN;
        const pairId = `netkit-pair-${pairN}`;
        const primaryId = `netkit-${pairN}-host`;
        const peerId = `netkit-${pairN}-peer`;
        const parent = findContainerAt(canvasPos, nodes.filter((n) => n.type === 'containerNode'));
        const makeNetkitNode = (id: string, label: string, nodeType: string, dx: number) => {
          const pos = parent
            ? { x: canvasPos.x - parent.position.x + dx, y: canvasPos.y - parent.position.y }
            : { x: canvasPos.x + dx, y: canvasPos.y };
          return {
            id, type: 'netNode' as const,
            position: pos,
            ...(parent ? { parentNode: parent.id } : {}),
            data: { nodeType, label, vethPairId: pairId },
          };
        };
        setNodes((nds) => [
          ...nds,
          makeNetkitNode(primaryId, `netkit-${pairN}-host`, 'netkit-primary', -100),
          makeNetkitNode(peerId,    `netkit-${pairN}-peer`, 'netkit-peer',    100),
        ]);
        setEdges((eds) => [...eds, {
          id: `netkit-link-${pairId}`,
          source: primaryId, sourceHandle: 'S-0-s',
          target: peerId,    targetHandle: 'S-0-t',
          data: { vethLink: true },
          className: 'veth-link',
          style: { stroke: '#0d9488', strokeWidth: 3, strokeDasharray: '6 3' },
          label: '⛓',
        }]);
        return;
      }

      const def = REGISTRY.get(nodeType);
      if (!def) return;
      const label = nextLabel(nodeType);

      if (def instanceof ContainerComponentDef) {
        setNodes((nds) => [...nds, {
          id: label, type: 'containerNode', position: canvasPos, zIndex: -1,
          style: { width: def.defaultWidth, height: def.defaultHeight },
          data: { nodeType, label },
        }]);
      } else {
        const parent = findContainerAt(canvasPos, nodes.filter((n) => n.type === 'containerNode'));
        setNodes((nds) => [...nds, {
          id: label, type: 'netNode',
          position: parent
            ? { x: canvasPos.x - parent.position.x, y: canvasPos.y - parent.position.y }
            : canvasPos,
          ...(parent ? { parentNode: parent.id } : {}),
          data: { nodeType, label },
        }]);
      }
    },
    [nodes, rfInstance, setNodes, setEdges]
  );

  const onNodeDrag = useCallback(
    (_: React.MouseEvent, draggedNode: Node<NetNodeData>) => {
      if (draggedNode.type === 'containerNode') { setHoverContainerId(null); return; }
      const absPos = absolutePosition(draggedNode);
      const found = findContainerAt(absPos, nodes.filter((n) => n.type === 'containerNode' && n.id !== draggedNode.parentNode));
      setHoverContainerId(found?.id ?? null);
    },
    [nodes]
  );

  const onNodeDragStop = useCallback(
    (_: React.MouseEvent, draggedNode: Node<NetNodeData>) => {
      setHoverContainerId(null);
      if (draggedNode.type === 'containerNode') return;
      const absPos = absolutePosition(draggedNode);
      const newParent = findContainerAt(absPos, nodes.filter((n) => n.type === 'containerNode'));
      if (newParent?.id === draggedNode.parentNode) return;
      setNodes((nds) => nds.map((n) => {
        if (n.id !== draggedNode.id) return n;
        if (newParent) return {
          ...n, parentNode: newParent.id,
          position: { x: absPos.x - newParent.position.x, y: absPos.y - newParent.position.y },
        };
        return { ...n, parentNode: undefined, position: absPos };
      }));
    },
    [nodes, setNodes]
  );

  const deleteSelected = useCallback(() => {
    if (!selectedNode) return;
    const toDelete = new Set([selectedNode.id]);
    if (selectedNode.type === 'containerNode') {
      nodes.forEach((n) => { if (n.parentNode === selectedNode.id) toDelete.add(n.id); });
    }
    const pairId = selectedNode.data.vethPairId;
    if (pairId) {
      nodes.forEach((n) => { if (n.data.vethPairId === pairId) toDelete.add(n.id); });
    }
    setNodes((nds) => nds.filter((n) => !toDelete.has(n.id)));
    setEdges((eds) => eds.filter((e) => !toDelete.has(e.source) && !toDelete.has(e.target)));
    setSelectedNode(null);
  }, [selectedNode, nodes, setNodes, setEdges]);

  const updateLabel = (label: string) => {
    if (!selectedNode) return;
    setNodes((nds) => nds.map((n) => n.id === selectedNode.id ? { ...n, data: { ...n.data, label } } : n));
    setSelectedNode((p) => p ? { ...p, data: { ...p.data, label } } : null);
  };

  const updateConfig = (key: string, value: string) => {
    if (!selectedNode) return;
    const config = { ...selectedNode.data.config, [key]: value };
    setNodes((nds) => nds.map((n) => n.id === selectedNode.id ? { ...n, data: { ...n.data, config } } : n));
    setSelectedNode((p) => p ? { ...p, data: { ...p.data, config } } : null);
  };

  const updatePosition = (axis: 'x' | 'y', value: number) => {
    if (!selectedNode) return;
    setNodes((nds) => nds.map((n) =>
      n.id === selectedNode.id ? { ...n, position: { ...n.position, [axis]: value } } : n
    ));
    setSelectedNode((p) => p ? { ...p, position: { ...p.position, [axis]: value } } : null);
  };

  const loadSample = () => {
    counters.current = { namespace: 1, socket: 1, 'nft:out': 1, interface: 1, xdp: 1, tc: 1 };
    setNodes(SAMPLE_NODES);
    setEdges(SAMPLE_EDGES);
    setSelectedNode(null);
  };

  const clearAll = () => {
    counters.current = {};
    setNodes([]);
    setEdges([]);
    setSelectedNode(null);
  };

  const doExport = () => {
    const blob = new Blob([JSON.stringify({ nodes, edges }, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    Object.assign(document.createElement('a'), { href: url, download: 'net-fiddle.json' }).click();
    URL.revokeObjectURL(url);
  };

  const doImport = () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = (ev) => {
        try {
          const parsed = JSON.parse(ev.target?.result as string);
          setNodes(parsed.nodes ?? []);
          setEdges(parsed.edges ?? []);
          setSelectedNode(null);
        } catch { alert('Invalid JSON file'); }
      };
      reader.readAsText(file);
    };
    input.click();
  };

  useEffect(() => {
    if (!selectedNode) return;
    const updated = nodes.find((n) => n.id === selectedNode.id);
    if (updated) setSelectedNode(updated);
  }, [nodes]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.key === 'Delete' || e.key === 'Backspace') && document.activeElement?.tagName !== 'INPUT') {
        deleteSelected();
      }
      if (e.key === 'Escape') setSelectedNode(null);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [deleteSelected]);

  const selectedDef = selectedNode ? REGISTRY.get(selectedNode.data.nodeType) : null;
  const selectedIsContainer = selectedDef instanceof ContainerComponentDef;

  return (
    <div className="app">
      <header className="header">
        <div className="header-title">
          <FlaskConical size={20} color="#6366f1" />
          Linux Network Topology Designer
          <span className="component-count">{nodes.length} components</span>
        </div>
        <div className="header-actions">
          <button className="btn btn-primary" onClick={loadSample}><FlaskConical size={14} /> Load Sample</button>
          <button className="btn btn-secondary" onClick={doImport}><Upload size={14} /> Import</button>
          <button className="btn btn-success" onClick={doExport}><Download size={14} /> Export</button>
          <button className="btn btn-danger" onClick={clearAll}><Trash2 size={14} /> Clear All</button>
        </div>
      </header>

      <div className="body">
        <aside className="sidebar">
          {SIDEBAR_GROUPS.map((group) => (
            <div key={group.label}>
              <div className="sidebar-group-label">{group.label}</div>
              {group.items.map((def) => {
                const Icon = def.icon;
                return (
                  <div key={def.type} className="component-item" draggable onDragStart={(e) => onDragStart(e, def.type)}>
                    <div className="component-icon"><Icon size={16} color={def.color} /></div>
                    {def.label}
                  </div>
                );
              })}
            </div>
          ))}
          <div className="quick-guide">
            <div className="quick-guide-title">Quick Guide</div>
            <ul>
              <li>Drag components to canvas</li>
              <li>Drop onto namespace to nest</li>
              <li>Drag handle to connect</li>
              <li><span className="flow-legend ingress" /> blue = ingress</li>
              <li><span className="flow-legend egress" /> amber = egress</li>
              <li>Red edge = flow mismatch</li>
            </ul>
          </div>
        </aside>

        <div
          ref={wrapperRef}
          className={`canvas-wrapper${isDragOver ? ' drag-over' : ''}`}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
        >
          <DragContext.Provider value={hoverContainerId}>
            <ReactFlow
              nodes={nodes}
              edges={styledEdges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onEdgeUpdateStart={onEdgeReconnectStart}
              onEdgeUpdate={onEdgeReconnect}
              onEdgeUpdateEnd={onEdgeReconnectEnd}
              onNodeClick={onNodeClick}
              onPaneClick={onPaneClick}
              onNodeDrag={onNodeDrag}
              onNodeDragStop={onNodeDragStop}
              onInit={setRfInstance}
              onEdgeMouseEnter={onEdgeMouseEnter}
              onEdgeMouseMove={onEdgeMouseMove}
              onEdgeMouseLeave={onEdgeMouseLeave}
              nodeTypes={nodeTypes}
              fitView={nodes.length > 0}
              deleteKeyCode={null}
            >
              <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#e2e8f0" />
              <Controls />
            </ReactFlow>
          </DragContext.Provider>
          {edgeTooltip && (
            <div className="edge-tooltip" style={{ left: edgeTooltip.x, top: edgeTooltip.y }}>
              {edgeTooltip.messages.map((msg, i) => <div key={i}>{msg}</div>)}
            </div>
          )}
        </div>

        {selectedNode && selectedDef && (
          <aside className="properties-panel">
            <div className="properties-header">
              <span className="properties-title">Properties</span>
              <div className="properties-header-actions">
                <button className="icon-btn danger" title="Delete" onClick={deleteSelected}><Trash2 size={15} /></button>
                <button className="icon-btn" title="Close" onClick={() => setSelectedNode(null)}><X size={15} /></button>
              </div>
            </div>

            <div className="prop-group">
              <span className="prop-label">Type</span>
              <input className="prop-input" readOnly value={selectedDef.label} />
            </div>

            <div className="prop-group">
              <span className="prop-label">Label</span>
              <input className="prop-input" value={selectedNode.data.label} onChange={(e) => updateLabel(e.target.value)} />
            </div>

            {/* Config dropdowns (e.g. TC direction, qdisc type) */}
            {selectedDef.configFields.map((field) => (
              <div className="prop-group" key={field.key}>
                <span className="prop-label">{field.label}</span>
                <select
                  className="prop-input prop-select"
                  value={selectedNode.data.config?.[field.key] ?? field.default}
                  onChange={(e) => updateConfig(field.key, e.target.value)}
                >
                  {field.options.map((opt) => <option key={opt} value={opt}>{opt}</option>)}
                </select>
              </div>
            ))}

            {!selectedIsContainer && (
              <div className="prop-group">
                <span className="prop-label">Position</span>
                <div className="position-row">
                  <div className="position-field">
                    <label>X</label>
                    <input className="prop-input" type="number" value={Math.round(selectedNode.position.x)} onChange={(e) => updatePosition('x', Number(e.target.value))} />
                  </div>
                  <div className="position-field">
                    <label>Y</label>
                    <input className="prop-input" type="number" value={Math.round(selectedNode.position.y)} onChange={(e) => updatePosition('y', Number(e.target.value))} />
                  </div>
                </div>
              </div>
            )}

            {selectedIsContainer && (
              <div className="prop-group">
                <span className="prop-label">Size</span>
                <div className="position-row">
                  <div className="position-field">
                    <label>W</label>
                    <input className="prop-input" type="number" readOnly value={Math.round((selectedNode.width ?? (selectedNode.style?.width as number) ?? 0))} />
                  </div>
                  <div className="position-field">
                    <label>H</label>
                    <input className="prop-input" type="number" readOnly value={Math.round((selectedNode.height ?? (selectedNode.style?.height as number) ?? 0))} />
                  </div>
                </div>
              </div>
            )}

            <div className="config-section">
              <div className="config-title">{selectedDef.configTitle}</div>
              <ul className="config-items">
                {selectedDef.configItems.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </div>
          </aside>
        )}
      </div>

      <footer className="status-bar">
        <div className="status-shortcuts">
          <div className="shortcut"><span className="kbd">Del</span> Delete selected</div>
          <div className="shortcut"><span className="kbd">Drag</span> handle to connect</div>
          <div className="shortcut"><span className="kbd">Esc</span> Deselect</div>
        </div>
        {violations.length > 0 && (
          <div className="status-violations" title={violations.map((v) => v.message).join('\n')}>
            ⚠ {violations.length} violation{violations.length > 1 ? 's' : ''}
          </div>
        )}
        <button className="status-help" title="Help">?</button>
      </footer>
    </div>
  );
}
