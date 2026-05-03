package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Subprocess mode: re-exec'd inside a target namespace to hold sockets.
	if len(os.Args) > 1 && os.Args[1] == "--sockhold" {
		runSockholdMain()
		return
	}

	listenAddr := flag.String("listen", ":7777", "Listen address")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("testdaemon must run as root")
	}

	d := newDaemon()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.handleHealthz)
	mux.HandleFunc("POST /reset", d.handleReset)
	mux.HandleFunc("GET /ns", d.handleListNs)
	mux.HandleFunc("POST /ns/{name}", d.handleCreateNs)
	mux.HandleFunc("DELETE /ns/{name}", d.handleDeleteNs)
	mux.HandleFunc("POST /ns/{name}/ip", d.handleIP)
	mux.HandleFunc("POST /ns/{name}/tc", d.handleTC)
	mux.HandleFunc("POST /ns/{name}/nft", d.handleNft)
	mux.HandleFunc("GET /bpf/objects", d.handleListBpf)
	mux.HandleFunc("GET /bpf/objects/{obj}", d.handleGetBpf)
	mux.HandleFunc("POST /ns/{name}/bpf/xdp", d.handleAttachXDP)
	mux.HandleFunc("POST /ns/{name}/bpf/tc", d.handleAttachTCBPF)
	mux.HandleFunc("GET /ns/{name}/process", d.handleListProcs)
	mux.HandleFunc("POST /ns/{name}/process", d.handleStartProc)
	mux.HandleFunc("DELETE /ns/{name}/process/{pid}", d.handleKillProc)

	log.Printf("testdaemon listening on %s", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

// ---- daemon state -------------------------------------------------------

type procEntry struct {
	cmd *exec.Cmd
	ns  string
}

type daemon struct {
	mu        sync.Mutex
	managedNs map[string]bool   // namespaces created via POST /ns
	procs     map[int]*procEntry // pid → entry for managed sockhold processes
	bpfObjs   map[string]string  // object name → absolute path to ELF file
	bpfDir    string
}

func newDaemon() *daemon {
	d := &daemon{
		managedNs: make(map[string]bool),
		procs:     make(map[int]*procEntry),
		bpfObjs:   make(map[string]string),
	}
	dir, err := os.MkdirTemp("", "testdaemon-bpf-*")
	if err == nil {
		d.bpfDir = dir
		buildBPFStubs(dir, d.bpfObjs)
	} else {
		log.Printf("warning: could not create BPF tmpdir: %v", err)
	}
	return d
}

// buildCmd constructs an exec.Cmd that runs prog+args inside the given
// namespace. "linux-host" means run directly without nsenter.
func (d *daemon) buildCmd(ns, prog string, args ...string) *exec.Cmd {
	if ns == "linux-host" {
		return exec.Command(prog, args...)
	}
	nsPath := "/var/run/netns/" + ns
	all := append([]string{"--net=" + nsPath, "--preserve-credentials", "--", prog}, args...)
	return exec.Command("nsenter", all...)
}

func (d *daemon) runInNs(ns, prog string, args ...string) ([]byte, error) {
	return d.buildCmd(ns, prog, args...).CombinedOutput()
}

// ---- helpers ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func statInode(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return st.Ino, nil
}

// ---- lifecycle handlers -------------------------------------------------

func (d *daemon) handleHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

func (d *daemon) handleReset(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	for _, e := range d.procs {
		e.cmd.Process.Kill()
	}
	managed := make([]string, 0, len(d.managedNs))
	for name := range d.managedNs {
		managed = append(managed, name)
	}
	d.managedNs = make(map[string]bool)
	d.procs = make(map[int]*procEntry)
	d.mu.Unlock()

	time.Sleep(150 * time.Millisecond)

	for _, name := range managed {
		if out, err := exec.Command("ip", "netns", "del", name).CombinedOutput(); err != nil {
			log.Printf("reset: ip netns del %s: %v: %s", name, err, out)
		}
	}
	w.WriteHeader(http.StatusOK)
}

// ---- namespace handlers -------------------------------------------------

func (d *daemon) handleListNs(w http.ResponseWriter, r *http.Request) {
	type nsItem struct {
		Name  string `json:"name"`
		Inode uint64 `json:"inode"`
	}
	var result []nsItem
	if inode, err := statInode("/proc/1/ns/net"); err == nil {
		result = append(result, nsItem{Name: "linux-host", Inode: inode})
	}
	if out, err := exec.Command("ip", "netns", "list").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name := strings.Fields(line)[0]
			path := "/var/run/netns/" + name
			if inode, err := statInode(path); err == nil {
				result = append(result, nsItem{Name: name, Inode: inode})
			}
		}
	}
	writeJSON(w, result)
}

func (d *daemon) handleCreateNs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "linux-host" {
		writeError(w, http.StatusBadRequest, "linux-host is reserved")
		return
	}
	if out, err := exec.Command("ip", "netns", "add", name).CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	d.mu.Lock()
	d.managedNs[name] = true
	d.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

func (d *daemon) handleDeleteNs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "linux-host" {
		writeError(w, http.StatusBadRequest, "linux-host cannot be deleted")
		return
	}
	d.mu.Lock()
	for pid, e := range d.procs {
		if e.ns == name {
			e.cmd.Process.Kill()
			delete(d.procs, pid)
		}
	}
	delete(d.managedNs, name)
	d.mu.Unlock()

	time.Sleep(50 * time.Millisecond)

	if out, err := exec.Command("ip", "netns", "del", name).CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ---- command handlers ---------------------------------------------------

func (d *daemon) handleIP(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var body struct {
		Cmdline []string `json:"cmdline"`
	}
	if err := decodeBody(r, &body); err != nil || len(body.Cmdline) == 0 {
		writeError(w, http.StatusBadRequest, "cmdline required")
		return
	}
	out, err := d.runInNs(ns, "ip", body.Cmdline...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *daemon) handleTC(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var body struct {
		Cmdline []string `json:"cmdline"`
	}
	if err := decodeBody(r, &body); err != nil || len(body.Cmdline) == 0 {
		writeError(w, http.StatusBadRequest, "cmdline required")
		return
	}
	out, err := d.runInNs(ns, "tc", body.Cmdline...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *daemon) handleNft(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var body struct {
		Cmdline []string `json:"cmdline"`
		Script  string   `json:"script"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	var cmd *exec.Cmd
	if body.Script != "" {
		cmd = d.buildCmd(ns, "nft", "-f", "-")
		cmd.Stdin = strings.NewReader(body.Script)
	} else if len(body.Cmdline) > 0 {
		cmd = d.buildCmd(ns, "nft", body.Cmdline...)
	} else {
		writeError(w, http.StatusBadRequest, "cmdline or script required")
		return
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ---- BPF handlers -------------------------------------------------------

func (d *daemon) handleListBpf(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	names := make([]string, 0, len(d.bpfObjs))
	for name := range d.bpfObjs {
		names = append(names, name)
	}
	d.mu.Unlock()
	sort.Strings(names)
	writeJSON(w, names)
}

func (d *daemon) handleGetBpf(w http.ResponseWriter, r *http.Request) {
	obj := r.PathValue("obj")
	d.mu.Lock()
	path, ok := d.bpfObjs[obj]
	d.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "BPF object not found: "+obj)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

type bpfAttachReq struct {
	Iface     string `json:"iface"`
	Object    string `json:"object"`
	Section   string `json:"section"`
	Direction string `json:"direction"` // TC only: "ingress" or "egress"
}

func (d *daemon) resolveBpfObject(name string) (string, bool) {
	d.mu.Lock()
	path, ok := d.bpfObjs[name]
	d.mu.Unlock()
	return path, ok
}

func (d *daemon) handleAttachXDP(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var req bpfAttachReq
	if err := decodeBody(r, &req); err != nil || req.Iface == "" || req.Object == "" {
		writeError(w, http.StatusBadRequest, "iface and object required")
		return
	}
	path, ok := d.resolveBpfObject(req.Object)
	if !ok {
		writeError(w, http.StatusNotFound, "BPF object not found: "+req.Object)
		return
	}
	sec := req.Section
	if sec == "" {
		sec = "xdp"
	}
	out, err := d.runInNs(ns, "ip", "link", "set", req.Iface, "xdp", "obj", path, "sec", sec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (d *daemon) handleAttachTCBPF(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var req bpfAttachReq
	if err := decodeBody(r, &req); err != nil || req.Iface == "" || req.Object == "" {
		writeError(w, http.StatusBadRequest, "iface and object required")
		return
	}
	path, ok := d.resolveBpfObject(req.Object)
	if !ok {
		writeError(w, http.StatusNotFound, "BPF object not found: "+req.Object)
		return
	}
	sec := req.Section
	if sec == "" {
		sec = "tc"
	}
	dir := req.Direction
	if dir == "" {
		dir = "ingress"
	}
	out, err := d.runInNs(ns, "tc", "filter", "add", "dev", req.Iface, dir, "bpf", "da", "obj", path, "sec", sec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out))+": "+err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ---- process handlers ---------------------------------------------------

type processReq struct {
	Type string `json:"type"`
	Port int    `json:"port"`
	Addr string `json:"addr"`
}

func (d *daemon) handleListProcs(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	d.mu.Lock()
	var pids []int
	for pid, e := range d.procs {
		if e.ns == ns {
			pids = append(pids, pid)
		}
	}
	d.mu.Unlock()
	if pids == nil {
		pids = []int{}
	}
	writeJSON(w, map[string][]int{"pids": pids})
}

func (d *daemon) handleStartProc(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("name")
	var req processReq
	if err := decodeBody(r, &req); err != nil || req.Type == "" {
		writeError(w, http.StatusBadRequest, "type required")
		return
	}
	pid, err := d.startSockhold(ns, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]int{"pid": pid})
}

func (d *daemon) handleKillProc(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pid")
		return
	}
	d.mu.Lock()
	e, ok := d.procs[pid]
	d.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}
	e.cmd.Process.Kill()
	w.WriteHeader(http.StatusOK)
}

// startSockhold re-execs this binary with --sockhold inside the target
// namespace. It waits for the child to print "ready\n" before returning.
func (d *daemon) startSockhold(ns string, req processReq) (int, error) {
	selfExe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable path: %w", err)
	}

	shArgs := []string{"--sockhold", "--type", req.Type}
	if req.Port != 0 {
		shArgs = append(shArgs, "--port", strconv.Itoa(req.Port))
	}
	if req.Addr != "" {
		shArgs = append(shArgs, "--addr", req.Addr)
	}

	cmd := d.buildCmd(ns, selfExe, shArgs...)

	// The child writes "ready\n" to stdout once sockets are bound, then
	// closes stdout so the parent gets EOF.
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return 0, err
	}
	pw.Close()

	pr.SetDeadline(time.Now().Add(5 * time.Second))
	out, _ := io.ReadAll(pr)
	pr.Close()

	if !strings.Contains(string(out), "ready") {
		cmd.Process.Kill()
		cmd.Wait()
		return 0, fmt.Errorf("sockhold did not become ready (output: %q)", string(out))
	}

	pid := cmd.Process.Pid
	d.mu.Lock()
	d.procs[pid] = &procEntry{cmd: cmd, ns: ns}
	d.mu.Unlock()

	go func() {
		cmd.Wait()
		d.mu.Lock()
		delete(d.procs, pid)
		d.mu.Unlock()
	}()

	return pid, nil
}
