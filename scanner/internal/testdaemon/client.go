// Package testdaemon provides a client for the test setup daemon.
// Tests import this package to create and tear down network namespaces,
// attach BPF programs, and start processes that hold sockets.
//
// The daemon address defaults to http://localhost:7777 and can be overridden
// by the NETFIDDLE_DAEMON_ADDR environment variable.
package testdaemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Client talks to the test setup daemon over HTTP.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// NewClient returns a Client pointed at the daemon. The address is taken from
// NETFIDDLE_DAEMON_ADDR if set, otherwise defaults to http://localhost:7777.
func NewClient() *Client {
	addr := os.Getenv("NETFIDDLE_DAEMON_ADDR")
	if addr == "" {
		addr = "http://localhost:7777"
	}
	return &Client{
		BaseURL:    addr,
		httpClient: &http.Client{},
	}
}

// ProcessRequest describes a socket-holding process to start.
type ProcessRequest struct {
	Type string `json:"type"`          // "tcp-listen", "udp-bind", "tcp-connect"
	Port int    `json:"port,omitempty"` // port to bind or connect to
	Addr string `json:"addr,omitempty"` // remote address for tcp-connect
}

// NsItem is one namespace returned by ListNs.
type NsItem struct {
	Name  string `json:"name"`
	Inode uint64 `json:"inode"`
}

// Healthz returns nil if the daemon is reachable and healthy.
func (c *Client) Healthz() error {
	resp, err := c.httpClient.Get(c.BaseURL + "/healthz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon unhealthy: %s", resp.Status)
	}
	return nil
}

// Reset kills all managed processes and deletes all non-host namespaces.
func (c *Client) Reset() error {
	return c.doVoid("POST", "/reset", nil)
}

// ListNs returns all currently existing named namespaces plus linux-host.
func (c *Client) ListNs() ([]NsItem, error) {
	resp, err := c.do("GET", "/ns", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var items []NsItem
	return items, json.NewDecoder(resp.Body).Decode(&items)
}

// CreateNs creates a named network namespace via ip netns add.
func (c *Client) CreateNs(name string) error {
	return c.doVoid("POST", "/ns/"+name, nil)
}

// DeleteNs deletes a named namespace and kills any processes inside it.
func (c *Client) DeleteNs(name string) error {
	return c.doVoid("DELETE", "/ns/"+name, nil)
}

// IP runs an ip subcommand inside the named namespace.
// Use "linux-host" for the host namespace.
//
// Example:
//
//	c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1")
func (c *Client) IP(ns string, args ...string) error {
	return c.doVoid("POST", "/ns/"+ns+"/ip", map[string][]string{"cmdline": args})
}

// TC runs a tc subcommand inside the named namespace.
//
// Example:
//
//	c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact")
func (c *Client) TC(ns string, args ...string) error {
	return c.doVoid("POST", "/ns/"+ns+"/tc", map[string][]string{"cmdline": args})
}

// NftCmd runs a single nft command inside the named namespace.
//
// Example:
//
//	c.NftCmd("ns-a", "flush", "ruleset")
func (c *Client) NftCmd(ns string, args ...string) error {
	return c.doVoid("POST", "/ns/"+ns+"/nft", map[string][]string{"cmdline": args})
}

// NftScript applies a complete nft ruleset fragment inside the named namespace.
// The script is passed to "nft -f -" via stdin.
//
// Example:
//
//	c.NftScript("ns-a", `table inet filter { chain input { type filter hook input priority 0; } }`)
func (c *Client) NftScript(ns, script string) error {
	return c.doVoid("POST", "/ns/"+ns+"/nft", map[string]string{"script": script})
}

// ListBpfObjects returns the names of BPF stub objects the daemon has built.
func (c *Client) ListBpfObjects() ([]string, error) {
	resp, err := c.do("GET", "/bpf/objects", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var names []string
	return names, json.NewDecoder(resp.Body).Decode(&names)
}

// AttachXDP attaches an XDP program from a named BPF stub object to an interface.
// objectName must be one of the names returned by ListBpfObjects (e.g. "xdp_pass.o").
func (c *Client) AttachXDP(ns, iface, objectName, section string) error {
	return c.doVoid("POST", "/ns/"+ns+"/bpf/xdp", map[string]string{
		"iface":   iface,
		"object":  objectName,
		"section": section,
	})
}

// AttachTCBPF attaches a TC BPF program to an interface.
// direction is "ingress" or "egress".
func (c *Client) AttachTCBPF(ns, iface, direction, objectName, section string) error {
	return c.doVoid("POST", "/ns/"+ns+"/bpf/tc", map[string]string{
		"iface":     iface,
		"direction": direction,
		"object":    objectName,
		"section":   section,
	})
}

// ListProcesses returns the PIDs of all managed processes in the namespace.
func (c *Client) ListProcesses(ns string) ([]int, error) {
	resp, err := c.do("GET", "/ns/"+ns+"/process", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var body struct {
		PIDs []int `json:"pids"`
	}
	return body.PIDs, json.NewDecoder(resp.Body).Decode(&body)
}

// StartProcess starts a socket-holding process inside the namespace.
// It blocks until the process signals that its socket is bound.
func (c *Client) StartProcess(ns string, req ProcessRequest) (pid int, err error) {
	resp, err := c.do("POST", "/ns/"+ns+"/process", req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return 0, err
	}
	var body struct {
		PID int `json:"pid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.PID, nil
}

// KillProcess kills a managed process by PID.
func (c *Client) KillProcess(ns string, pid int) error {
	return c.doVoid("DELETE", fmt.Sprintf("/ns/%s/process/%d", ns, pid), nil)
}

// ---- internal helpers ---------------------------------------------------

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) doVoid(method, path string, body any) error {
	resp, err := c.do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}
	var errBody struct {
		Error string `json:"error"`
	}
	data, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(data, &errBody)
	msg := errBody.Error
	if msg == "" {
		msg = string(data)
	}
	return fmt.Errorf("daemon %s: %s", resp.Status, msg)
}
