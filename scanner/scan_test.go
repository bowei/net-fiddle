//go:build linux

package scan_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"net-fiddle/scanner/internal/builder"
	"net-fiddle/scanner/internal/collector"
	"net-fiddle/scanner/internal/layout"
	"net-fiddle/scanner/internal/linker"
	"net-fiddle/scanner/internal/testdaemon"
	"net-fiddle/scanner/internal/topology"
)

// ============================================================
// Helpers
// ============================================================

func requireDaemon(t *testing.T) *testdaemon.Client {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	c := testdaemon.NewClient()
	if err := c.Healthz(); err != nil {
		t.Skipf("test daemon unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Reset(); err != nil {
			t.Logf("cleanup reset: %v", err)
		}
	})
	return c
}

type scanResult struct {
	Snaps  []collector.NsSnapshot
	Pairs  map[linker.PairKey]linker.PairInfo
	Result builder.BuildResult
}

func doScan(t *testing.T) scanResult {
	t.Helper()
	return doScanOpts(t, false)
}

func doScanNoSockets(t *testing.T) scanResult {
	t.Helper()
	return doScanOpts(t, true)
}

func doScanOpts(t *testing.T, noSockets bool) scanResult {
	t.Helper()
	nses, err := collector.EnumerateNamespaces()
	if err != nil {
		t.Fatalf("EnumerateNamespaces: %v", err)
	}
	var snaps []collector.NsSnapshot
	for _, ns := range nses {
		snaps = append(snaps, collector.CollectAll(ns, noSockets))
	}
	pairs := linker.Link(snaps)
	result := builder.Build(snaps, pairs)
	return scanResult{Snaps: snaps, Pairs: pairs, Result: result}
}

func snap(t *testing.T, sr scanResult, name string) *collector.NsSnapshot {
	t.Helper()
	for i := range sr.Snaps {
		if sr.Snaps[i].Ns.Name == name {
			return &sr.Snaps[i]
		}
	}
	t.Fatalf("namespace %q not found in snapshots", name)
	return nil
}

func snapExists(sr scanResult, name string) bool {
	for _, s := range sr.Snaps {
		if s.Ns.Name == name {
			return true
		}
	}
	return false
}

func iface(t *testing.T, s *collector.NsSnapshot, name string) *collector.InterfaceInfo {
	t.Helper()
	for i := range s.Interfaces {
		if s.Interfaces[i].IfName == name {
			return &s.Interfaces[i]
		}
	}
	t.Fatalf("interface %q not found in namespace %q", name, s.Ns.Name)
	return nil
}

func hasFlag(ifc *collector.InterfaceInfo, flag string) bool {
	for _, f := range ifc.Flags {
		if strings.EqualFold(f, flag) {
			return true
		}
	}
	return false
}

func nodeOf(t *testing.T, sr scanResult, id string) topology.Node {
	t.Helper()
	for _, n := range sr.Result.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("node %q not found", id)
	return topology.Node{}
}

func nodeExists(sr scanResult, id string) bool {
	for _, n := range sr.Result.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

func edgeOf(t *testing.T, sr scanResult, src, tgt string) topology.Edge {
	t.Helper()
	for _, e := range sr.Result.Edges {
		if e.Source == src && e.Target == tgt {
			return e
		}
	}
	t.Fatalf("edge %q → %q not found", src, tgt)
	return topology.Edge{}
}

func edgeExists(sr scanResult, src, tgt string) bool {
	for _, e := range sr.Result.Edges {
		if e.Source == src && e.Target == tgt {
			return true
		}
	}
	return false
}

func pairOf(t *testing.T, sr scanResult, nsName, ifaceName string) linker.PairInfo {
	t.Helper()
	s := snap(t, sr, nsName)
	ifc := iface(t, s, ifaceName)
	key := linker.PairKey{NsInode: s.Ns.Inode, IfIndex: ifc.IfIndex}
	info, ok := sr.Pairs[key]
	if !ok {
		t.Fatalf("no pair entry for (%s, %s)", nsName, ifaceName)
	}
	return info
}

func pairExists(sr scanResult, nsName, ifaceName string) bool {
	for _, s := range sr.Snaps {
		if s.Ns.Name != nsName {
			continue
		}
		for _, ifc := range s.Interfaces {
			if ifc.IfName == ifaceName {
				key := linker.PairKey{NsInode: s.Ns.Inode, IfIndex: ifc.IfIndex}
				_, ok := sr.Pairs[key]
				return ok
			}
		}
	}
	return false
}

func nodesOfType(sr scanResult, nodeType, parentID string) []topology.Node {
	var out []topology.Node
	for _, n := range sr.Result.Nodes {
		if n.Data.NodeType == nodeType && n.ParentNode == parentID {
			out = append(out, n)
		}
	}
	return out
}

func vethLinkEdges(sr scanResult) []topology.Edge {
	var out []topology.Edge
	for _, e := range sr.Result.Edges {
		if e.Data != nil && e.Data.VethLink {
			out = append(out, e)
		}
	}
	return out
}

func requireBpfStubs(t *testing.T, c *testdaemon.Client) {
	t.Helper()
	objs, err := c.ListBpfObjects()
	if err != nil || len(objs) == 0 {
		t.Skip("BPF stubs not available (clang required)")
	}
}

func mustIP(t *testing.T, c *testdaemon.Client, ns string, args ...string) {
	t.Helper()
	if err := c.IP(ns, args...); err != nil {
		t.Skipf("ip %v in %s failed (kernel may not support this): %v", args, ns, err)
	}
}

// findSocketNode returns the first socket node in the given namespace container.
func findSocketNode(sr scanResult, nsContainerID string) *topology.Node {
	nodes := nodesOfType(sr, "socket", nsContainerID)
	if len(nodes) == 0 {
		return nil
	}
	return &nodes[0]
}

// containerID returns the ReactFlow container node ID for a namespace.
// The builder uses "ns-" + snap.Ns.Name.
func containerID(nsName string) string {
	return "ns-" + nsName
}

// ============================================================
// Section 1: Namespace Enumeration
// ============================================================

func Test_NS01_HostAlwaysPresent(t *testing.T) {
	requireDaemon(t)

	nses, err := collector.EnumerateNamespaces()
	if err != nil {
		t.Fatalf("EnumerateNamespaces: %v", err)
	}
	if len(nses) == 0 {
		t.Fatal("expected at least one namespace, got none")
	}

	var hostCount int
	var host *collector.NsInfo
	for i := range nses {
		if nses[i].Name == "host" {
			hostCount++
			host = &nses[i]
		}
	}
	if hostCount != 1 {
		t.Fatalf("expected exactly 1 entry with Name==\"host\", got %d", hostCount)
	}
	if host.Inode == 0 {
		t.Error("host Inode is zero")
	}
	if host.Path != "/proc/1/ns/net" {
		t.Errorf("host Path = %q, want /proc/1/ns/net", host.Path)
	}
}

func Test_NS02_SingleNamedNamespace(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}

	nses, err := collector.EnumerateNamespaces()
	if err != nil {
		t.Fatalf("EnumerateNamespaces: %v", err)
	}

	var nsA, host *collector.NsInfo
	for i := range nses {
		switch nses[i].Name {
		case "ns-a":
			nsA = &nses[i]
		case "host":
			host = &nses[i]
		}
	}
	if nsA == nil {
		t.Fatal("no entry with Name==\"ns-a\"")
	}
	if host == nil {
		t.Fatal("no entry with Name==\"host\"")
	}
	if nsA.Inode == host.Inode {
		t.Error("ns-a and host share the same Inode")
	}
	if nsA.Path != "/var/run/netns/ns-a" {
		t.Errorf("ns-a Path = %q, want /var/run/netns/ns-a", nsA.Path)
	}
}

func Test_NS03_MultipleNamedNamespaces(t *testing.T) {
	c := requireDaemon(t)
	for _, name := range []string{"ns-a", "ns-b", "ns-c"} {
		if err := c.CreateNs(name); err != nil {
			t.Fatalf("CreateNs(%s): %v", name, err)
		}
	}

	nses, err := collector.EnumerateNamespaces()
	if err != nil {
		t.Fatalf("EnumerateNamespaces: %v", err)
	}

	required := map[string]bool{"host": false, "ns-a": false, "ns-b": false, "ns-c": false}
	for _, ns := range nses {
		required[ns.Name] = true
	}
	for name, found := range required {
		if !found {
			t.Errorf("namespace %q not found", name)
		}
	}

	inodes := make(map[uint64]string)
	for _, ns := range nses {
		if prev, dup := inodes[ns.Inode]; dup {
			t.Errorf("duplicate inode %d for %q and %q", ns.Inode, prev, ns.Name)
		}
		inodes[ns.Inode] = ns.Name
	}
}

func Test_NS04_DeletedNamespaceAbsent(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-gone"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.DeleteNs("ns-gone"); err != nil {
		t.Fatalf("DeleteNs: %v", err)
	}

	nses, err := collector.EnumerateNamespaces()
	if err != nil {
		t.Fatalf("EnumerateNamespaces: %v", err)
	}
	for _, ns := range nses {
		if ns.Name == "ns-gone" {
			t.Error("deleted namespace \"ns-gone\" still present in enumeration")
		}
	}
}

// ============================================================
// Section 2: Interface Collection
// ============================================================

func Test_IF01_EmptyNamespaceNoInterfaces(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-empty"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-empty")
	if len(s.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces, got %d: %v", len(s.Interfaces), s.Interfaces)
	}
}

func Test_IF02_SingleDummyInterface(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("IP add dummy: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "dummy0", "up"); err != nil {
		t.Fatalf("IP set up: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d: %v", len(s.Interfaces), s.Interfaces)
	}
	ifc := iface(t, s, "dummy0")
	if ifc.Kind != "ether" && ifc.Kind != "dummy" {
		t.Errorf("Kind = %q, want \"ether\" or \"dummy\"", ifc.Kind)
	}
	if ifc.PeerIfIndex != 0 {
		t.Errorf("PeerIfIndex = %d, want 0", ifc.PeerIfIndex)
	}
}

func Test_IF03_InterfaceFlagsReflectUPState(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("IP add dummy: %v", err)
	}

	sr1 := doScan(t)
	s1 := snap(t, sr1, "ns-a")
	if hasFlag(iface(t, s1, "dummy0"), "UP") {
		t.Error("interface should not have UP flag before setting up")
	}

	if err := c.IP("ns-a", "link", "set", "dummy0", "up"); err != nil {
		t.Fatalf("IP set up: %v", err)
	}

	sr2 := doScan(t)
	s2 := snap(t, sr2, "ns-a")
	if !hasFlag(iface(t, s2, "dummy0"), "UP") {
		t.Error("interface should have UP flag after setting up")
	}
}

func Test_IF04_MultipleInterfaces(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	for _, name := range []string{"dummy0", "dummy1", "dummy2"} {
		if err := c.IP("ns-a", "link", "add", name, "type", "dummy"); err != nil {
			t.Fatalf("IP add %s: %v", name, err)
		}
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Interfaces) != 3 {
		t.Fatalf("expected 3 interfaces, got %d: %v", len(s.Interfaces), s.Interfaces)
	}
	names := make(map[string]bool)
	for _, ifc := range s.Interfaces {
		names[ifc.IfName] = true
	}
	for _, want := range []string{"dummy0", "dummy1", "dummy2"} {
		if !names[want] {
			t.Errorf("interface %q not found", want)
		}
	}
}

// ============================================================
// Section 3: Veth Pair Detection — Same Namespace
// ============================================================

func Test_VETH01_SameNamespacePair(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("IP add veth pair: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")

	if len(s.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d: %v", len(s.Interfaces), s.Interfaces)
	}
	v0 := iface(t, s, "veth0")
	v1 := iface(t, s, "veth1")

	if v0.Kind != "veth" {
		t.Errorf("veth0.Kind = %q, want \"veth\"", v0.Kind)
	}
	if v1.Kind != "veth" {
		t.Errorf("veth1.Kind = %q, want \"veth\"", v1.Kind)
	}
	if v0.PeerIfIndex != v1.IfIndex {
		t.Errorf("veth0.PeerIfIndex=%d != veth1.IfIndex=%d", v0.PeerIfIndex, v1.IfIndex)
	}
	if v1.PeerIfIndex != v0.IfIndex {
		t.Errorf("veth1.PeerIfIndex=%d != veth0.IfIndex=%d", v1.PeerIfIndex, v0.IfIndex)
	}
	if v0.LinkNetNsID != nil {
		t.Errorf("veth0.LinkNetNsID = %v, want nil (peer is in same ns)", v0.LinkNetNsID)
	}

	p0 := pairOf(t, sr, "ns-a", "veth0")
	p1 := pairOf(t, sr, "ns-a", "veth1")
	if p0.PairID != p1.PairID {
		t.Errorf("veth0 PairID=%q != veth1 PairID=%q", p0.PairID, p1.PairID)
	}
	if !strings.HasPrefix(p0.PairID, "veth-pair-") {
		t.Errorf("PairID %q does not start with \"veth-pair-\"", p0.PairID)
	}
}

func Test_VETH02_SameNamespacePairMapEntries(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("IP add veth pair: %v", err)
	}

	sr := doScan(t)
	if len(sr.Pairs) != 2 {
		t.Errorf("pairs map has %d entries, want 2 (one per end)", len(sr.Pairs))
	}
	p0 := pairOf(t, sr, "ns-a", "veth0")
	p1 := pairOf(t, sr, "ns-a", "veth1")
	if p0.PairID != p1.PairID {
		t.Errorf("PairIDs differ: veth0=%q veth1=%q", p0.PairID, p1.PairID)
	}
	if p0.Kind != "veth" {
		t.Errorf("Kind = %q, want \"veth\"", p0.Kind)
	}
}

// ============================================================
// Section 4: Veth Pair Detection — Cross-Namespace
// ============================================================

func Test_VETHCROSS01_PeerMovedToOtherNamespace(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("IP add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("IP move veth1 to ns-b: %v", err)
	}

	sr := doScan(t)
	sA := snap(t, sr, "ns-a")
	sB := snap(t, sr, "ns-b")

	if len(sA.Interfaces) != 1 {
		t.Errorf("ns-a: expected 1 interface, got %d", len(sA.Interfaces))
	}
	if len(sB.Interfaces) != 1 {
		t.Errorf("ns-b: expected 1 interface, got %d", len(sB.Interfaces))
	}

	v0 := iface(t, sA, "veth0")
	v1 := iface(t, sB, "veth1")
	if v0.PeerIfIndex != v1.IfIndex {
		t.Errorf("veth0.PeerIfIndex=%d != veth1.IfIndex=%d", v0.PeerIfIndex, v1.IfIndex)
	}
	if v0.LinkNetNsID == nil {
		t.Error("veth0.LinkNetNsID should be non-nil when peer is in another namespace")
	}

	p0 := pairOf(t, sr, "ns-a", "veth0")
	p1 := pairOf(t, sr, "ns-b", "veth1")
	if p0.PairID != p1.PairID {
		t.Errorf("PairID mismatch: ns-a/veth0=%q, ns-b/veth1=%q", p0.PairID, p1.PairID)
	}
}

func Test_VETHCROSS02_TwoIndependentPairs(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth0/veth1: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth2", "type", "veth", "peer", "name", "veth3"); err != nil {
		t.Fatalf("add veth2/veth3: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth3", "netns", "ns-c"); err != nil {
		t.Fatalf("move veth3: %v", err)
	}

	sr := doScan(t)
	if len(sr.Pairs) != 4 {
		t.Errorf("pairs map has %d entries, want 4 (2 pairs × 2 ends)", len(sr.Pairs))
	}

	p0 := pairOf(t, sr, "ns-a", "veth0")
	p2 := pairOf(t, sr, "ns-a", "veth2")
	if p0.PairID == p2.PairID {
		t.Error("veth0 and veth2 should have different PairIDs")
	}
	if pairOf(t, sr, "ns-a", "veth0").PairID != pairOf(t, sr, "ns-b", "veth1").PairID {
		t.Error("ns-a/veth0 and ns-b/veth1 should have the same PairID")
	}
	if pairOf(t, sr, "ns-a", "veth2").PairID != pairOf(t, sr, "ns-c", "veth3").PairID {
		t.Error("ns-a/veth2 and ns-c/veth3 should have the same PairID")
	}
}

func Test_VETHCROSS03_SameInterfaceNameInBothNamespaces(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	// Create with a temp peer name to avoid collision, then rename in ns-b.
	if err := c.IP("ns-a", "link", "add", "eth0", "type", "veth", "peer", "name", "eth0-peer"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "eth0-peer", "netns", "ns-b"); err != nil {
		t.Fatalf("move eth0-peer to ns-b: %v", err)
	}
	if err := c.IP("ns-b", "link", "set", "eth0-peer", "name", "eth0"); err != nil {
		t.Fatalf("rename in ns-b: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "eth0", "up"); err != nil {
		t.Fatalf("set ns-a/eth0 up: %v", err)
	}
	if err := c.IP("ns-b", "link", "set", "eth0", "up"); err != nil {
		t.Fatalf("set ns-b/eth0 up: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "eth0"); err != nil {
		t.Fatalf("addr ns-a: %v", err)
	}
	if err := c.IP("ns-b", "addr", "add", "10.0.0.2/24", "dev", "eth0"); err != nil {
		t.Fatalf("addr ns-b: %v", err)
	}

	sr := doScan(t)
	// Linker must correctly pair by ifindex, not name.
	p0 := pairOf(t, sr, "ns-a", "eth0")
	p1 := pairOf(t, sr, "ns-b", "eth0")
	if p0.PairID != p1.PairID {
		t.Errorf("same-name veth pair not linked: ns-a/eth0 PairID=%q, ns-b/eth0 PairID=%q", p0.PairID, p1.PairID)
	}

	vle := vethLinkEdges(sr)
	if len(vle) == 0 {
		t.Error("expected at least one veth-link edge")
	}
	for _, e := range vle {
		if e.ClassName != "veth-link" {
			t.Errorf("edge ClassName = %q, want \"veth-link\"", e.ClassName)
		}
		if e.Data == nil || !e.Data.VethLink {
			t.Error("edge Data.VethLink should be true")
		}
	}
}

func Test_VETHCROSS04_IfindexCollisionNotPaired(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	if err := c.IP("ns-c", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy in ns-c: %v", err)
	}

	sr := doScan(t)
	if len(sr.Pairs) != 2 {
		t.Errorf("pairs map has %d entries, want 2 (one pair, two ends)", len(sr.Pairs))
	}
	if pairExists(sr, "ns-c", "dummy0") {
		t.Error("ns-c/dummy0 should not appear in pairs map")
	}
	p0 := pairOf(t, sr, "ns-a", "veth0")
	p1 := pairOf(t, sr, "ns-b", "veth1")
	if p0.PairID != p1.PairID {
		t.Errorf("veth0/veth1 PairIDs differ: %q vs %q", p0.PairID, p1.PairID)
	}
}

func Test_VETHCROSS05_ThreeNamespaceChain(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	// ns-a ↔ ns-b
	if err := c.IP("ns-a", "link", "add", "veth-ab", "type", "veth", "peer", "name", "veth-ba"); err != nil {
		t.Fatalf("add ab pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth-ba", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth-ba: %v", err)
	}
	// ns-b ↔ ns-c
	if err := c.IP("ns-b", "link", "add", "veth-bc", "type", "veth", "peer", "name", "veth-cb"); err != nil {
		t.Fatalf("add bc pair: %v", err)
	}
	if err := c.IP("ns-b", "link", "set", "veth-cb", "netns", "ns-c"); err != nil {
		t.Fatalf("move veth-cb: %v", err)
	}

	sr := doScan(t)
	if len(sr.Pairs) != 4 {
		t.Errorf("pairs map has %d entries, want 4 (2 pairs)", len(sr.Pairs))
	}

	sB := snap(t, sr, "ns-b")
	if len(sB.Interfaces) != 2 {
		t.Errorf("ns-b: expected 2 interfaces (veth-ba, veth-bc), got %d", len(sB.Interfaces))
	}
	iface(t, sB, "veth-ba")
	iface(t, sB, "veth-bc")

	pAB := pairOf(t, sr, "ns-a", "veth-ab")
	pBA := pairOf(t, sr, "ns-b", "veth-ba")
	if pAB.PairID != pBA.PairID {
		t.Error("ns-a/veth-ab and ns-b/veth-ba should share a PairID")
	}
	pBC := pairOf(t, sr, "ns-b", "veth-bc")
	pCB := pairOf(t, sr, "ns-c", "veth-cb")
	if pBC.PairID != pCB.PairID {
		t.Error("ns-b/veth-bc and ns-c/veth-cb should share a PairID")
	}
	if pAB.PairID == pBC.PairID {
		t.Error("ab and bc pairs should have different PairIDs")
	}

	vle := vethLinkEdges(sr)
	if len(vle) != 2 {
		t.Errorf("expected 2 pairLinkEdge edges, got %d", len(vle))
	}
}

// ============================================================
// Section 5: Netkit Pair Detection
// ============================================================

func Test_NK01_NetkitPairBasicDetection(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	mustIP(t, c, "ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
	if err := c.IP("ns-a", "link", "set", "nk1", "netns", "ns-b"); err != nil {
		t.Fatalf("move nk1: %v", err)
	}

	sr := doScan(t)
	sA := snap(t, sr, "ns-a")
	sB := snap(t, sr, "ns-b")

	nk0 := iface(t, sA, "nk0")
	nk1 := iface(t, sB, "nk1")
	if nk0.Kind != "netkit" {
		t.Errorf("nk0.Kind = %q, want \"netkit\"", nk0.Kind)
	}
	if nk1.Kind != "netkit" {
		t.Errorf("nk1.Kind = %q, want \"netkit\"", nk1.Kind)
	}
	if nk0.NetkitMode == "" {
		t.Error("nk0.NetkitMode should be non-empty (primary end)")
	}
	if nk1.NetkitMode != "" {
		t.Errorf("nk1.NetkitMode = %q, want \"\" (peer end)", nk1.NetkitMode)
	}

	p0 := pairOf(t, sr, "ns-a", "nk0")
	p1 := pairOf(t, sr, "ns-b", "nk1")
	if p0.PairID != p1.PairID {
		t.Errorf("PairID mismatch: nk0=%q, nk1=%q", p0.PairID, p1.PairID)
	}
	if !strings.HasPrefix(p0.PairID, "netkit-pair-") {
		t.Errorf("PairID %q does not start with \"netkit-pair-\"", p0.PairID)
	}
	if p0.Kind != "netkit" {
		t.Errorf("Kind = %q, want \"netkit\"", p0.Kind)
	}
}

func Test_NK02_NetkitPrimaryVsPeerNodeType(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	mustIP(t, c, "ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
	if err := c.IP("ns-a", "link", "set", "nk1", "netns", "ns-b"); err != nil {
		t.Fatalf("move nk1: %v", err)
	}

	sr := doScan(t)
	n0 := nodeOf(t, sr, "ns-a-nk0")
	n1 := nodeOf(t, sr, "ns-b-nk1")
	if n0.Data.NodeType != "netkit-primary" {
		t.Errorf("ns-a-nk0 NodeType = %q, want \"netkit-primary\"", n0.Data.NodeType)
	}
	if n1.Data.NodeType != "netkit-peer" {
		t.Errorf("ns-b-nk1 NodeType = %q, want \"netkit-peer\"", n1.Data.NodeType)
	}
}

func Test_NK03_TwoNetkitPairsNotConfused(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	mustIP(t, c, "ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
	if err := c.IP("ns-a", "link", "set", "nk1", "netns", "ns-b"); err != nil {
		t.Fatalf("move nk1: %v", err)
	}
	mustIP(t, c, "ns-a", "link", "add", "nk2", "type", "netkit", "mode", "l3", "peer", "name", "nk3")
	if err := c.IP("ns-a", "link", "set", "nk3", "netns", "ns-c"); err != nil {
		t.Fatalf("move nk3: %v", err)
	}

	sr := doScan(t)
	for _, tc := range []struct {
		id       string
		wantType string
	}{
		{"ns-a-nk0", "netkit-primary"},
		{"ns-a-nk2", "netkit-primary"},
		{"ns-b-nk1", "netkit-peer"},
		{"ns-c-nk3", "netkit-peer"},
	} {
		n := nodeOf(t, sr, tc.id)
		if n.Data.NodeType != tc.wantType {
			t.Errorf("%s NodeType = %q, want %q", tc.id, n.Data.NodeType, tc.wantType)
		}
	}
	p0 := pairOf(t, sr, "ns-a", "nk0")
	p2 := pairOf(t, sr, "ns-a", "nk2")
	if p0.PairID == p2.PairID {
		t.Error("nk0 and nk2 should have different PairIDs")
	}
}

func Test_NK04_MixedVethAndNetkitPairs(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	mustIP(t, c, "ns-a", "link", "add", "nk0", "type", "netkit", "mode", "l3", "peer", "name", "nk1")
	if err := c.IP("ns-a", "link", "set", "nk1", "netns", "ns-b"); err != nil {
		t.Fatalf("move nk1: %v", err)
	}

	sr := doScan(t)
	if len(sr.Pairs) != 4 {
		t.Errorf("pairs map has %d entries, want 4 (2 pairs × 2 ends)", len(sr.Pairs))
	}

	pVeth := pairOf(t, sr, "ns-a", "veth0")
	if !strings.HasPrefix(pVeth.PairID, "veth-pair-") {
		t.Errorf("veth PairID %q should start with \"veth-pair-\"", pVeth.PairID)
	}
	pNk := pairOf(t, sr, "ns-a", "nk0")
	if !strings.HasPrefix(pNk.PairID, "netkit-pair-") {
		t.Errorf("netkit PairID %q should start with \"netkit-pair-\"", pNk.PairID)
	}

	if nodeOf(t, sr, "ns-a-veth0").Data.NodeType != "veth-end" {
		t.Error("ns-a-veth0 should be veth-end")
	}
	if nodeOf(t, sr, "ns-a-nk0").Data.NodeType != "netkit-primary" {
		t.Error("ns-a-nk0 should be netkit-primary")
	}

	vle := vethLinkEdges(sr)
	if len(vle) != 2 {
		t.Errorf("expected 2 pairLinkEdge edges, got %d", len(vle))
	}
}

// ============================================================
// Section 6: nftables Hook Detection
// ============================================================

func Test_NFT01_NoRulesNoHooks(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 0 {
		t.Errorf("expected 0 nft hooks, got %d: %v", len(s.NftHooks), s.NftHooks)
	}
}

func Test_NFT02_SingleHook(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter {
  chain input { type filter hook input priority 0; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 1 {
		t.Fatalf("expected 1 nft hook, got %d: %v", len(s.NftHooks), s.NftHooks)
	}
	h := s.NftHooks[0]
	if h.Hook != "input" {
		t.Errorf("Hook = %q, want \"input\"", h.Hook)
	}
	hasFilter := false
	for _, tbl := range h.Tables {
		if tbl == "filter" {
			hasFilter = true
		}
	}
	if !hasFilter {
		t.Errorf("Tables %v should contain \"filter\"", h.Tables)
	}
}

func Test_NFT03_MultipleHooksFromOneTable(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter {
  chain pre  { type filter hook prerouting  priority 0; }
  chain in   { type filter hook input       priority 0; }
  chain fwd  { type filter hook forward     priority 0; }
  chain out  { type filter hook output      priority 0; }
  chain post { type filter hook postrouting priority 0; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 5 {
		t.Fatalf("expected 5 nft hooks, got %d: %v", len(s.NftHooks), s.NftHooks)
	}
	wantHooks := map[string]bool{
		"prerouting": false, "input": false, "forward": false,
		"output": false, "postrouting": false,
	}
	for _, h := range s.NftHooks {
		wantHooks[h.Hook] = true
	}
	for hook, found := range wantHooks {
		if !found {
			t.Errorf("hook %q not found", hook)
		}
	}
}

func Test_NFT04_MultipleTablesAtSameHookCollapsed(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter { chain in { type filter hook input priority 0; } }
table inet mangle { chain in { type filter hook input priority -50; } }
`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 1 {
		t.Fatalf("expected 1 nft hook entry (collapsed), got %d: %v", len(s.NftHooks), s.NftHooks)
	}
	h := s.NftHooks[0]
	if h.Hook != "input" {
		t.Errorf("Hook = %q, want \"input\"", h.Hook)
	}
	tables := make(map[string]bool)
	for _, tbl := range h.Tables {
		tables[tbl] = true
	}
	if !tables["filter"] || !tables["mangle"] {
		t.Errorf("Tables %v should contain both \"filter\" and \"mangle\"", h.Tables)
	}

	// Builder should emit exactly one nftables-input node for ns-a
	nsID := containerID("ns-a")
	inputNodes := nodesOfType(sr, "nftables-input", nsID)
	if len(inputNodes) != 1 {
		t.Errorf("expected 1 nftables-input node, got %d", len(inputNodes))
	}
}

func Test_NFT05_BridgeAndNetdevIgnored(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter { chain in { type filter hook input priority 0; } }
table bridge btable { chain pre { type filter hook prerouting priority 0; } }
`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 1 {
		t.Fatalf("expected 1 nft hook (bridge ignored), got %d: %v", len(s.NftHooks), s.NftHooks)
	}
	if s.NftHooks[0].Hook != "input" {
		t.Errorf("hook = %q, want \"input\"", s.NftHooks[0].Hook)
	}
}

func Test_NFT06_NATTableAtPreAndPostrouting(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet nat {
  chain pre  { type nat hook prerouting  priority -100; }
  chain post { type nat hook postrouting priority  100; }
}
`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.NftHooks) != 2 {
		t.Fatalf("expected 2 nft hooks, got %d: %v", len(s.NftHooks), s.NftHooks)
	}
	hooks := make(map[string][]string)
	for _, h := range s.NftHooks {
		hooks[h.Hook] = h.Tables
	}
	for _, hook := range []string{"prerouting", "postrouting"} {
		tables, ok := hooks[hook]
		if !ok {
			t.Errorf("hook %q not found", hook)
			continue
		}
		if len(tables) != 1 || tables[0] != "nat" {
			t.Errorf("hook %q Tables = %v, want [\"nat\"]", hook, tables)
		}
	}
}

// ============================================================
// Section 7: TC Qdisc Detection
// ============================================================

func Test_TC01_NoInterestingQdiscs(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Qdiscs) != 0 {
		t.Errorf("expected 0 qdiscs (noqueue filtered), got %d: %v", len(s.Qdiscs), s.Qdiscs)
	}
}

func Test_TC02_ClsactQdiscSignalsTCHooks(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Qdiscs) != 1 {
		t.Fatalf("expected 1 qdisc, got %d: %v", len(s.Qdiscs), s.Qdiscs)
	}
	if s.Qdiscs[0].IfName != "veth0" || s.Qdiscs[0].Kind != "clsact" {
		t.Errorf("qdisc = %+v, want {veth0 clsact}", s.Qdiscs[0])
	}

	nsID := containerID("ns-a")
	inNodes := nodesOfType(sr, "traffic-control", nsID)
	var hasIngress, hasEgress bool
	for _, n := range inNodes {
		switch n.Data.Config["direction"] {
		case "ingress":
			hasIngress = true
		case "egress":
			hasEgress = true
		}
	}
	if !hasIngress {
		t.Error("expected traffic-control ingress node for veth0")
	}
	if !hasEgress {
		t.Error("expected traffic-control egress node for veth0")
	}
}

func Test_TC03_EgressFqQdisc(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "fq"); err != nil {
		t.Fatalf("add fq: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	found := false
	for _, q := range s.Qdiscs {
		if q.IfName == "veth0" && q.Kind == "fq" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected qdisc {veth0 fq} in %v", s.Qdiscs)
	}

	nsID := containerID("ns-a")
	qdiscNodes := nodesOfType(sr, "qdisc", nsID)
	if len(qdiscNodes) == 0 {
		t.Fatal("expected at least one qdisc node")
	}
	if qdiscNodes[0].Data.Config["type"] != "fq" {
		t.Errorf("qdisc node config.type = %q, want \"fq\"", qdiscNodes[0].Data.Config["type"])
	}
}

func Test_TC04_MultipleInterfacesQdiscsAttributedCorrectly(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth0/1: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth2", "type", "veth", "peer", "name", "veth3"); err != nil {
		t.Fatalf("add veth2/3: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact on veth0: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth2", "root", "fq"); err != nil {
		t.Fatalf("add fq on veth2: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Qdiscs) != 2 {
		t.Errorf("expected 2 qdiscs, got %d: %v", len(s.Qdiscs), s.Qdiscs)
	}

	nsID := containerID("ns-a")
	// qdisc node for veth2
	qdiscNode, ok := findNodeByID(sr, "ns-a-veth2-qdisc")
	if !ok {
		t.Fatal("expected qdisc node for veth2 (ns-a-veth2-qdisc)")
	}
	if qdiscNode.Data.Config["type"] != "fq" {
		t.Errorf("veth2 qdisc node config.type = %q, want \"fq\"", qdiscNode.Data.Config["type"])
	}

	// traffic-control nodes only for veth0
	tcNodes := nodesOfType(sr, "traffic-control", nsID)
	for _, n := range tcNodes {
		if strings.Contains(n.ID, "veth2") {
			t.Errorf("unexpected traffic-control node for veth2: %s", n.ID)
		}
	}
}

func findNodeByID(sr scanResult, id string) (topology.Node, bool) {
	for _, n := range sr.Result.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return topology.Node{}, false
}

// ============================================================
// Section 8: BPF Program Detection
// ============================================================

func Test_BPF01_XDPAttachedToInterface(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp"); err != nil {
		t.Fatalf("AttachXDP: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")

	var xdpEntry *collector.BpfAttachment
	for i := range s.BpfAttach {
		if s.BpfAttach[i].IfName == "veth0" && s.BpfAttach[i].AttachType == "xdp" {
			xdpEntry = &s.BpfAttach[i]
		}
	}
	if xdpEntry == nil {
		t.Fatal("expected BpfAttach entry with IfName=veth0 AttachType=xdp")
	}

	nsID := containerID("ns-a")
	xdpNodes := nodesOfType(sr, "xdp-program", nsID)
	if len(xdpNodes) == 0 {
		t.Fatal("expected xdp-program node as child of ns-a")
	}

	// Ingress chain: veth0 → xdp-program → ...
	xdpNodeID := xdpNodes[0].ID
	if !edgeExists(sr, "ns-a-veth0", xdpNodeID) {
		t.Errorf("expected edge ns-a-veth0 → %s", xdpNodeID)
	}
}

func Test_BPF02_TCBPFIngressFilter(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if err := c.AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc"); err != nil {
		t.Fatalf("AttachTCBPF: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")

	if len(s.BpfAttach) != 1 {
		t.Fatalf("expected 1 BpfAttach entry, got %d", len(s.BpfAttach))
	}
	if s.BpfAttach[0].AttachType != "ingress" {
		t.Errorf("AttachType = %q, want \"ingress\"", s.BpfAttach[0].AttachType)
	}

	nsID := containerID("ns-a")
	tcBpfNodes := nodesOfType(sr, "tc-bpf-program", nsID)
	if len(tcBpfNodes) == 0 {
		t.Fatal("expected tc-bpf-program node")
	}
	if tcBpfNodes[0].Data.Config["direction"] != "ingress" {
		t.Errorf("tc-bpf-program config.direction = %q, want \"ingress\"", tcBpfNodes[0].Data.Config["direction"])
	}
}

func Test_BPF03_TCBPFEgressFilter(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if err := c.AttachTCBPF("ns-a", "veth0", "egress", "tc_pass.o", "tc"); err != nil {
		t.Fatalf("AttachTCBPF: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")

	if len(s.BpfAttach) != 1 {
		t.Fatalf("expected 1 BpfAttach entry, got %d", len(s.BpfAttach))
	}
	if s.BpfAttach[0].AttachType != "egress" {
		t.Errorf("AttachType = %q, want \"egress\"", s.BpfAttach[0].AttachType)
	}

	nsID := containerID("ns-a")
	tcBpfNodes := nodesOfType(sr, "tc-bpf-program", nsID)
	if len(tcBpfNodes) == 0 {
		t.Fatal("expected tc-bpf-program node")
	}
	if tcBpfNodes[0].Data.Config["direction"] != "egress" {
		t.Errorf("tc-bpf-program config.direction = %q, want \"egress\"", tcBpfNodes[0].Data.Config["direction"])
	}
}

func Test_BPF04_TCBPFWithoutClsactIsSchedBpf(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "handle", "1:", "fq"); err != nil {
		t.Fatalf("add root fq: %v", err)
	}
	// Get BPF object path from daemon to pass to tc filter
	objs, err := c.ListBpfObjects()
	if err != nil || len(objs) == 0 {
		t.Skip("BPF stubs not available")
	}
	if err := c.TC("ns-a", "filter", "add", "dev", "veth0", "parent", "1:", "bpf", "da", "obj", objs[0], "sec", "tc"); err != nil {
		t.Skipf("tc filter add failed (kernel may not support): %v", err)
	}

	sr := doScan(t)
	nsID := containerID("ns-a")
	schedBpfNodes := nodesOfType(sr, "sched-bpf", nsID)
	if len(schedBpfNodes) == 0 {
		t.Error("expected sched-bpf node (TC BPF without clsact)")
	}
	// No traffic-control egress node for veth0
	tcNodes := nodesOfType(sr, "traffic-control", nsID)
	for _, n := range tcNodes {
		if strings.Contains(n.ID, "veth0") && n.Data.Config["direction"] == "egress" {
			t.Errorf("unexpected traffic-control egress node for veth0: %s", n.ID)
		}
	}
}

func Test_BPF05_XDPAndTCBPFOnSameInterface(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if err := c.AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp"); err != nil {
		t.Fatalf("AttachXDP: %v", err)
	}
	if err := c.AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc"); err != nil {
		t.Fatalf("AttachTCBPF: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.BpfAttach) != 2 {
		t.Fatalf("expected 2 BpfAttach entries, got %d", len(s.BpfAttach))
	}

	nsID := containerID("ns-a")
	xdpNodes := nodesOfType(sr, "xdp-program", nsID)
	tcBpfNodes := nodesOfType(sr, "tc-bpf-program", nsID)
	if len(xdpNodes) == 0 {
		t.Error("expected xdp-program node")
	}
	if len(tcBpfNodes) == 0 {
		t.Error("expected tc-bpf-program node")
	}

	// Ingress chain: veth0 → xdp-program → tc-bpf-program → traffic-control → ...
	xdpID := xdpNodes[0].ID
	tcBpfID := tcBpfNodes[0].ID
	tcIngressID := "ns-a-veth0-tc-ingress"
	if !edgeExists(sr, "ns-a-veth0", xdpID) {
		t.Errorf("expected edge ns-a-veth0 → %s", xdpID)
	}
	if !edgeExists(sr, xdpID, tcBpfID) {
		t.Errorf("expected edge %s → %s", xdpID, tcBpfID)
	}
	if !edgeExists(sr, tcBpfID, tcIngressID) {
		t.Errorf("expected edge %s → %s", tcBpfID, tcIngressID)
	}
}

// ============================================================
// Section 9: Route Detection
// ============================================================

func Test_RT01_NoRoutesNoRoutingTableNode(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if s.HasRoutes {
		t.Error("HasRoutes should be false when no routes are configured")
	}
	if nodeExists(sr, "ns-a-routing") {
		t.Error("routing-table node should not be emitted when HasRoutes=false")
	}
}

func Test_RT02_WithRouteHasRoutingTableNode(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "dummy0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "dummy0"); err != nil {
		t.Fatalf("addr add: %v", err)
	}
	if err := c.IP("ns-a", "route", "add", "10.1.0.0/24", "via", "10.0.0.2"); err != nil {
		t.Fatalf("route add: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if !s.HasRoutes {
		t.Error("HasRoutes should be true")
	}
	if !nodeExists(sr, "ns-a-routing") {
		t.Error("routing-table node should be emitted when HasRoutes=true")
	}
}

// ============================================================
// Section 10: Socket Detection
// ============================================================

func Test_SOCK01_NoSocketsInNamespace(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Sockets) != 0 {
		t.Errorf("expected 0 sockets, got %d: %v", len(s.Sockets), s.Sockets)
	}
}

func Test_SOCK02_TCPListeningSocket(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "dummy0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "127.0.0.1/8", "dev", "dummy0"); err != nil {
		t.Fatalf("addr add: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9000}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Sockets) != 1 {
		t.Fatalf("expected 1 socket entry, got %d: %v", len(s.Sockets), s.Sockets)
	}
	if s.Sockets[0].PID <= 0 {
		t.Errorf("PID = %d, want > 0", s.Sockets[0].PID)
	}

	nsID := containerID("ns-a")
	socketNodes := nodesOfType(sr, "socket", nsID)
	if len(socketNodes) == 0 {
		t.Fatal("expected socket node")
	}
	if socketNodes[0].Data.Label != "testdaemon" {
		t.Errorf("socket label = %q, want \"testdaemon\"", socketNodes[0].Data.Label)
	}
}

func Test_SOCK03_UDPBoundSocket(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "dummy0", "type", "dummy"); err != nil {
		t.Fatalf("add dummy: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "dummy0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "udp-bind", Port: 9001}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Sockets) != 1 {
		t.Errorf("expected 1 socket entry, got %d: %v", len(s.Sockets), s.Sockets)
	}
}

func Test_SOCK04_TwoProcessesTwoSocketNodes(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	pid1, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9000})
	if err != nil {
		t.Fatalf("StartProcess 1: %v", err)
	}
	pid2, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9001})
	if err != nil {
		t.Fatalf("StartProcess 2: %v", err)
	}
	if pid1 == pid2 {
		t.Errorf("expected distinct PIDs, both are %d", pid1)
	}

	sr := doScan(t)
	s := snap(t, sr, "ns-a")
	if len(s.Sockets) != 2 {
		t.Errorf("expected 2 socket entries, got %d: %v", len(s.Sockets), s.Sockets)
	}
	pids := make(map[int]bool)
	for _, sock := range s.Sockets {
		pids[sock.PID] = true
	}
	if len(pids) != 2 {
		t.Error("expected 2 distinct PIDs in socket entries")
	}

	nsID := containerID("ns-a")
	socketNodes := nodesOfType(sr, "socket", nsID)
	if len(socketNodes) != 2 {
		t.Errorf("expected 2 socket nodes, got %d", len(socketNodes))
	}
}

func Test_SOCK05_NoSocketsFlagSuppressesCollection(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9000}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScanNoSockets(t)
	s := snap(t, sr, "ns-a")
	if len(s.Sockets) != 0 {
		t.Errorf("expected 0 sockets with noSockets=true, got %d", len(s.Sockets))
	}

	nsID := containerID("ns-a")
	socketNodes := nodesOfType(sr, "socket", nsID)
	if len(socketNodes) != 0 {
		t.Errorf("expected 0 socket nodes with noSockets=true, got %d", len(socketNodes))
	}
}

func Test_SOCK06_TCPConnectionBetweenNamespaces(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	for _, cmd := range [][]string{
		{"ns-a", "link", "set", "veth0", "up"},
		{"ns-b", "link", "set", "veth1", "up"},
	} {
		if err := c.IP(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("IP %v: %v", cmd, err)
		}
	}
	if err := c.IP("ns-a", "addr", "add", "192.168.1.1/24", "dev", "veth0"); err != nil {
		t.Fatalf("addr ns-a: %v", err)
	}
	if err := c.IP("ns-b", "addr", "add", "192.168.1.2/24", "dev", "veth1"); err != nil {
		t.Fatalf("addr ns-b: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 8080}); err != nil {
		t.Fatalf("StartProcess listener: %v", err)
	}
	if _, err := c.StartProcess("ns-b", testdaemon.ProcessRequest{Type: "tcp-connect", Addr: "192.168.1.1", Port: 8080}); err != nil {
		t.Fatalf("StartProcess connector: %v", err)
	}

	sr := doScan(t)
	sA := snap(t, sr, "ns-a")
	sB := snap(t, sr, "ns-b")
	if len(sA.Sockets) == 0 {
		t.Error("expected at least 1 socket in ns-a")
	}
	if len(sB.Sockets) == 0 {
		t.Error("expected at least 1 socket in ns-b")
	}

	if len(nodesOfType(sr, "socket", containerID("ns-a"))) == 0 {
		t.Error("expected socket node in ns-a")
	}
	if len(nodesOfType(sr, "socket", containerID("ns-b"))) == 0 {
		t.Error("expected socket node in ns-b")
	}
}

// ============================================================
// Section 11: Edge Chain Correctness
// ============================================================

func Test_EDGE01_IngressChainNoNftablesNoTC(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0"); err != nil {
		t.Fatalf("addr add: %v", err)
	}

	sr := doScan(t)
	if !edgeExists(sr, "ns-a-veth0", "ns-a-routing") {
		t.Error("expected edge ns-a-veth0 → ns-a-routing")
	}
	// No nftables-prerouting node configured → no edge to it
	if nodeExists(sr, "ns-a-nft-prerouting") {
		t.Error("nftables-prerouting node should not exist when no nft rules configured")
	}
}

func Test_EDGE02_FullIngressChainWithNftablesAndRouting(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0"); err != nil {
		t.Fatalf("addr add: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter {
  chain pre { type filter hook prerouting priority 0; }
  chain in  { type filter hook input      priority 0; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 8080}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)

	// Edge sequence: veth0 → nft-prerouting → routing → nft-input → socket
	chain := []struct{ src, tgt string }{
		{"ns-a-veth0", "ns-a-nft-prerouting"},
		{"ns-a-nft-prerouting", "ns-a-routing"},
		{"ns-a-routing", "ns-a-nft-input"},
	}
	for _, step := range chain {
		e := edgeOf(t, sr, step.src, step.tgt)
		if !strings.HasSuffix(e.SourceHandle, "-s") {
			t.Errorf("edge %s→%s SourceHandle=%q does not end in -s", step.src, step.tgt, e.SourceHandle)
		}
		if !strings.HasSuffix(e.TargetHandle, "-t") {
			t.Errorf("edge %s→%s TargetHandle=%q does not end in -t", step.src, step.tgt, e.TargetHandle)
		}
	}
	// nft-input → socket edge
	sockNode := findSocketNode(sr, containerID("ns-a"))
	if sockNode == nil {
		t.Fatal("no socket node found in ns-a")
	}
	if !edgeExists(sr, "ns-a-nft-input", sockNode.ID) {
		t.Errorf("expected edge ns-a-nft-input → %s", sockNode.ID)
	}
}

func Test_EDGE03_EgressChainWithTCAndQdisc(t *testing.T) {
	c := requireDaemon(t)
	if err := c.CreateNs("ns-a"); err != nil {
		t.Fatalf("CreateNs: %v", err)
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "root", "fq"); err != nil {
		t.Fatalf("add fq: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter {
  chain out  { type filter hook output      priority 0; }
  chain post { type filter hook postrouting priority 100; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 8080}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)

	sockNode := findSocketNode(sr, containerID("ns-a"))
	if sockNode == nil {
		t.Fatal("no socket node found in ns-a")
	}

	// Egress chain: socket → nft-output → nft-postrouting → tc-egress → qdisc → veth0
	chain := []struct{ src, tgt string }{
		{sockNode.ID, "ns-a-nft-output"},
		{"ns-a-nft-output", "ns-a-nft-postrouting"},
		{"ns-a-nft-postrouting", "ns-a-veth0-tc-egress"},
		{"ns-a-veth0-tc-egress", "ns-a-veth0-qdisc"},
		{"ns-a-veth0-qdisc", "ns-a-veth0"},
	}
	for _, step := range chain {
		if !edgeExists(sr, step.src, step.tgt) {
			t.Errorf("expected egress edge %s → %s", step.src, step.tgt)
		}
	}
}

func Test_EDGE04_PairLinkEdgeAttributes(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}

	sr := doScan(t)
	vle := vethLinkEdges(sr)
	if len(vle) != 1 {
		t.Fatalf("expected exactly 1 veth-link edge, got %d", len(vle))
	}
	e := vle[0]
	if e.ClassName != "veth-link" {
		t.Errorf("ClassName = %q, want \"veth-link\"", e.ClassName)
	}
	if e.SourceHandle != "S-0-s" {
		t.Errorf("SourceHandle = %q, want \"S-0-s\"", e.SourceHandle)
	}
	if e.TargetHandle != "S-0-t" {
		t.Errorf("TargetHandle = %q, want \"S-0-t\"", e.TargetHandle)
	}
}

// ============================================================
// Section 12: Complex / Combined Scenarios
// ============================================================

func Test_COMPLEX01_TwoNamespaceRoutingTopology(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	// ns-a ↔ host
	for _, cmd := range [][]string{
		{"linux-host", "link", "add", "veth-ha", "type", "veth", "peer", "name", "veth-ah"},
		{"linux-host", "link", "set", "veth-ah", "netns", "ns-a"},
		{"linux-host", "link", "set", "veth-ha", "up"},
		{"ns-a", "link", "set", "veth-ah", "up"},
		{"linux-host", "addr", "add", "10.0.1.1/24", "dev", "veth-ha"},
		{"ns-a", "addr", "add", "10.0.1.2/24", "dev", "veth-ah"},
	} {
		if err := c.IP(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("IP %v: %v", cmd, err)
		}
	}
	// ns-b ↔ host
	for _, cmd := range [][]string{
		{"linux-host", "link", "add", "veth-hb", "type", "veth", "peer", "name", "veth-bh"},
		{"linux-host", "link", "set", "veth-bh", "netns", "ns-b"},
		{"linux-host", "link", "set", "veth-hb", "up"},
		{"ns-b", "link", "set", "veth-bh", "up"},
		{"linux-host", "addr", "add", "10.0.2.1/24", "dev", "veth-hb"},
		{"ns-b", "addr", "add", "10.0.2.2/24", "dev", "veth-bh"},
	} {
		if err := c.IP(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("IP %v: %v", cmd, err)
		}
	}
	// routes
	if err := c.IP("ns-a", "route", "add", "default", "via", "10.0.1.1"); err != nil {
		t.Fatalf("route ns-a: %v", err)
	}
	if err := c.IP("ns-b", "route", "add", "default", "via", "10.0.2.1"); err != nil {
		t.Fatalf("route ns-b: %v", err)
	}

	sr := doScan(t)

	// Pair detection
	pHA := pairOf(t, sr, "host", "veth-ha")
	pAH := pairOf(t, sr, "ns-a", "veth-ah")
	if pHA.PairID != pAH.PairID {
		t.Errorf("host/veth-ha and ns-a/veth-ah should share PairID")
	}
	pHB := pairOf(t, sr, "host", "veth-hb")
	pBH := pairOf(t, sr, "ns-b", "veth-bh")
	if pHB.PairID != pBH.PairID {
		t.Errorf("host/veth-hb and ns-b/veth-bh should share PairID")
	}

	// All 3 namespaces have routes
	for _, nsName := range []string{"host", "ns-a", "ns-b"} {
		if !snap(t, sr, nsName).HasRoutes {
			t.Errorf("namespace %q should have HasRoutes=true", nsName)
		}
	}

	// 3 container nodes
	var containerCount int
	for _, n := range sr.Result.Nodes {
		if n.Type == "containerNode" {
			containerCount++
		}
	}
	if containerCount < 3 {
		t.Errorf("expected at least 3 container nodes (host, ns-a, ns-b), got %d", containerCount)
	}

	// 2 pair-link edges
	vle := vethLinkEdges(sr)
	if len(vle) != 2 {
		t.Errorf("expected 2 pair-link edges, got %d", len(vle))
	}

	// Layout: distinct x positions
	layout.Layout(&sr.Result)
	xPositions := make(map[float64]string)
	for _, n := range sr.Result.Nodes {
		if n.Type == "containerNode" {
			if prev, dup := xPositions[n.Position.X]; dup {
				t.Errorf("containers %q and %q have the same x position %v", prev, n.ID, n.Position.X)
			}
			xPositions[n.Position.X] = n.ID
		}
	}
}

func Test_COMPLEX02_FullStack(t *testing.T) {
	c := requireDaemon(t)
	requireBpfStubs(t, c)
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	// veth pair
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	for _, cmd := range [][]string{
		{"ns-a", "link", "set", "veth0", "up"},
		{"ns-b", "link", "set", "veth1", "up"},
		{"ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0"},
		{"ns-b", "addr", "add", "10.0.0.2/24", "dev", "veth1"},
	} {
		if err := c.IP(cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("IP %v: %v", cmd, err)
		}
	}
	// nftables in ns-a
	if err := c.NftScript("ns-a", `
table inet filter {
  chain pre  { type filter hook prerouting  priority 0; }
  chain in   { type filter hook input       priority 0; }
  chain out  { type filter hook output      priority 0; }
  chain post { type filter hook postrouting priority 100; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}
	// TC + BPF
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if err := c.AttachXDP("ns-a", "veth0", "xdp_pass.o", "xdp"); err != nil {
		t.Fatalf("AttachXDP: %v", err)
	}
	if err := c.AttachTCBPF("ns-a", "veth0", "ingress", "tc_pass.o", "tc"); err != nil {
		t.Fatalf("AttachTCBPF: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 8080}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)
	sA := snap(t, sr, "ns-a")
	sB := snap(t, sr, "ns-b")

	if len(sA.Interfaces) != 1 {
		t.Errorf("ns-a: expected 1 interface, got %d", len(sA.Interfaces))
	}
	if len(sB.Interfaces) != 1 {
		t.Errorf("ns-b: expected 1 interface, got %d", len(sB.Interfaces))
	}
	if len(sA.BpfAttach) != 2 {
		t.Errorf("ns-a: expected 2 BpfAttach entries, got %d", len(sA.BpfAttach))
	}
	if len(sA.NftHooks) != 4 {
		t.Errorf("ns-a: expected 4 NftHooks, got %d", len(sA.NftHooks))
	}
	if !sA.HasRoutes {
		t.Error("ns-a: expected HasRoutes=true")
	}
	if len(sA.Sockets) != 1 {
		t.Errorf("ns-a: expected 1 socket, got %d", len(sA.Sockets))
	}

	// Node types in ns-a
	nsAID := containerID("ns-a")
	for _, wantType := range []string{
		"veth-end", "xdp-program", "tc-bpf-program", "traffic-control",
		"nftables-prerouting", "nftables-input", "nftables-output", "nftables-postrouting",
		"routing-table", "socket",
	} {
		if len(nodesOfType(sr, wantType, nsAID)) == 0 {
			t.Errorf("ns-a: expected at least one %q node", wantType)
		}
	}
	// Verify both ingress and egress traffic-control nodes exist
	tcNodes := nodesOfType(sr, "traffic-control", nsAID)
	var hasTC bool
	for _, n := range tcNodes {
		if n.Data.Config["direction"] == "ingress" || n.Data.Config["direction"] == "egress" {
			hasTC = true
		}
	}
	if !hasTC || len(tcNodes) < 2 {
		t.Errorf("expected ingress and egress traffic-control nodes, got %d TC nodes", len(tcNodes))
	}

	// ns-b has veth-end
	nsBID := containerID("ns-b")
	if len(nodesOfType(sr, "veth-end", nsBID)) == 0 {
		t.Error("ns-b: expected veth-end node")
	}

	// 1 pair-link edge
	vle := vethLinkEdges(sr)
	if len(vle) != 1 {
		t.Errorf("expected 1 pair-link edge, got %d", len(vle))
	}

	// Ingress chain
	xdpNodes := nodesOfType(sr, "xdp-program", nsAID)
	tcBpfNodes := nodesOfType(sr, "tc-bpf-program", nsAID)
	if len(xdpNodes) == 0 || len(tcBpfNodes) == 0 {
		t.Fatal("missing xdp or tc-bpf node for ingress chain check")
	}
	xdpID := xdpNodes[0].ID
	tcBpfID := tcBpfNodes[0].ID
	ingressChain := []struct{ src, tgt string }{
		{"ns-a-veth0", xdpID},
		{xdpID, tcBpfID},
		{tcBpfID, "ns-a-veth0-tc-ingress"},
		{"ns-a-veth0-tc-ingress", "ns-a-nft-prerouting"},
		{"ns-a-nft-prerouting", "ns-a-routing"},
		{"ns-a-routing", "ns-a-nft-input"},
	}
	for _, step := range ingressChain {
		if !edgeExists(sr, step.src, step.tgt) {
			t.Errorf("missing ingress edge: %s → %s", step.src, step.tgt)
		}
	}

	// Egress chain
	sockNode := findSocketNode(sr, nsAID)
	if sockNode == nil {
		t.Fatal("no socket node in ns-a")
	}
	egressChain := []struct{ src, tgt string }{
		{sockNode.ID, "ns-a-nft-output"},
		{"ns-a-nft-output", "ns-a-nft-postrouting"},
		{"ns-a-nft-postrouting", "ns-a-veth0-tc-egress"},
		{"ns-a-veth0-tc-egress", "ns-a-veth0"},
	}
	for _, step := range egressChain {
		if !edgeExists(sr, step.src, step.tgt) {
			t.Errorf("missing egress edge: %s → %s", step.src, step.tgt)
		}
	}

	// JSON validates against topology spec
	validateTopologySpec(t, sr)
}

func Test_COMPLEX03_LargeTopologyFiveNamespaces(t *testing.T) {
	c := requireDaemon(t)
	for _, ns := range []string{"ns-a", "ns-b", "ns-c", "ns-d", "ns-e"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	// ns-a ↔ ns-b via veth
	if err := c.IP("ns-a", "link", "add", "veth-ab", "type", "veth", "peer", "name", "veth-ba"); err != nil {
		t.Fatalf("add ab veth: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth-ba", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth-ba: %v", err)
	}
	// ns-b ↔ ns-c via netkit
	mustIP(t, c, "ns-b", "link", "add", "nk-bc", "type", "netkit", "mode", "l3", "peer", "name", "nk-cb")
	if err := c.IP("ns-b", "link", "set", "nk-cb", "netns", "ns-c"); err != nil {
		t.Fatalf("move nk-cb: %v", err)
	}
	// ns-c ↔ ns-d via veth
	if err := c.IP("ns-c", "link", "add", "veth-cd", "type", "veth", "peer", "name", "veth-dc"); err != nil {
		t.Fatalf("add cd veth: %v", err)
	}
	if err := c.IP("ns-c", "link", "set", "veth-dc", "netns", "ns-d"); err != nil {
		t.Fatalf("move veth-dc: %v", err)
	}
	// ns-d ↔ ns-e via netkit
	mustIP(t, c, "ns-d", "link", "add", "nk-de", "type", "netkit", "mode", "l3", "peer", "name", "nk-ed")
	if err := c.IP("ns-d", "link", "set", "nk-ed", "netns", "ns-e"); err != nil {
		t.Fatalf("move nk-ed: %v", err)
	}
	// nftables input hook in each namespace
	for _, ns := range []string{"ns-a", "ns-b", "ns-c", "ns-d", "ns-e"} {
		if err := c.NftScript(ns, `table inet filter { chain in { type filter hook input priority 0; } }`); err != nil {
			t.Fatalf("NftScript(%s): %v", ns, err)
		}
	}
	// Sockets in ns-a and ns-e
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9000}); err != nil {
		t.Fatalf("StartProcess ns-a: %v", err)
	}
	if _, err := c.StartProcess("ns-e", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 9001}); err != nil {
		t.Fatalf("StartProcess ns-e: %v", err)
	}

	sr := doScan(t)

	// 4 pairs: ab(veth), bc(netkit), cd(veth), de(netkit)
	if len(sr.Pairs) != 8 {
		t.Errorf("expected 8 pair entries (4 pairs × 2 ends), got %d", len(sr.Pairs))
	}

	// Correct node types
	checks := []struct {
		ns       string
		ifc      string
		wantType string
	}{
		{"ns-a", "veth-ab", "veth-end"},
		{"ns-b", "veth-ba", "veth-end"},
		{"ns-b", "nk-bc", "netkit-primary"},
		{"ns-c", "nk-cb", "netkit-peer"},
		{"ns-c", "veth-cd", "veth-end"},
		{"ns-d", "veth-dc", "veth-end"},
		{"ns-d", "nk-de", "netkit-primary"},
		{"ns-e", "nk-ed", "netkit-peer"},
	}
	for _, ch := range checks {
		nodeID := ch.ns + "-" + ch.ifc
		n := nodeOf(t, sr, nodeID)
		if n.Data.NodeType != ch.wantType {
			t.Errorf("%s NodeType = %q, want %q", nodeID, n.Data.NodeType, ch.wantType)
		}
	}

	// 5 container nodes
	var containerCount int
	for _, n := range sr.Result.Nodes {
		if n.Type == "containerNode" {
			containerCount++
		}
	}
	if containerCount < 5 {
		t.Errorf("expected at least 5 container nodes, got %d", containerCount)
	}

	// Layout: distinct x positions, children within parent bounds
	layout.Layout(&sr.Result)
	xPositions := make(map[float64]string)
	containerPos := make(map[string]topology.Node)
	for _, n := range sr.Result.Nodes {
		if n.Type == "containerNode" {
			if prev, dup := xPositions[n.Position.X]; dup {
				t.Errorf("containers %q and %q share x=%v", prev, n.ID, n.Position.X)
			}
			xPositions[n.Position.X] = n.ID
			containerPos[n.ID] = n
		}
	}
	for _, n := range sr.Result.Nodes {
		if n.Type == "netNode" && n.ParentNode != "" {
			parent, ok := containerPos[n.ParentNode]
			if !ok {
				continue
			}
			if parent.Style == nil {
				continue
			}
			if n.Position.X < 0 || n.Position.X > parent.Style.Width {
				t.Errorf("node %s x=%v is outside parent %s width=%v", n.ID, n.Position.X, parent.ID, parent.Style.Width)
			}
			if n.Position.Y < 0 || n.Position.Y > parent.Style.Height {
				t.Errorf("node %s y=%v is outside parent %s height=%v", n.ID, n.Position.Y, parent.ID, parent.Style.Height)
			}
		}
	}
}

func Test_COMPLEX04_ScanOutputRoundTripsTopologySpec(t *testing.T) {
	c := requireDaemon(t)
	// Build a topology with at least one of each interesting node type
	for _, ns := range []string{"ns-a", "ns-b"} {
		if err := c.CreateNs(ns); err != nil {
			t.Fatalf("CreateNs(%s): %v", ns, err)
		}
	}
	if err := c.IP("ns-a", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1"); err != nil {
		t.Fatalf("add veth pair: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth1", "netns", "ns-b"); err != nil {
		t.Fatalf("move veth1: %v", err)
	}
	if err := c.IP("ns-a", "link", "set", "veth0", "up"); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := c.IP("ns-a", "addr", "add", "10.0.0.1/24", "dev", "veth0"); err != nil {
		t.Fatalf("addr add: %v", err)
	}
	if err := c.NftScript("ns-a", `
table inet filter {
  chain in  { type filter hook input priority 0; }
  chain out { type filter hook output priority 0; }
}`); err != nil {
		t.Fatalf("NftScript: %v", err)
	}
	if err := c.TC("ns-a", "qdisc", "add", "dev", "veth0", "clsact"); err != nil {
		t.Fatalf("add clsact: %v", err)
	}
	if _, err := c.StartProcess("ns-a", testdaemon.ProcessRequest{Type: "tcp-listen", Port: 8080}); err != nil {
		t.Fatalf("StartProcess: %v", err)
	}

	sr := doScan(t)
	layout.Layout(&sr.Result)
	validateTopologySpec(t, sr)
}

// validateTopologySpec checks that the builder output satisfies the structural
// constraints described in doc/topology-spec.json.
func validateTopologySpec(t *testing.T, sr scanResult) {
	t.Helper()

	topo := topology.Topology{Nodes: sr.Result.Nodes, Edges: sr.Result.Edges}
	data, err := json.Marshal(topo)
	if err != nil {
		t.Fatalf("json.Marshal topology: %v", err)
	}
	var parsed topology.Topology
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON round-trip failed: %v", err)
	}

	validNetNodeTypes := map[string]bool{
		"interface": true, "veth-end": true, "netkit-primary": true, "netkit-peer": true,
		"socket": true, "network": true, "routing-table": true,
		"nftables-prerouting": true, "nftables-input": true, "nftables-forward": true,
		"nftables-output": true, "nftables-postrouting": true,
		"traffic-control": true, "qdisc": true,
		"xdp-program": true, "tc-bpf-program": true, "sched-bpf": true,
		"netkit-bpf-ingress": true, "netkit-bpf-egress": true,
	}

	// containerNode children appear after their parent
	containerIdx := make(map[string]int)
	for i, n := range parsed.Nodes {
		if n.Type == "containerNode" {
			containerIdx[n.ID] = i
		}
	}
	for i, n := range parsed.Nodes {
		if n.Type == "netNode" && n.ParentNode != "" {
			parentI, ok := containerIdx[n.ParentNode]
			if !ok {
				t.Errorf("node %q has parentNode %q which is not a containerNode", n.ID, n.ParentNode)
			} else if i < parentI {
				t.Errorf("child %q (idx %d) appears before parent %q (idx %d)", n.ID, i, n.ParentNode, parentI)
			}
		}
	}

	// netNode.data.nodeType must be in enum
	for _, n := range parsed.Nodes {
		if n.Type == "netNode" && !validNetNodeTypes[n.Data.NodeType] {
			t.Errorf("node %q has invalid nodeType %q", n.ID, n.Data.NodeType)
		}
	}

	// Edge handle conventions
	for _, e := range parsed.Edges {
		if !strings.HasSuffix(e.SourceHandle, "-s") {
			t.Errorf("edge %q SourceHandle=%q does not end in -s", e.ID, e.SourceHandle)
		}
		if !strings.HasSuffix(e.TargetHandle, "-t") {
			t.Errorf("edge %q TargetHandle=%q does not end in -t", e.ID, e.TargetHandle)
		}
		if e.Data != nil && e.Data.VethLink {
			if e.ClassName != "veth-link" {
				t.Errorf("pairLinkEdge %q has className=%q, want \"veth-link\"", e.ID, e.ClassName)
			}
		}
	}

	// Verify spec file is reachable (informational)
	_, thisFile, _, ok := runtime.Caller(1)
	if ok {
		specPath := filepath.Join(filepath.Dir(thisFile), "..", "doc", "topology-spec.json")
		if _, err := os.Stat(specPath); err != nil {
			t.Logf("topology-spec.json not found at %s (non-fatal)", specPath)
		}
	}
}
