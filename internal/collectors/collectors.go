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
	"syscall"
	"time"

	"github.com/schp/homelab-stat-for-bio/internal/config"
	"github.com/schp/homelab-stat-for-bio/internal/models"
)

func Collect(c config.Config) models.Status {
	s := models.Status{Cluster: "Healthy", Updated: time.Now().UTC()}
	if err := host(&s); err != nil { s.Errors = append(s.Errors, "host: "+err.Error()) }
	if c.Collectors.Kubernetes { if err := kubernetes(&s); err != nil { s.Errors = append(s.Errors, "kubernetes: "+err.Error()) } }
	if c.Collectors.Docker { if err := docker(&s); err != nil { s.Errors = append(s.Errors, "docker: "+err.Error()) } }
	if c.Collectors.Proxmox { if err := proxmox(&s, c); err != nil { s.Errors = append(s.Errors, "proxmox: "+err.Error()) } }
	if len(s.Errors) > 0 { s.Cluster = "Degraded" }
	return s
}

func command(name string, args ...string) (string, error) { b, err := exec.Command(name, args...).Output(); if err != nil { return "", err }; return strings.TrimSpace(string(b)), nil }

func kubernetes(s *models.Status) error {
	nodes, err := command("kubectl", "get", "nodes", "--no-headers"); if err != nil { return err }
	pods, err := command("kubectl", "get", "pods", "-A", "--no-headers"); if err != nil { return err }
	s.Nodes, s.Pods = lines(nodes), lines(pods); return nil
}

func docker(s *models.Status) error { out, err := command("docker", "ps", "-q"); if err != nil { return err }; s.Containers = lines(out); return nil }

func host(s *models.Status) error {
	cpu, err := cpuPercent(); if err != nil { return err }; s.CPU = cpu
	mem, err := memoryPercent(); if err != nil { return err }; s.Memory = mem
	var stat syscall.Statfs_t; if err := syscall.Statfs("/", &stat); err != nil { return err }
	total := float64(stat.Blocks)*float64(stat.Bsize)/(1<<40); available := float64(stat.Bavail)*float64(stat.Bsize)/(1<<40)
	s.StorageTotal, s.StorageUsed = total, total-available; return nil
}

func cpuPercent() (float64, error) { a, err := cpuSample(); if err != nil { return 0, err }; time.Sleep(200*time.Millisecond); b, err := cpuSample(); if err != nil { return 0, err }; total, idle := b.total-a.total, b.idle-a.idle; if total == 0 { return 0, nil }; return round((1-float64(idle)/float64(total))*100), nil }
type sample struct{ total, idle uint64 }
func cpuSample() (sample, error) { f, err := os.Open("/proc/stat"); if err != nil { return sample{}, err }; defer f.Close(); scanner := bufio.NewScanner(f); if !scanner.Scan() { return sample{}, io.ErrUnexpectedEOF }; fields := strings.Fields(scanner.Text()); var r sample; for i := 1; i < len(fields); i++ { n, e := strconv.ParseUint(fields[i],10,64); if e != nil{return r,e}; r.total+=n; if i==4 || i==5 {r.idle+=n} }; return r,nil }
func memoryPercent() (float64,error) { b,err:=os.ReadFile("/proc/meminfo"); if err!=nil{return 0,err}; var total,available float64; for _,l:=range strings.Split(string(b),"\n") { f:=strings.Fields(l); if len(f)<2 {continue}; n,_:=strconv.ParseFloat(f[1],64); if f[0]=="MemTotal:" {total=n}; if f[0]=="MemAvailable:" {available=n} }; if total==0{return 0,fmt.Errorf("MemTotal unavailable")}; return round((1-available/total)*100),nil }
func lines(s string) int { if strings.TrimSpace(s)=="" {return 0}; return len(strings.Split(strings.TrimSpace(s),"\n")) }
func round(v float64) float64 { return math.Round(v*10)/10 }

func proxmox(s *models.Status, c config.Config) error {
	if c.Proxmox.URL == "" || c.Proxmox.TokenID == "" || c.Proxmox.TokenSecret == "" { return fmt.Errorf("url, token_id, and token secret are required") }
	client:=&http.Client{Timeout:10*time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !c.Proxmox.VerifyTLS}}}; endpoint:=strings.TrimRight(c.Proxmox.URL,"/")+"/api2/json/cluster/resources"
	req,err:=http.NewRequest(http.MethodGet,endpoint,nil); if err!=nil{return err}; req.Header.Set("Authorization", "PVEAPIToken="+c.Proxmox.TokenID+"="+c.Proxmox.TokenSecret)
	resp,err:=client.Do(req); if err!=nil{return err}; defer resp.Body.Close(); if resp.StatusCode/100!=2{return fmt.Errorf("API returned %s",resp.Status)}
	var payload struct{ Data []struct{ Type string `json:"type"`; MaxMem float64 `json:"maxmem"`; Mem float64 `json:"mem"`; MaxDisk float64 `json:"maxdisk"`; Disk float64 `json:"disk"` } `json:"data"` }; if err:=json.NewDecoder(resp.Body).Decode(&payload);err!=nil{return err}
	var maxMem, usedMem, maxDisk, usedDisk float64; for _,r:=range payload.Data { if r.Type=="node" { s.Nodes++; maxMem+=r.MaxMem; usedMem+=r.Mem; maxDisk+=r.MaxDisk; usedDisk+=r.Disk } }; if maxMem>0{s.Memory=round(usedMem/maxMem*100)}; if maxDisk>0{s.StorageTotal=maxDisk/(1<<40);s.StorageUsed=usedDisk/(1<<40)}; return nil
}
