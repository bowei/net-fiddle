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
  type XYPosition,
  type ReactFlowInstance,
} from 'reactflow';
import 'reactflow/dist/style.css';

import CustomNode, { type NetNodeData } from './CustomNode';
import ContainerNode from './ContainerNode';
import { REGISTRY, SIDEBAR_ITEMS } from './components/registry';
import { ContainerComponentDef } from './components/base';
import { DragContext } from './DragContext';
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

// Returns the canvas-absolute position of a node, accounting for parent offset.
// Container nodes are always top-level, so positionAbsolute is their actual position.
function absolutePosition(node: Node): XYPosition {
  return node.positionAbsolute ?? node.position;
}

// Finds the topmost container node whose bounding box contains the given canvas point.
function findContainerAt(pos: XYPosition, containers: Node[]): Node | undefined {
  return containers.find((c) => {
    const w = c.width ?? (c.style?.width as number) ?? 300;
    const h = c.height ?? (c.style?.height as number) ?? 200;
    return (
      pos.x >= c.position.x &&
      pos.x <= c.position.x + w &&
      pos.y >= c.position.y &&
      pos.y <= c.position.y + h
    );
  });
}

const SAMPLE_NODES: Node<NetNodeData>[] = [
  {
    id: 'ns-1',
    type: 'containerNode',
    position: { x: 40, y: 40 },
    style: { width: 320, height: 240 },
    zIndex: -1,
    data: { nodeType: 'namespace', label: 'ns-1' },
  },
  {
    id: 'interface-1',
    type: 'netNode',
    parentNode: 'ns-1',
    position: { x: 80, y: 80 },
    data: { nodeType: 'interface', label: 'interface-1' },
  },
  {
    id: 'nftables-2',
    type: 'netNode',
    position: { x: 430, y: 80 },
    data: { nodeType: 'nftables', label: 'nftables-2' },
  },
  {
    id: 'tc-3',
    type: 'netNode',
    position: { x: 430, y: 200 },
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
  const [hoverContainerId, setHoverContainerId] = useState<string | null>(null);
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

  const nextLabel = (nodeType: string): string => {
    const def = REGISTRY.get(nodeType);
    const prefix = def?.typeLabel ?? nodeType;
    const count = (counters.current[nodeType] ?? 0) + 1;
    counters.current[nodeType] = count;
    return `${prefix}-${count}`;
  };

  // --- Sidebar drag-drop onto canvas ---

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
      const pos = rfInstance.project({
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });
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

      const def = REGISTRY.get(nodeType);
      if (!def) return;

      const bounds = wrapperRef.current.getBoundingClientRect();
      const canvasPos = rfInstance.project({
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });

      const label = nextLabel(nodeType);

      if (def instanceof ContainerComponentDef) {
        const newNode: Node<NetNodeData> = {
          id: label,
          type: 'containerNode',
          position: canvasPos,
          style: { width: def.defaultWidth, height: def.defaultHeight },
          zIndex: -1,
          data: { nodeType, label },
        };
        setNodes((nds) => [...nds, newNode]);
      } else {
        const containers = nodes.filter((n) => n.type === 'containerNode');
        const parent = findContainerAt(canvasPos, containers);
        const newNode: Node<NetNodeData> = {
          id: label,
          type: 'netNode',
          position: parent
            ? { x: canvasPos.x - parent.position.x, y: canvasPos.y - parent.position.y }
            : canvasPos,
          ...(parent ? { parentNode: parent.id } : {}),
          data: { nodeType, label },
        };
        setNodes((nds) => [...nds, newNode]);
      }
    },
    [nodes, rfInstance, setNodes]
  );

  // --- Canvas node drag (for drop-target highlight feedback) ---

  const onNodeDrag = useCallback(
    (_: React.MouseEvent, draggedNode: Node<NetNodeData>) => {
      if (draggedNode.type === 'containerNode') {
        setHoverContainerId(null);
        return;
      }
      const absPos = absolutePosition(draggedNode);
      const found = findContainerAt(
        absPos,
        nodes.filter((n) => n.type === 'containerNode' && n.id !== draggedNode.parentNode)
      );
      setHoverContainerId(found?.id ?? null);
    },
    [nodes]
  );

  // Reparent a node after it has been dropped:
  // - If it landed inside a container it wasn't already in → assign parentNode
  // - If it was a child and landed outside all containers → remove parentNode
  const onNodeDragStop = useCallback(
    (_: React.MouseEvent, draggedNode: Node<NetNodeData>) => {
      setHoverContainerId(null);
      if (draggedNode.type === 'containerNode') return;

      const absPos = absolutePosition(draggedNode);
      const containers = nodes.filter((n) => n.type === 'containerNode');
      const newParent = findContainerAt(absPos, containers);
      const oldParentId = draggedNode.parentNode;

      if (newParent?.id === oldParentId) return; // no change

      setNodes((nds) =>
        nds.map((n) => {
          if (n.id !== draggedNode.id) return n;
          if (newParent) {
            return {
              ...n,
              parentNode: newParent.id,
              position: {
                x: absPos.x - newParent.position.x,
                y: absPos.y - newParent.position.y,
              },
            };
          }
          // leaving a container — restore absolute position, drop parentNode
          return { ...n, parentNode: undefined, position: absPos };
        })
      );
    },
    [nodes, setNodes]
  );

  // --- CRUD actions ---

  const deleteSelected = useCallback(() => {
    if (!selectedNode) return;
    setNodes((nds) => {
      const toDelete = new Set([selectedNode.id]);
      // Delete children when a container is removed
      if (selectedNode.type === 'containerNode') {
        nds.forEach((n) => { if (n.parentNode === selectedNode.id) toDelete.add(n.id); });
      }
      return nds.filter((n) => !toDelete.has(n.id));
    });
    setEdges((eds) =>
      eds.filter((e) => e.source !== selectedNode.id && e.target !== selectedNode.id)
    );
    setSelectedNode(null);
  }, [selectedNode, setNodes, setEdges]);

  const updateLabel = (label: string) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) => (n.id === selectedNode.id ? { ...n, data: { ...n.data, label } } : n))
    );
    setSelectedNode((prev) => (prev ? { ...prev, data: { ...prev.data, label } } : null));
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
      prev ? { ...prev, position: { ...prev.position, [axis]: value } } : null
    );
  };

  const loadSample = () => {
    counters.current = { namespace: 1, interface: 1, nftables: 2, 'traffic-control': 3 };
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

  // Keep selectedNode in sync when nodes move on the canvas.
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

  const selectedDef = selectedNode ? REGISTRY.get(selectedNode.data.nodeType) : null;
  const selectedIsContainer = selectedDef instanceof ContainerComponentDef;

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
            {SIDEBAR_ITEMS.map((def) => {
              const Icon = def.icon;
              return (
                <div
                  key={def.type}
                  className="component-item"
                  draggable
                  onDragStart={(e) => onDragStart(e, def.type)}
                >
                  <div className="component-icon">
                    <Icon size={16} color={def.color} />
                  </div>
                  {def.label}
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
              <li>Drop onto namespace to nest</li>
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
          <DragContext.Provider value={hoverContainerId}>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onNodeClick={onNodeClick}
              onPaneClick={onPaneClick}
              onNodeDrag={onNodeDrag}
              onNodeDragStop={onNodeDragStop}
              onInit={setRfInstance}
              nodeTypes={nodeTypes}
              fitView={nodes.length > 0}
              deleteKeyCode={null}
            >
              <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#e2e8f0" />
              <Controls />
            </ReactFlow>
          </DragContext.Provider>
        </div>

        {/* Properties Panel */}
        {selectedNode && selectedDef && (
          <aside className="properties-panel">
            <div className="properties-header">
              <span className="properties-title">Properties</span>
              <div className="properties-header-actions">
                <button className="icon-btn danger" title="Delete" onClick={deleteSelected}>
                  <Trash2 size={15} />
                </button>
                <button className="icon-btn" title="Close" onClick={() => setSelectedNode(null)}>
                  <X size={15} />
                </button>
              </div>
            </div>

            <div className="prop-group">
              <span className="prop-label">Type</span>
              <input className="prop-input" readOnly value={selectedDef.label} />
            </div>

            <div className="prop-group">
              <span className="prop-label">Label</span>
              <input
                className="prop-input"
                value={selectedNode.data.label}
                onChange={(e) => updateLabel(e.target.value)}
              />
            </div>

            {!selectedIsContainer && (
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
            )}

            {selectedIsContainer && (
              <div className="prop-group">
                <span className="prop-label">Size</span>
                <div className="position-row">
                  <div className="position-field">
                    <label>W</label>
                    <input
                      className="prop-input"
                      type="number"
                      readOnly
                      value={Math.round(
                        (selectedNode.width ?? (selectedNode.style?.width as number) ?? 0)
                      )}
                    />
                  </div>
                  <div className="position-field">
                    <label>H</label>
                    <input
                      className="prop-input"
                      type="number"
                      readOnly
                      value={Math.round(
                        (selectedNode.height ?? (selectedNode.style?.height as number) ?? 0)
                      )}
                    />
                  </div>
                </div>
              </div>
            )}

            <div className="config-section">
              <div className="config-title">{selectedDef.configTitle}</div>
              <ul className="config-items">
                {selectedDef.configItems.map((item) => (
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
            onto namespace to nest
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
