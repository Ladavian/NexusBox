package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// session 存储
var (
	sessions   = make(map[string]sessionInfo)
	sessionMu  sync.RWMutex
)

type sessionInfo struct {
	ExpiresAt time.Time
}

// generateToken 生成随机 session token
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashPassword SHA256 哈希（简单但有效）
func hashPassword(pwd string) string {
	h := sha256.Sum256([]byte(pwd))
	return hex.EncodeToString(h[:])
}

// cleanExpiredSessions 清理过期 session
func cleanExpiredSessions() {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	now := time.Now()
	for token, s := range sessions {
		if now.After(s.ExpiresAt) {
			delete(sessions, token)
		}
	}
}

// authMiddleware 认证中间件，返回 http.Handler
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 检查 cookie 中的 token
		cookie, err := r.Cookie("nexusbox_token")
		if err != nil || cookie.Value == "" {
			writeJSONError(w, http.StatusUnauthorized, "未登录")
			return
		}

		sessionMu.RLock()
		s, ok := sessions[cookie.Value]
		sessionMu.RUnlock()

		if !ok || time.Now().After(s.ExpiresAt) {
			writeJSONError(w, http.StatusUnauthorized, "登录已过期")
			return
		}

		// 每次请求续期 24 小时
		sessionMu.Lock()
		if entry, exists := sessions[cookie.Value]; exists {
			entry.ExpiresAt = time.Now().Add(24 * time.Hour)
			sessions[cookie.Value] = entry
		}
		sessionMu.Unlock()

		next(w, r)
	}
}

// handleLogin 登录处理
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	subscribeMu.RLock()
	cfgUsername := subscribeConfig.Username
	cfgPassword := subscribeConfig.Password
	subscribeMu.RUnlock()

	// 如果未设置账密，拒绝登录
	if cfgUsername == "" || cfgPassword == "" {
		writeJSONError(w, http.StatusUnauthorized, "未设置登录账户，请先通过配置初始化")
		return
	}

	// 恒定时间比较防止时序攻击
	usernameOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(cfgUsername)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(cfgPassword)) == 1

	if !usernameOK || !passwordOK {
		writeJSONError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token, err := generateToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "生成 token 失败")
		return
	}

	sessionMu.Lock()
	sessions[token] = sessionInfo{ExpiresAt: time.Now().Add(24 * time.Hour)}
	sessionMu.Unlock()

	// 设置 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "nexusbox_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 小时
	})

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "登录成功",
	})
}

// handleLogout 登出
func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	cookie, err := r.Cookie("nexusbox_token")
	if err == nil && cookie.Value != "" {
		sessionMu.Lock()
		delete(sessions, cookie.Value)
		sessionMu.Unlock()
	}

	// 清除 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "nexusbox_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
	})

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "已登出",
	})
}

// handleAuthStatus 检查当前登录状态
func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	cookie, err := r.Cookie("nexusbox_token")
	if err != nil || cookie.Value == "" {
		respondJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
		return
	}

	sessionMu.RLock()
	s, ok := sessions[cookie.Value]
	sessionMu.RUnlock()

	if !ok || time.Now().After(s.ExpiresAt) {
		respondJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

// handleAuthConfig 获取或更新认证配置（账密）
func handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		subscribeMu.RLock()
		u := subscribeConfig.Username
		subscribeMu.RUnlock()
		respondJSON(w, http.StatusOK, map[string]string{
			"username": u,
		})

	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "无效的请求格式")
			return
		}
		if req.Username == "" {
			writeJSONError(w, http.StatusBadRequest, "用户名不能为空")
			return
		}
		if req.Password == "" {
			writeJSONError(w, http.StatusBadRequest, "密码不能为空")
			return
		}

		subscribeMu.Lock()
		subscribeConfig.Username = req.Username
		subscribeConfig.Password = req.Password
		subscribeMu.Unlock()

		if err := saveSubscribeConfig(); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "认证配置已保存",
		})

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
