package collector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func EnumerateNamespaces() ([]NsInfo, error) {
	seen := make(map[uint64]bool)
	var result []NsInfo

	add := func(ns NsInfo) {
		if seen[ns.Inode] {
			return
		}
		seen[ns.Inode] = true
		result = append(result, ns)
	}

	// Host namespace
	if inode, err := statInode("/proc/1/ns/net"); err == nil {
		add(NsInfo{Name: "host", Path: "/proc/1/ns/net", Inode: inode})
	}

	// Named namespaces via ip netns list
	if out, err := exec.Command("ip", "netns", "list").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name := strings.Fields(line)[0]
			path := "/var/run/netns/" + name
			if inode, err := statInode(path); err == nil {
				add(NsInfo{Name: name, Path: path, Inode: inode})
			}
		}
	}

	// Process namespaces
	entries, _ := filepath.Glob("/proc/[0-9]*/ns/net")
	for _, entry := range entries {
		inode, err := statInode(entry)
		if err != nil || seen[inode] {
			continue
		}
		parts := strings.Split(entry, "/")
		pid, _ := strconv.Atoi(parts[2])
		comm := readComm(pid)
		name := fmt.Sprintf("ns-%s-%d", comm, pid)
		add(NsInfo{Name: name, Path: entry, Inode: inode, PID: pid})
	}

	return result, nil
}

func statInode(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return st.Ino, nil
}

func readComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "proc"
	}
	return strings.TrimSpace(string(data))
}
