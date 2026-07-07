package main

import (
	"io"
	"net/http"
	"os"
)

// handleConfigsRaw 处理原始 YAML 配置的读取和写入
// GET  /configs/raw  → 返回 config.yaml 的原始内容
// POST /configs/raw  → 将请求体内容写入 config.yaml 并热重载内核
func handleConfigsRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 读取 config.yaml 文件
		content, err := os.ReadFile(configTarget)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "读取配置文件失败: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)

	case http.MethodPost:
		// 读取请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "读取请求体失败: "+err.Error())
			return
		}
		defer r.Body.Close()

		// 写入 config.yaml
		if err := os.WriteFile(configTarget, body, 0644); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "写入配置文件失败: "+err.Error())
			return
		}

		// 热重载内核
		if err := reloadCore(); err != nil {
			// 配置已保存但重载失败，返回部分成功
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
