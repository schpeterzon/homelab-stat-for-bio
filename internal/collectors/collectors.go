package collectors

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/schpeterzon/homelab-stat-for-bio/internal/config"
	"github.com/schpeterzon/homelab-stat-for-bio/internal/models"
)

// Collector can be registered without changing the collection coordinator.
// It returns its own result and never writes shared Status state.
type Collector interface {
	Name() string
	Collect() (models.Result, error)
}

func Collect(c config.Config) models.Status {
	s := models.Status{Updated: time.Now().UTC()}
	registry := []Collector{HostCollector{StorageTotalTB: c.Storage.TotalTB, StorageUsedPercent: c.Storage.UsedPercent}}
	if c.Collectors.Kubernetes {
		registry = append(registry, KubernetesCollector{Kubeconfig: c.Kubeconfig})
	}
	if c.Collectors.Docker {
		registry = append(registry, DockerCollector{Hosts: c.DockerHosts})
	}
	if c.Collectors.Proxmox {
		registry = append(registry, ProxmoxCollector{Config: c})
	}

	type outcome struct {
		name   string
		result models.Result
		err    error
	}
	results := make(chan outcome, len(registry))
	var wg sync.WaitGroup
	for _, collector := range registry {
		wg.Add(1)
		go func(collector Collector) {
			defer wg.Done()
			result, err := collector.Collect()
			results <- outcome{collector.Name(), result, err}
		}(collector)
	}
	wg.Wait()
	close(results)
	for outcome := range results {
		// A collector may return partial data with an error, e.g. one of
		// several Docker endpoints unreachable; keep both.
		if outcome.err != nil {
			s.Errors = append(s.Errors, outcome.name+": "+outcome.err.Error())
		}
		outcome.result.Apply(&s)
	}
	headline(&s, c)
	// Every enabled collector is expected to report. A failure is shown as
	// Degraded rather than hidden: disable collectors this host cannot reach.
	s.Health = "Operational"
	if len(s.Errors) > 0 {
		s.Health = "Degraded"
	}
	sort.Strings(s.Errors)
	return s
}

type HostCollector struct{ StorageTotalTB, StorageUsedPercent float64 }

func (HostCollector) Name() string { return "host" }
func (h HostCollector) Collect() (models.Result, error) {
	cpu, err := cpuPercent()
	if err != nil {
		return models.Result{}, err
	}
	memory, err := memoryPercent()
	if err != nil {
		return models.Result{}, err
	}
	storage, err := hostStorage()
	if err != nil {
		return models.Result{}, err
	}
	if h.StorageTotalTB > 0 {
		storage.Total = h.StorageTotalTB
		storage.Used = h.StorageTotalTB * h.StorageUsedPercent / 100
		storage.Mountpoint = "configured"
	}
	hostname, _ := os.Hostname()
	kernel, _ := command("uname", "-r")
	uptime, err := uptimeSeconds()
	if err != nil {
		return models.Result{}, err
	}
	return models.Result{System: &models.SystemStatus{CPU: cpu, Memory: memory, Storage: storage, Hostname: hostname, Uptime: uptime, Kernel: kernel}}, nil
}

// hostStorage avoids reporting a tiny immutable overlay on bootc/OSTree hosts.
// On conventional systems /var resolves to the root filesystem.
func hostStorage() (models.Storage, error) {
	root, err := statStorage("/")
	if err != nil {
		return models.Storage{}, err
	}
	if root.Total >= 1 {
		return root, nil
	}
	if persistent, err := statStorage("/var"); err == nil && persistent.Total > root.Total {
		return persistent, nil
	}
	return root, nil
}

func statStorage(path string) (models.Storage, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return models.Storage{}, err
	}
	total := float64(fs.Blocks) * float64(fs.Bsize) / (1 << 40)
	available := float64(fs.Bavail) * float64(fs.Bsize) / (1 << 40)
	return models.Storage{Used: total - available, Total: total, Mountpoint: path}, nil
}

type KubernetesCollector struct{ Kubeconfig string }

func (KubernetesCollector) Name() string { return "kubernetes" }
func (k KubernetesCollector) Collect() (models.Result, error) {
	kubectl := func(args ...string) (string, error) {
		if k.Kubeconfig != "" {
			args = append([]string{"--kubeconfig", k.Kubeconfig}, args...)
		}
		return command("kubectl", append(args, "--request-timeout=10s")...)
	}
	var nodes, pods struct {
		Items []struct {
			Status struct {
				Phase      string                          `json:"phase"`
				Conditions []struct{ Type, Status string } `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	out, err := kubectl("get", "nodes", "-o", "json")
	if err != nil {
		return models.Result{}, fmt.Errorf("node query failed: %w", err)
	}
	if err := json.Unmarshal([]byte(out), &nodes); err != nil {
		return models.Result{}, err
	}
	out, err = kubectl("get", "pods", "-A", "-o", "json")
	if err != nil {
		return models.Result{}, fmt.Errorf("pod query failed: %w", err)
	}
	if err := json.Unmarshal([]byte(out), &pods); err != nil {
		return models.Result{}, err
	}
	st := models.KubernetesStatus{Nodes: len(nodes.Items), Pods: len(pods.Items)}
	for _, n := range nodes.Items {
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				st.Ready++
			}
		}
	}
	for _, p := range pods.Items {
		if p.Status.Phase == "Running" {
			st.Running++
		}
	}
	if out, err := kubectl("get", "deployments", "-A", "--no-headers"); err == nil {
		st.Deployments = lines(out)
	}
	// Usage needs metrics-server (bundled with k3s); absence is not an error.
	if out, err := kubectl("top", "nodes", "--no-headers"); err == nil {
		st.CPU, st.Memory = topAverages(out)
	}
	return models.Result{Kubernetes: &st}, nil
}

// topAverages averages the CPU% and MEMORY% columns of `kubectl top nodes`.
func topAverages(out string) (cpu, mem float64) {
	n := 0.0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		c, err1 := strconv.ParseFloat(strings.TrimSuffix(f[2], "%"), 64)
		m, err2 := strconv.ParseFloat(strings.TrimSuffix(f[4], "%"), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		cpu, mem, n = cpu+c, mem+m, n+1
	}
	if n == 0 {
		return 0, 0
	}
	return round(cpu / n), round(mem / n)
}

// DockerCollector counts running containers on every endpoint. "local" uses
// the local daemon; "ssh:user@host" runs `docker ps -q` over SSH, which pairs
// with a forced command in the remote authorized_keys (see README).
type DockerCollector struct{ Hosts []string }

func (DockerCollector) Name() string { return "docker" }
func (d DockerCollector) Collect() (models.Result, error) {
	hosts := d.Hosts
	if len(hosts) == 0 {
		hosts = []string{"local"}
	}
	var st models.DockerStatus
	var failed []string
	for _, h := range hosts {
		var out string
		var err error
		name := h
		if target, ok := strings.CutPrefix(h, "ssh:"); ok {
			name = target
			if i := strings.LastIndex(target, "@"); i >= 0 {
				name = target[i+1:]
			}
			out, err = command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", target, "docker", "ps", "-q")
		} else {
			name, _ = os.Hostname()
			out, err = command("docker", "ps", "-q")
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", h, err))
			continue
		}
		st.Hosts = append(st.Hosts, models.DockerHost{Name: name, Containers: lines(out)})
		st.Containers += lines(out)
	}
	var err error
	if len(failed) > 0 {
		err = fmt.Errorf("unreachable endpoints: %s", strings.Join(failed, "; "))
	}
	if len(st.Hosts) == 0 {
		return models.Result{}, err
	}
	return models.Result{Docker: &st}, err
}

type ProxmoxCollector struct{ Config config.Config }

func (ProxmoxCollector) Name() string { return "proxmox" }
func (p ProxmoxCollector) Collect() (models.Result, error) {
	c := p.Config
	if c.Proxmox.URL == "" || c.Proxmox.TokenID == "" || c.Proxmox.TokenSecret == "" {
		return models.Result{}, fmt.Errorf("url, token_id, and token secret are required")
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !c.Proxmox.VerifyTLS}}}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(c.Proxmox.URL, "/")+"/api2/json/cluster/resources", nil)
	if err != nil {
		return models.Result{}, err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.Proxmox.TokenID+"="+c.Proxmox.TokenSecret)
	resp, err := client.Do(req)
	if err != nil {
		return models.Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return models.Result{}, fmt.Errorf("API returned %s", resp.Status)
	}
	var payload struct {
		Data []resource `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return models.Result{}, err
	}
	out := aggregate(payload.Data)
	return models.Result{Proxmox: &out}, nil
}

// resource is one entry of /api2/json/cluster/resources.
type resource struct {
	Type     string  `json:"type"`
	Node     string  `json:"node"`
	Status   string  `json:"status"`
	Storage  string  `json:"storage"`
	Shared   int     `json:"shared"`
	Template int     `json:"template"`
	CPU      float64 `json:"cpu"`
	MaxCPU   float64 `json:"maxcpu"`
	Mem      float64 `json:"mem"`
	MaxMem   float64 `json:"maxmem"`
	Disk     float64 `json:"disk"`
	MaxDisk  float64 `json:"maxdisk"`
	Uptime   int64   `json:"uptime"`
}

func aggregate(data []resource) models.ProxmoxStatus {
	var out models.ProxmoxStatus
	var cpuWeighted, mem, maxMem float64
	hosts := map[string]*models.ProxmoxNode{}
	seenShared := map[string]bool{}
	for _, r := range data {
		if r.Type == "node" {
			out.Nodes++
			n := &models.ProxmoxNode{Name: r.Node, Online: r.Status == "online", Uptime: r.Uptime}
			if n.Online {
				out.Online++
				out.Cores += int(r.MaxCPU)
				cpuWeighted += r.CPU * r.MaxCPU
				mem, maxMem = mem+r.Mem, maxMem+r.MaxMem
				n.CPU = round(r.CPU * 100)
				if r.MaxMem > 0 {
					n.Memory = round(r.Mem / r.MaxMem * 100)
				}
			}
			hosts[r.Node] = n
		}
	}
	for _, r := range data {
		switch r.Type {
		case "qemu", "lxc":
			if r.Template == 1 {
				continue
			}
			out.Guests++
			if r.Status == "running" {
				out.Running++
				if n := hosts[r.Node]; n != nil {
					n.Guests++
				}
			}
		case "storage":
			if r.Status != "available" || r.MaxDisk == 0 {
				continue
			}
			if r.Shared == 1 {
				if seenShared[r.Storage] {
					continue
				}
				seenShared[r.Storage] = true
			}
			out.Storage.Total += r.MaxDisk / (1 << 40)
			out.Storage.Used += r.Disk / (1 << 40)
		}
	}
	if out.Cores > 0 {
		out.CPU = round(cpuWeighted / float64(out.Cores) * 100)
	}
	if maxMem > 0 {
		out.Memory = round(mem / maxMem * 100)
		out.MemTB = maxMem / (1 << 40)
	}
	out.Storage.Used, out.Storage.Total = math.Round(out.Storage.Used*100)/100, math.Round(out.Storage.Total*100)/100
	for _, n := range hosts {
		out.Hosts = append(out.Hosts, *n)
	}
	sort.Slice(out.Hosts, func(i, j int) bool { return out.Hosts[i].Name < out.Hosts[j].Name })
	return out
}

// headline promotes cluster-wide Proxmox figures to the headline metrics so
// the cards describe the whole lab, not the VM the agent happens to run on.
func headline(s *models.Status, c config.Config) {
	s.Scope = "host"
	if c.MetricsSource == "host" || s.Proxmox.Online == 0 {
		return
	}
	s.Scope = "cluster"
	s.System.CPU, s.System.Memory = s.Proxmox.CPU, s.Proxmox.Memory
	if c.Storage.TotalTB == 0 && s.Proxmox.Storage.Total > 0 {
		s.System.Storage = s.Proxmox.Storage
		s.System.Storage.Mountpoint = "proxmox"
	}
}

func command(name string, args ...string) (string, error) {
	b, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
func cpuPercent() (float64, error) {
	a, err := cpuSample()
	if err != nil {
		return 0, err
	}
	time.Sleep(200 * time.Millisecond)
	b, err := cpuSample()
	if err != nil {
		return 0, err
	}
	total, idle := b.total-a.total, b.idle-a.idle
	if total == 0 {
		return 0, nil
	}
	return round((1 - float64(idle)/float64(total)) * 100), nil
}

type sample struct{ total, idle uint64 }

func cpuSample() (sample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return sample{}, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return sample{}, io.ErrUnexpectedEOF
	}
	fields := strings.Fields(scanner.Text())
	var s sample
	for i := 1; i < len(fields); i++ {
		n, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return s, err
		}
		s.total += n
		if i == 4 || i == 5 {
			s.idle += n
		}
	}
	return s, nil
}
func memoryPercent() (float64, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	var total, available float64
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseFloat(fields[1], 64)
		if fields[0] == "MemTotal:" {
			total = value
		}
		if fields[0] == "MemAvailable:" {
			available = value
		}
	}
	if total == 0 {
		return 0, fmt.Errorf("MemTotal unavailable")
	}
	return round((1 - available/total) * 100), nil
}
func uptimeSeconds() (int64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	return int64(n), err
}
func lines(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(s), "\n"))
}
func round(v float64) float64 { return math.Round(v*10) / 10 }
