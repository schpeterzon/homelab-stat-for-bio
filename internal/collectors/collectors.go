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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/schp/homelab-stat-for-bio/internal/config"
	"github.com/schp/homelab-stat-for-bio/internal/models"
)

// Collector can be registered without changing the collection coordinator.
// It returns its own result and never writes shared Status state.
type Collector interface {
	Name() string
	Collect() (models.Result, error)
}

func Collect(c config.Config) models.Status {
	s := models.Status{Health: "Healthy", Updated: time.Now().UTC()}
	registry := []Collector{HostCollector{StorageTotalTB: c.Storage.TotalTB, StorageUsedPercent: c.Storage.UsedPercent}}
	if c.Collectors.Kubernetes {
		registry = append(registry, KubernetesCollector{})
	}
	if c.Collectors.Docker {
		registry = append(registry, DockerCollector{})
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
		if outcome.err != nil {
			s.Errors = append(s.Errors, outcome.name+": "+outcome.err.Error())
			continue
		}
		outcome.result.Apply(&s)
	}
	// A missing optional source (for example kubectl on the Docker VM) is a
	// collector notice, not a health failure. Health is reserved for freshness;
	// the README keeps the notices visible for diagnosis.
	s.Health = "Healthy"
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

type KubernetesCollector struct{}

func (KubernetesCollector) Name() string { return "kubernetes" }
func (KubernetesCollector) Collect() (models.Result, error) {
	nodes, err := command("kubectl", "get", "nodes", "--no-headers")
	if err != nil {
		return models.Result{}, fmt.Errorf("kubectl unavailable or query failed: %w", err)
	}
	pods, err := command("kubectl", "get", "pods", "-A", "--no-headers")
	if err != nil {
		return models.Result{}, fmt.Errorf("pod query failed: %w", err)
	}
	return models.Result{Kubernetes: &models.KubernetesStatus{Nodes: lines(nodes), Pods: lines(pods)}}, nil
}

type DockerCollector struct{}

func (DockerCollector) Name() string { return "docker" }
func (DockerCollector) Collect() (models.Result, error) {
	out, err := command("docker", "ps", "-q")
	if err != nil {
		return models.Result{}, fmt.Errorf("docker unavailable or query failed: %w", err)
	}
	return models.Result{Docker: &models.DockerStatus{Containers: lines(out)}}, nil
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
		Data []struct {
			Type    string  `json:"type"`
			MaxMem  float64 `json:"maxmem"`
			Mem     float64 `json:"mem"`
			MaxDisk float64 `json:"maxdisk"`
			Disk    float64 `json:"disk"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return models.Result{}, err
	}
	var out models.ProxmoxStatus
	var maxMem float64
	for _, r := range payload.Data {
		if r.Type == "node" {
			out.Nodes++
			maxMem += r.MaxMem
			out.Memory += r.Mem
			out.Storage.Total += r.MaxDisk / (1 << 40)
			out.Storage.Used += r.Disk / (1 << 40)
		}
	}
	if maxMem > 0 {
		out.Memory = round(out.Memory / maxMem * 100)
	}
	return models.Result{Proxmox: &out}, nil
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
