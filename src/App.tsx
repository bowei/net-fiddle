import {
  useState,
  useCallback,
  useRef,
  useEffect,
  DragEvent,
} from 'react';
import ReactFlow, {
  addEdge,
  Background,
  BackgroundVariant,
  Controls,
  useNodesState,
  useEdgesState,
  type Connection,
  type Node,
  type ReactFlowInstance,
} from 'reactflow';
import 'reactflow/dist/style.css';

import CustomNode, { type NetNodeData } from './CustomNode';
import { NODE_CONFIG, SIDEBAR_ITEMS, type NodeType } from './nodeConfig';
import {
  Upload,
  Download,
  Trash2,
  X,
  FlaskConical,
} from 'lucide-react';

const nodeTypes = { netNode: CustomNode };

const SAMPLE_NODES: Node<NetNodeData>[] = [
  {
    id: 'interface-1',
    type: 'netNode',
    position: { x: 96, y: 160 },
    data: { nodeType: 'interface', label: 'interface-1' },
  },
  {
    id: 'nftables-2',
    type: 'netNode',
    position: { x: 300, y: 80 },
    data: { nodeType: 'nftables', label: 'nftables-2' },
  },
  {
    id: 'tc-3',
    type: 'netNode',
    position: { x: 310, y: 200 },
    data: { nodeType: 'traffic-control', label: 'tc-3' },
  },
];

const SAMPLE_EDGES = [
  { id: 'e1-2', source: 'interface-1', target: 'nftables-2' },
  { id: 'e1-3', source: 'interface-1', target: 'tc-3' },
];

export default function App() {
  const [nodes, setNodes, onNodesChange] = useNodesState<NetNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNode, setSelectedNode] = useState<Node<NetNodeData> | null>(null);
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const counters = useRef<Record<string, number>>({});

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node<NetNodeData>) => {
    setSelectedNode(node);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, []);

  const nextLabel = (nodeType: NodeType) => {
    const cfg = NODE_CONFIG[nodeType];
    const count = (counters.current[nodeType] ?? 0) + 1;
    counters.current[nodeType] = count;
    return `${cfg.typeLabel}-${count}`;
  };

  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: NodeType) => {
    event.dataTransfer.setData('application/netfiddle', nodeType);
    event.dataTransfer.effectAllowed = 'move';
  };

  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      setIsDragOver(false);
      const nodeType = event.dataTransfer.getData('application/netfiddle') as NodeType;
      if (!nodeType || !rfInstance || !wrapperRef.current) return;

      const bounds = wrapperRef.current.getBoundingClientRect();
      const position = rfInstance.project({
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });

      const label = nextLabel(nodeType);
      const id = label;
      const newNode: Node<NetNodeData> = {
        id,
        type: 'netNode',
        position,
        data: { nodeType, label },
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [rfInstance, setNodes]
  );

  const onDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    setIsDragOver(true);
  }, []);

  const onDragLeave = useCallback(() => setIsDragOver(false), []);

  const deleteSelected = useCallback(() => {
    if (!selectedNode) return;
    setNodes((nds) => nds.filter((n) => n.id !== selectedNode.id));
    setEdges((eds) =>
      eds.filter(
        (e) => e.source !== selectedNode.id && e.target !== selectedNode.id
      )
    );
    setSelectedNode(null);
  }, [selectedNode, setNodes, setEdges]);

  const updateLabel = (label: string) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) =>
        n.id === selectedNode.id ? { ...n, data: { ...n.data, label } } : n
      )
    );
    setSelectedNode((prev) =>
      prev ? { ...prev, data: { ...prev.data, label } } : null
    );
  };

  const updatePosition = (axis: 'x' | 'y', value: number) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) =>
        n.id === selectedNode.id
          ? { ...n, position: { ...n.position, [axis]: value } }
          : n
      )
    );
    setSelectedNode((prev) =>
      prev
        ? { ...prev, position: { ...prev.position, [axis]: value } }
        : null
    );
  };

  const loadSample = () => {
    counters.current = { interface: 1, nftables: 2, 'traffic-control': 3 };
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
    const data = JSON.stringify({ nodes, edges }, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'net-fiddle.json';
    a.click();
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
        } catch {
          alert('Invalid JSON file');
        }
      };
      reader.readAsText(file);
    };
    input.click();
  };

  // Update selectedNode when nodes change (e.g. drag reposition)
  useEffect(() => {
    if (!selectedNode) return;
    const updated = nodes.find((n) => n.id === selectedNode.id);
    if (updated) setSelectedNode(updated);
  }, [nodes]); // eslint-disable-line react-hooks/exhaustive-deps

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (document.activeElement?.tagName === 'INPUT') return;
        deleteSelected();
      }
      if (e.key === 'Escape') setSelectedNode(null);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [deleteSelected]);

  const selectedCfg = selectedNode ? NODE_CONFIG[selectedNode.data.nodeType] : null;

  return (
    <div className="app">
      {/* Header */}
      <header className="header">
        <div className="header-title">
          <FlaskConical size={20} color="#6366f1" />
          Linux Network Topology Designer
          <span className="component-count">{nodes.length} components</span>
        </div>
        <div className="header-actions">
          <button className="btn btn-primary" onClick={loadSample}>
            <FlaskConical size={14} />
            Load Sample
          </button>
          <button className="btn btn-secondary" onClick={doImport}>
            <Upload size={14} />
            Import
          </button>
          <button className="btn btn-success" onClick={doExport}>
            <Download size={14} />
            Export
          </button>
          <button className="btn btn-danger" onClick={clearAll}>
            <Trash2 size={14} />
            Clear All
          </button>
        </div>
      </header>

      <div className="body">
        {/* Sidebar */}
        <aside className="sidebar">
          <div>
            <div className="sidebar-section-title">Network Components</div>
            {SIDEBAR_ITEMS.map((nodeType) => {
              const cfg = NODE_CONFIG[nodeType];
              const { Icon } = cfg;
              return (
                <div
                  key={nodeType}
                  className="component-item"
                  draggable
                  onDragStart={(e) => onDragStart(e, nodeType)}
                >
                  <div className="component-icon">
                    <Icon size={16} color={cfg.color} />
                  </div>
                  {cfg.label}
                </div>
              );
            })}
          </div>

          <div className="quick-guide">
            <div className="quick-guide-title">Quick Guide</div>
            <ul>
              <li>Drag components to canvas</li>
              <li>Click to select and edit</li>
              <li>Drag nodes to reposition</li>
              <li>Drag handle to connect</li>
              <li>Del to delete selected</li>
            </ul>
          </div>
        </aside>

        {/* Canvas */}
        <div
          ref={wrapperRef}
          className={`canvas-wrapper${isDragOver ? ' drag-over' : ''}`}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
        >
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            onInit={setRfInstance}
            nodeTypes={nodeTypes}
            fitView={nodes.length > 0}
            deleteKeyCode={null}
          >
            <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#e2e8f0" />
            <Controls />
          </ReactFlow>
        </div>

        {/* Properties Panel */}
        {selectedNode && selectedCfg && (
          <aside className="properties-panel">
            <div className="properties-header">
              <span className="properties-title">Properties</span>
              <div className="properties-header-actions">
                <button
                  className="icon-btn danger"
                  title="Delete"
                  onClick={deleteSelected}
                >
                  <Trash2 size={15} />
                </button>
                <button
                  className="icon-btn"
                  title="Close"
                  onClick={() => setSelectedNode(null)}
                >
                  <X size={15} />
                </button>
              </div>
            </div>

            <div className="prop-group">
              <span className="prop-label">Type</span>
              <input
                className="prop-input"
                readOnly
                value={selectedCfg.label}
              />
            </div>

            <div className="prop-group">
              <span className="prop-label">Label</span>
              <input
                className="prop-input"
                value={selectedNode.data.label}
                onChange={(e) => updateLabel(e.target.value)}
              />
            </div>

            <div className="prop-group">
              <span className="prop-label">Position</span>
              <div className="position-row">
                <div className="position-field">
                  <label>X</label>
                  <input
                    className="prop-input"
                    type="number"
                    value={Math.round(selectedNode.position.x)}
                    onChange={(e) => updatePosition('x', Number(e.target.value))}
                  />
                </div>
                <div className="position-field">
                  <label>Y</label>
                  <input
                    className="prop-input"
                    type="number"
                    value={Math.round(selectedNode.position.y)}
                    onChange={(e) => updatePosition('y', Number(e.target.value))}
                  />
                </div>
              </div>
            </div>

            <div className="config-section">
              <div className="config-title">{selectedCfg.configTitle}</div>
              <ul className="config-items">
                {selectedCfg.configItems.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </div>
          </aside>
        )}
      </div>

      {/* Status Bar */}
      <footer className="status-bar">
        <div className="status-shortcuts">
          <div className="shortcut">
            <span className="kbd">Del</span>
            Delete selected
          </div>
          <div className="shortcut">
            <span className="kbd">Drag</span>
            handle to connect nodes
          </div>
          <div className="shortcut">
            <span className="kbd">Esc</span>
            Deselect
          </div>
        </div>
        <button className="status-help" title="Help">?</button>
      </footer>
    </div>
  );
}
