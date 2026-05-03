package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// bpfStub describes one BPF pass-through stub to build at daemon startup.
type bpfStub struct {
	name string // output ELF name, e.g. "xdp_pass.o"
	src  string // embedded C source
}

var bpfStubs = []bpfStub{
	{
		name: "xdp_pass.o",
		src: `
#define SEC(x) __attribute__((section(x), used))
enum { XDP_PASS = 2 };
SEC("xdp") int xdp_pass(void *ctx) { return XDP_PASS; }
char _license[] SEC("license") = "GPL";
`,
	},
	{
		name: "tc_pass.o",
		src: `
#define SEC(x) __attribute__((section(x), used))
enum { TC_ACT_OK = 0 };
SEC("tc") int tc_pass(void *ctx) { return TC_ACT_OK; }
char _license[] SEC("license") = "GPL";
`,
	},
	{
		name: "sched_act_pass.o",
		src: `
#define SEC(x) __attribute__((section(x), used))
enum { TC_ACT_OK = 0 };
SEC("action") int sched_act_pass(void *ctx) { return TC_ACT_OK; }
char _license[] SEC("license") = "GPL";
`,
	},
}

// buildBPFStubs tries to compile each stub with clang -target bpf.
// Successful builds are recorded in objs (name → absolute path).
// If clang is unavailable the function logs a warning and returns without error.
func buildBPFStubs(dir string, objs map[string]string) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		log.Printf("clang not found; BPF stub objects will be unavailable (XDP/TC BPF tests will be skipped)")
		return
	}

	for _, stub := range bpfStubs {
		base := stub.name[:len(stub.name)-2] // strip ".o"
		srcPath := filepath.Join(dir, base+".c")
		objPath := filepath.Join(dir, stub.name)

		if err := os.WriteFile(srcPath, []byte(stub.src), 0644); err != nil {
			log.Printf("BPF stub %s: write source: %v", stub.name, err)
			continue
		}

		out, err := exec.Command(
			clang, "-O2", "-target", "bpf", "-c", srcPath, "-o", objPath,
		).CombinedOutput()
		if err != nil {
			log.Printf("BPF stub %s: clang: %v\n%s", stub.name, err, out)
			continue
		}

		objs[stub.name] = objPath
		log.Printf("built BPF stub: %s", stub.name)
	}
}
