package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ===== 数据结构 =====

type TrafficPolicyConfig struct {
	Mode           string   `json:"mode"`             // "all", "whitelist", "blacklist"
	EnableFastPath bool     `json:"enable_fast_path"` // DIRECT Fast Path 开关
	Whitelist      []Client `json:"whitelist"`
	Blacklist      []Client `json:"blacklist"`
}

type Client struct {
	IP     string `json:"ip"`
	Remark string `json:"remark"`
}

type DiscoveredDevice struct {
	IP      string `json:"ip"`
	Hostname string `json:"hostname"`
	Vendor  string `json:"vendor"`
}

var (
	trafficPolicyConfig TrafficPolicyConfig
	trafficPolicyMu     sync.RWMutex
	trafficPolicyFile   string
)

func init() {
	trafficPolicyFile = getEnv("TRAFFIC_POLICY_FILE", filepath.Join(filepath.Dir(configTarget), "traffic_policy.json"))
}

// ===== 配置持久化 =====

func loadTrafficPolicyConfig() {
	trafficPolicyMu.Lock()
	defer trafficPolicyMu.Unlock()

	defaultCfg := TrafficPolicyConfig{
		Mode:           "all",
		EnableFastPath: false,
		Whitelist:      []Client{},
		Blacklist:      []Client{},
	}

	data, err := os.ReadFile(trafficPolicyFile)
	if err != nil {
		trafficPolicyConfig = defaultCfg
		return
	}

	var cfg TrafficPolicyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("解析流量策略配置失败: %v，使用默认配置", err)
		trafficPolicyConfig = defaultCfg
		return
	}

	// 默认值填充
	if cfg.Mode == "" {
		cfg.Mode = "all"
	}
	if cfg.Whitelist == nil {
		cfg.Whitelist = []Client{}
	}
	if cfg.Blacklist == nil {
		cfg.Blacklist = []Client{}
	}

	trafficPolicyConfig = cfg
	log.Printf("流量策略已加载: mode=%s, whitelist=%d, blacklist=%d", cfg.Mode, len(cfg.Whitelist), len(cfg.Blacklist))
}

func saveTrafficPolicyConfig() error {
	trafficPolicyMu.RLock()
	data, err := json.MarshalIndent(trafficPolicyConfig, "", "  ")
	trafficPolicyMu.RUnlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(trafficPolicyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(trafficPolicyFile, data, 0644)
}

// ===== nftables 规则管理 =====

func applyTrafficPolicyRules() error {
	trafficPolicyMu.RLock()
	mode := trafficPolicyConfig.Mode
	whitelist := trafficPolicyConfig.Whitelist
	blacklist := trafficPolicyConfig.Blacklist
	fastPath := trafficPolicyConfig.EnableFastPath
	trafficPolicyMu.RUnlock()

	cleanupTrafficPolicyRules()

	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[TrafficPolicy] 命令失败 %s %v: %v, %s", name, args, err, string(out))
		}
	}

	runCmd("nft", "add", "table", "ip", "nexusbox_tproxy")

	// ---- Fast Path ----
	if fastPath {
		if err := applyFastPathRules(); err != nil {
			log.Printf("[TrafficPolicy] Fast Path 失败: %v", err)
		}
	}

	if mode == "all" {
		return nil
	}

	var bypassIPs []string
	if mode == "whitelist" {
		runCmd("nft", "add", "set", "ip", "nexusbox_tproxy", "traffic_proxy", "{ type ipv4_addr; flags interval; }")
		runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "traffic_proxy")
		for _, c := range whitelist {
			if isValidCIDR(c.IP) {
				runCmd("nft", "add", "element", "ip", "nexusbox_tproxy", "traffic_proxy", "{", c.IP, "}")
			}
		}
		runCmd("nft", "add", "chain", "ip", "nexusbox_tproxy", "prerouting", "{ type filter hook prerouting priority mangle; policy accept; }", "2>/dev/null")
		runCmd("nft", "insert", "rule", "ip", "nexusbox_tproxy", "prerouting", "ip", "saddr", "!=", "@traffic_proxy", "return")
	} else if mode == "blacklist" {
		for _, c := range blacklist {
			if c.IP != "" {
				bypassIPs = append(bypassIPs, c.IP)
			}
		}
		if len(bypassIPs) > 0 {
			runCmd("nft", "add", "set", "ip", "nexusbox_tproxy", "traffic_bypass", "{ type ipv4_addr; flags interval; }")
			runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "traffic_bypass")
			for _, ip := range bypassIPs {
				if isValidCIDR(ip) {
					runCmd("nft", "add", "element", "ip", "nexusbox_tproxy", "traffic_bypass", "{", ip, "}")
				}
			}
			runCmd("nft", "add", "chain", "ip", "nexusbox_tproxy", "prerouting", "{ type filter hook prerouting priority mangle; policy accept; }", "2>/dev/null")
			runCmd("nft", "insert", "rule", "ip", "nexusbox_tproxy", "prerouting", "ip", "saddr", "@traffic_bypass", "return")
		}
	}

	return nil
}

// ===== DIRECT Fast Path =====

const fastPathCacheFile = "/opt/nexusbox/var/fastpath_cn.cidr"

func applyFastPathRules() error {
	if _, err := os.Stat(fastPathCacheFile); os.IsNotExist(err) {
		if err := downloadCNIPList(); err != nil {
			return fmt.Errorf("download CN IP list failed: %w", err)
		}
	}

	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[FastPath] cmd failed %s: %v", name, err)
		}
	}

	runCmd("nft", "add", "set", "ip", "nexusbox_tproxy", "direct_bypass", "{ type ipv4_addr; flags interval; }")
	runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "direct_bypass")

	f, err := os.Open(fastPathCacheFile)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		runCmd("nft", "add", "element", "ip", "nexusbox_tproxy", "direct_bypass", "{", line, "}")
		count++
	}
	log.Printf("[FastPath] loaded %d CN IP ranges into direct_bypass", count)

	runCmd("nft", "add", "chain", "ip", "nexusbox_tproxy", "prerouting", "{ type filter hook prerouting priority mangle; policy accept; }", "2>/dev/null")
	runCmd("nft", "insert", "rule", "ip", "nexusbox_tproxy", "prerouting", "ip", "daddr", "@direct_bypass", "return")

	log.Printf("[FastPath] DIRECT Fast Path enabled, CN IP traffic bypasses Mihomo")
	return nil
}

func cleanupFastPathRules() {
	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.CombinedOutput()
	}
	runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "direct_bypass", "2>/dev/null")
	runCmd("nft", "delete", "set", "ip", "nexusbox_tproxy", "direct_bypass", "2>/dev/null")
}

func downloadCNIPList() error {
	urls := []string{
		"https://raw.githubusercontent.com/mayaxcn/china-ip-list/master/chnroute.txt",
	}

	var lastErr error
	for _, u := range urls {
		log.Printf("[FastPath] downloading CN IP list: %s", u)
		resp, err := http.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		dir := filepath.Dir(fastPathCacheFile)
		os.MkdirAll(dir, 0755)
		f, err := os.Create(fastPathCacheFile)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return err
		}
		log.Printf("[FastPath] CN IP list cached to %s", fastPathCacheFile)
		return nil
	}
	return fmt.Errorf("all sources failed: %w", lastErr)
}

func cleanupTrafficPolicyRules() {
	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.CombinedOutput() // 忽略错误
	}

	// 删除 traffic policy 相关规则和集合
	runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "traffic_proxy", "2>/dev/null")
	runCmd("nft", "flush", "set", "ip", "nexusbox_tproxy", "traffic_bypass", "2>/dev/null")
	runCmd("nft", "delete", "set", "ip", "nexusbox_tproxy", "traffic_proxy", "2>/dev/null")
	runCmd("nft", "delete", "set", "ip", "nexusbox_tproxy", "traffic_bypass", "2>/dev/null")

	// 同时清理 Fast Path 规则
	cleanupFastPathRules()
}

func isValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	if err == nil {
		return true
	}
	ip := net.ParseIP(s)
	return ip != nil
}

// reapplyTrafficPolicy 热更新流量策略（不重启 Mihomo）
func reapplyTrafficPolicy() error {
	if err := saveTrafficPolicyConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	if err := applyTrafficPolicyRules(); err != nil {
		return fmt.Errorf("应用规则失败: %w", err)
	}
	return nil
}

// ===== 设备发现 =====

func discoverDevices() []DiscoveredDevice {
	var devices []DiscoveredDevice
	seen := make(map[string]bool)

	// 1. 读取 DHCP leases
	if f, err := os.Open("/var/lib/misc/dnsmasq.leases"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 4 {
				ip := parts[2]
				if !seen[ip] && !isPrivateOrSpecial(ip) {
					seen[ip] = true
					devices = append(devices, DiscoveredDevice{
						IP:       ip,
						Hostname: parts[3],
						Vendor:   "",
					})
				}
			}
		}
	}

	// 2. 读取 ARP 表
	if f, err := os.Open("/proc/net/arp"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Scan() // skip header
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) >= 4 {
				ip := parts[0]
				if !seen[ip] && !isPrivateOrSpecial(ip) {
					seen[ip] = true
					devices = append(devices, DiscoveredDevice{
						IP:       ip,
						Hostname: "",
						Vendor:   "",
					})
				}
			}
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].IP < devices[j].IP
	})

	return devices
}

func isPrivateOrSpecial(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	// 排除 0.0.0.0, 127.x, 网关等
	if ip.String() == "0.0.0.0" {
		return true
	}
	if ip.IsLoopback() {
		return true
	}
	return false
}

// ===== HTTP Handler =====

func handleTrafficPolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		trafficPolicyMu.RLock()
		cfg := trafficPolicyConfig
		trafficPolicyMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)

	case http.MethodPost:
		var cfg TrafficPolicyConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSONError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
			return
		}

		// 默认值
		if cfg.Mode == "" {
			cfg.Mode = "all"
		}
		if cfg.Whitelist == nil {
			cfg.Whitelist = []Client{}
		}
		if cfg.Blacklist == nil {
			cfg.Blacklist = []Client{}
		}

		trafficPolicyMu.Lock()
		trafficPolicyConfig = cfg
		trafficPolicyMu.Unlock()

		if err := reapplyTrafficPolicy(); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "应用流量策略失败: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "流量策略已更新并生效",
		})

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func handleTrafficPolicyDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := discoverDevices()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func handleTrafficPolicyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	trafficPolicyMu.RLock()
	mode := trafficPolicyConfig.Mode
	whitelistCount := len(trafficPolicyConfig.Whitelist)
	blacklistCount := len(trafficPolicyConfig.Blacklist)
	fastPath := trafficPolicyConfig.EnableFastPath
	trafficPolicyMu.RUnlock()

	// 从 nftables 获取当前连接数（简化：返回配置状态）
	status := map[string]interface{}{
		"mode":            mode,
		"fast_path":       fastPath,
		"whitelist_count": whitelistCount,
		"blacklist_count": blacklistCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
