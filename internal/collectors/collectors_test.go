package collectors

import "testing"

func TestAggregate(t *testing.T) {
	const tb = 1 << 40
	got := aggregate([]resource{
		{Type: "node", Node: "a", Status: "online", CPU: 0.5, MaxCPU: 4, Mem: 2, MaxMem: 8, Uptime: 10},
		{Type: "node", Node: "b", Status: "online", CPU: 0.1, MaxCPU: 12, Mem: 2, MaxMem: 8},
		{Type: "node", Node: "c", Status: "offline", MaxCPU: 8, MaxMem: 8},
		{Type: "qemu", Node: "a", Status: "running"},
		{Type: "lxc", Node: "b", Status: "stopped"},
		{Type: "qemu", Node: "a", Status: "stopped", Template: 1},
		{Type: "storage", Node: "a", Storage: "local", Status: "available", Disk: 1 * tb, MaxDisk: 2 * tb},
		{Type: "storage", Node: "a", Storage: "nas", Shared: 1, Status: "available", Disk: 1 * tb, MaxDisk: 4 * tb},
		{Type: "storage", Node: "b", Storage: "nas", Shared: 1, Status: "available", Disk: 1 * tb, MaxDisk: 4 * tb},
	})
	// CPU is weighted by cores: (0.5*4 + 0.1*12) / 16 = 20 %.
	if got.Nodes != 3 || got.Online != 2 || got.Cores != 16 || got.CPU != 20 || got.Memory != 25 {
		t.Fatalf("nodes/cpu/memory: %+v", got)
	}
	if got.Guests != 2 || got.Running != 1 {
		t.Fatalf("guests: %+v", got)
	}
	if got.Storage.Total != 6 || got.Storage.Used != 2 {
		t.Fatalf("shared storage must be counted once: %+v", got.Storage)
	}
	if len(got.Hosts) != 3 || got.Hosts[0].Guests != 1 || got.Hosts[2].Online {
		t.Fatalf("hosts: %+v", got.Hosts)
	}
}

func TestTopAverages(t *testing.T) {
	cpu, mem := topAverages("venus 250m 6% 2000Mi 40%\nmars 750m 18% 1000Mi 20%\n")
	if cpu != 12 || mem != 30 {
		t.Fatalf("got %v %v", cpu, mem)
	}
}
