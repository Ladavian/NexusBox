package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// handleConfigsRaw 处理原始 YAML 配置的读取和写入
// GET  /configs/raw  → 返回 config.yaml 的原始内容
// POST /configs/raw  → 将请求体内容写入 config.yaml 并热重载内核
func handleConfigsRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		content, err := os.ReadFile(configTarget)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "读取配置文件失败: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "读取请求体失败: "+err.Error())
			return
		}
		defer r.Body.Close()

		if err := os.WriteFile(configTarget, body, 0644); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "写入配置文件失败: "+err.Error())
			return
		}

		if err := reloadCore(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"status":"warning","message":"配置已写入，但热重载失败: `+err.Error()+`"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok","message":"配置已写入并成功热重载"}`)

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// syncConfigFields 精准更新 config.yaml 中的特定字段，不覆盖其他内容
// 用于订阅设置变更和配置页参数修改
func syncConfigFields(cfg SubscribeConfig) error {
	// 如果配置文件不存在，生成新的
	if _, err := os.Stat(configTarget); os.IsNotExist(err) {
		return generateConfig(cfg)
	}

	content, err := os.ReadFile(configTarget)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	text := string(content)

	// 需要同步的字段列表
	type fieldRule struct {
		key   string
		value string
	}
	var rules []fieldRule

	rules = append(rules, fieldRule{"mixed-port", fmt.Sprintf("%d", cfg.ProxyPort)})
	rules = append(rules, fieldRule{"tproxy-port", fmt.Sprintf("%d", cfg.TproxyPort)})
	rules = append(rules, fieldRule{"external-controller", fmt.Sprintf("'0.0.0.0:%d'", cfg.PanelPort)})
	rules = append(rules, fieldRule{"secret", fmt.Sprintf("'%s'", cfg.PanelSecret)})
	rules = append(rules, fieldRule{"external-controller-unix", fmt.Sprintf("'%s'", coreSocket)})
	rules = append(rules, fieldRule{"allow-lan", "true"})
	rules = append(rules, fieldRule{"ipv6", "true"})
	rules = append(rules, fieldRule{"unified-delay", "true"})
	rules = append(rules, fieldRule{"routing-mark", "255"})
	rules = append(rules, fieldRule{"geodata-mode", "false"})

	// 面板 UI 选择
	uiPath := "ui/meta"
	if cfg.UIPanel == "zashboard" {
		uiPath = "ui/zash"
	}
	rules = append(rules, fieldRule{"external-ui", uiPath})

	if cfg.UIPanel == "zashboard" {
		rules = append(rules, fieldRule{"external-ui-url", `"https://github.com/Zephyruso/zashboard/releases/latest/download/dist-cdn-fonts.zip"`})
	}

	// 应用每个字段的更新
	for _, r := range rules {
		re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(r.key) + `):\s*.*$`)
		if re.MatchString(text) {
			text = re.ReplaceAllString(text, "$1: "+r.value)
		} else {
			// 字段不存在则追加
			text = strings.TrimRight(text, "\n") + "\n" + r.key + ": " + r.value + "\n"
		}
	}

	// 确保 proxy-providers 存在（只追加不删除）
	if len(cfg.Subscriptions) > 0 {
		if !regexp.MustCompile(`(?m)^proxy-providers:`).MatchString(text) {
			var providersBuf strings.Builder
			for i, sub := range cfg.Subscriptions {
				interval := sub.UpdateInterval
				if interval <= 0 { interval = 86400 }
				health := sub.HealthInterval
				if health <= 0 { health = 300 }
				providersBuf.WriteString(fmt.Sprintf("  %s:\n    type: http\n    url: \"%s\"\n    interval: %d\n    path: proxies/%s.yaml\n    health-check:\n      enable: true\n      url: \"https://www.gstatic.com/generate_204\"\n      interval: %d\n",
					sub.Name, sub.URL, interval, sub.Name, health))
				if sub.Prefix != "" {
					providersBuf.WriteString(fmt.Sprintf("    override:\n      additional-prefix: \"%s\"\n", sub.Prefix))
				}
				if i < len(cfg.Subscriptions)-1 {
					providersBuf.WriteString("\n")
				}
			}
			text = strings.TrimRight(text, "\n") + "\nproxy-providers:\n" + providersBuf.String() + "\n"
		}
	}

	// 清理多余空行
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	return os.WriteFile(configTarget, []byte(text), 0644)
}

// syncConfigFileFields 将配置页修改的字段同步到 config.yaml
func syncConfigFileFields(fields map[string]interface{}) {
	content, err := os.ReadFile(configTarget)
	if err != nil {
		return
	}
	text := string(content)

	for key, val := range fields {
		var valueStr string
		switch v := val.(type) {
		case float64:
			if v == float64(int(v)) {
				valueStr = fmt.Sprintf("%d", int(v))
			} else {
				valueStr = fmt.Sprintf("%v", v)
			}
		case bool:
			valueStr = fmt.Sprintf("%v", v)
		case string:
			valueStr = fmt.Sprintf("'%s'", v)
		default:
			continue
		}

		if key == "tun" || key == "dns" || key == "sniffer" || key == "profile" {
			continue
		}

		re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(key) + `):\s*.*$`)
		if re.MatchString(text) {
			text = re.ReplaceAllString(text, "$1: "+valueStr)
		} else {
			text = strings.TrimRight(text, "\n") + "\n" + key + ": " + valueStr + "\n"
		}
	}

	os.WriteFile(configTarget, []byte(text), 0644)
}

// handleConfigReset 一键还原为内置默认配置
// POST /configs/raw/reset
func handleConfigReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	defaultContent, err := getDefaultConfig()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "读取默认配置失败: "+err.Error())
		return
	}

	dir := filepath.Dir(configTarget)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "创建目录失败: "+err.Error())
		return
	}

	if err := os.WriteFile(configTarget, defaultContent, 0644); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "写入默认配置失败: "+err.Error())
		return
	}

	if err := reloadCore(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"warning","message":"默认配置已还原，但热重载失败: `+err.Error()+`"}`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"status":"ok","message":"已还原为默认配置并热重载"}`)
}
