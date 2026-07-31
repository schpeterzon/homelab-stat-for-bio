package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	RepositoryPath string
	Template       string
	README         string
	StatusFile     string
	AssetsDir      string
	ClusterName    string
	Collectors     struct { Kubernetes, Docker, Proxmox bool }
	Publish        struct { Enabled bool; Remote, Branch, Message string }
	Proxmox        struct { URL, TokenID, TokenSecret string; VerifyTLS bool }
}

func Defaults() Config {
	var c Config
	c.RepositoryPath, c.Template, c.README, c.StatusFile, c.AssetsDir, c.ClusterName = ".", "template.md", "README.md", "status.json", "assets", "Homelab"
	c.Collectors.Kubernetes, c.Collectors.Docker = true, true
	c.Publish.Remote, c.Publish.Branch, c.Publish.Message = "origin", "main", "chore: update homelab status"
	c.Proxmox.VerifyTLS = true
	return c
}

func Load(path string) (Config, error) {
	c := Defaults()
	if path == "" { return c, nil }
	b, err := os.ReadFile(path)
	if err != nil { if os.IsNotExist(err) { return c, nil }; return c, err }
	// JSON is valid YAML and is useful for automation; accept it before the small YAML parser.
	if json.Unmarshal(b, &c) == nil { return c, c.normalize(filepath.Dir(path)) }
	values, err := parseYAML(string(b)); if err != nil { return c, err }
	set := func(key string, target *string) { if v, ok := values[key]; ok { *target = v } }
	set("repository_path", &c.RepositoryPath); set("template", &c.Template); set("readme", &c.README); set("status_file", &c.StatusFile); set("assets_dir", &c.AssetsDir); set("cluster_name", &c.ClusterName)
	set("publish.remote", &c.Publish.Remote); set("publish.branch", &c.Publish.Branch); set("publish.message", &c.Publish.Message)
	set("proxmox.url", &c.Proxmox.URL); set("proxmox.token_id", &c.Proxmox.TokenID); set("proxmox.token_secret", &c.Proxmox.TokenSecret)
	c.Collectors.Kubernetes = boolValue(values, "collectors.kubernetes", c.Collectors.Kubernetes); c.Collectors.Docker = boolValue(values, "collectors.docker", c.Collectors.Docker); c.Collectors.Proxmox = boolValue(values, "collectors.proxmox", c.Collectors.Proxmox)
	c.Publish.Enabled = boolValue(values, "publish.enabled", c.Publish.Enabled); c.Proxmox.VerifyTLS = boolValue(values, "proxmox.verify_tls", c.Proxmox.VerifyTLS)
	return c, c.normalize(filepath.Dir(path))
}

func (c *Config) normalize(base string) error {
	if c.Proxmox.TokenSecret == "" { c.Proxmox.TokenSecret = os.Getenv("PROXMOX_TOKEN_SECRET") }
	if c.RepositoryPath == "" { c.RepositoryPath = "." }
	if !filepath.IsAbs(c.RepositoryPath) { c.RepositoryPath = filepath.Join(base, c.RepositoryPath) }
	return nil
}

func boolValue(v map[string]string, key string, fallback bool) bool { if s, ok := v[key]; ok { b, err := strconv.ParseBool(s); if err == nil { return b } }; return fallback }

func parseYAML(input string) (map[string]string, error) {
	result, sections := map[string]string{}, []string{}
	s := bufio.NewScanner(strings.NewReader(input))
	for s.Scan() {
		line := strings.TrimRight(s.Text(), " \t"); clean := strings.TrimSpace(line)
		if clean == "" || strings.HasPrefix(clean, "#") { continue }
		indent := (len(line)-len(strings.TrimLeft(line, " "))) / 2
		if indent > len(sections) { return nil, fmt.Errorf("invalid indentation near %q", clean) }
		if indent < len(sections) { sections = sections[:indent] }
		parts := strings.SplitN(clean, ":", 2); if len(parts) != 2 { return nil, fmt.Errorf("invalid YAML line %q", clean) }
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if value == "" { sections = append(sections, key); continue }
		value = strings.Trim(strings.TrimSpace(strings.SplitN(value, " #", 2)[0]), "\"'")
		result[strings.Join(append(sections, key), ".")] = value
	}
	return result, s.Err()
}
