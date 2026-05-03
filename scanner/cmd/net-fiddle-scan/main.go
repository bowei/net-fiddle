package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"net-fiddle/scanner/internal/builder"
	"net-fiddle/scanner/internal/collector"
	"net-fiddle/scanner/internal/layout"
	"net-fiddle/scanner/internal/linker"
	"net-fiddle/scanner/internal/topology"
)

const version = "0.1.0"

func main() {
	var (
		outputFile  = flag.String("output", "", "Write JSON to FILE instead of stdout")
		onlyNs      = flag.String("namespace", "", "Comma-separated list of namespaces to scan (default: all)")
		excludeNs   = flag.String("exclude-ns", "", "Comma-separated list of namespaces to skip")
		noSockets   = flag.Bool("no-sockets", false, "Omit socket nodes")
		pretty      = flag.Bool("pretty", false, "Pretty-print JSON output")
		verbose     = flag.Bool("verbose", false, "Print collection summary to stderr")
		showVersion = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("net-fiddle-scan", version)
		return
	}

	if os.Geteuid() != 0 {
		log.Fatal("net-fiddle-scan must run as root")
	}

	// Build allow/deny sets
	onlySet := splitSet(*onlyNs)
	excludeSet := splitSet(*excludeNs)

	// Enumerate namespaces
	namespaces, err := collector.EnumerateNamespaces()
	if err != nil {
		log.Fatalf("namespace enumeration failed: %v", err)
	}

	// Filter
	var filtered []collector.NsInfo
	for _, ns := range namespaces {
		if len(onlySet) > 0 && !onlySet[ns.Name] {
			continue
		}
		if excludeSet[ns.Name] {
			continue
		}
		filtered = append(filtered, ns)
	}

	if *verbose {
		log.Printf("Scanning %d namespace(s)", len(filtered))
	}

	// Collect
	snapshots := make([]collector.NsSnapshot, 0, len(filtered))
	for _, ns := range filtered {
		snap := collector.CollectAll(ns, *noSockets)
		if *verbose {
			log.Printf("  %s: %d ifaces, %d bpf, %d nft hooks, %d qdiscs, %d sockets",
				ns.Name,
				len(snap.Interfaces),
				len(snap.BpfAttach),
				len(snap.NftHooks),
				len(snap.Qdiscs),
				len(snap.Sockets),
			)
		}
		snapshots = append(snapshots, snap)
	}

	// Link pairs
	pairMap := linker.Link(snapshots)

	// Build topology
	result := builder.Build(snapshots, pairMap)

	// Layout
	layout.Layout(&result)

	// Emit
	topo := topology.Topology{
		Nodes: result.Nodes,
		Edges: result.Edges,
	}

	var out []byte
	if *pretty {
		out, err = json.MarshalIndent(topo, "", "  ")
	} else {
		out, err = json.Marshal(topo)
	}
	if err != nil {
		log.Fatalf("JSON marshal failed: %v", err)
	}
	out = append(out, '\n')

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, out, 0644); err != nil {
			log.Fatalf("write %s: %v", *outputFile, err)
		}
		if *verbose {
			log.Printf("Wrote %s (%d bytes)", *outputFile, len(out))
		}
	} else {
		os.Stdout.Write(out)
	}
}

func splitSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			m[v] = true
		}
	}
	return m
}
