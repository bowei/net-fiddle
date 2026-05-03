package topology

type Topology struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`                  // "netNode" or "containerNode"
	Position   Position   `json:"position"`
	ParentNode string     `json:"parentNode,omitempty"`
	ZIndex     *int       `json:"zIndex,omitempty"`
	Style      *NodeStyle `json:"style,omitempty"`
	Data       NodeData   `json:"data"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type NodeStyle struct {
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type NodeData struct {
	NodeType   string            `json:"nodeType"`
	Label      string            `json:"label"`
	Config     map[string]string `json:"config,omitempty"`
	VethPairID string            `json:"vethPairId,omitempty"`
}

type Edge struct {
	ID           string     `json:"id"`
	Source       string     `json:"source"`
	SourceHandle string     `json:"sourceHandle"`
	Target       string     `json:"target"`
	TargetHandle string     `json:"targetHandle"`
	Data         *EdgeData  `json:"data,omitempty"`
	ClassName    string     `json:"className,omitempty"`
	Style        *EdgeStyle `json:"style,omitempty"`
	Label        string     `json:"label,omitempty"`
}

type EdgeData struct {
	VethLink bool `json:"vethLink,omitempty"`
}

type EdgeStyle struct {
	Stroke          string  `json:"stroke,omitempty"`
	StrokeWidth     float64 `json:"strokeWidth,omitempty"`
	StrokeDashArray string  `json:"strokeDasharray,omitempty"`
}
