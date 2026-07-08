package main

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
)

var staticFS fs.FS

var (
	socketPath          string
	baseURL             string
	nexusboxPidFile       string
	nexusboxBinDir        string
	corePidFile         string
	coreBin             string
	coreSocket          string
	metaDir             string
	zashDir             string
	nexusboxConfigFile    string
	configTarget        string
	infoLogFile         string
	coreWorkDir         string
	tcpAddr             string
	originalBaseURL     string
)

func init() {
	socketPath = getEnv("SOCKET_PATH", "/opt/nexusbox/var/app.sock")
	baseURL = getEnv("BASE_URL", "/")
	originalBaseURL = baseURL
	nexusboxPidFile = getEnv("FLUXOR_PID_FILE", "/opt/nexusbox/var/nexusbox.pid")
	nexusboxBinDir = getEnv("FLUXOR_BIN_DIR", "/opt/nexusbox/")
	corePidFile = getEnv("CORE_PID_FILE", "/opt/nexusbox/var/core.pid")
	coreBin = getEnv("CORE_BIN", "/opt/mihomo/mihomo")
	coreSocket = getEnv("CORE_SOCKET", "/opt/nexusbox/var/core.sock")
	metaDir = getEnv("META_DIR", "/opt/config/ui/meta")
	zashDir = getEnv("ZASH_DIR", "/opt/config/ui/zash")
	nexusboxConfigFile = getEnv("FLUXOR_CONFIG_FILE", "/opt/config/nexusbox.json")
	configTarget = getEnv("CONFIG_TARGET", "/opt/config/config.yaml")
	infoLogFile = getEnv("INFO_LOG_FILE", "/opt/nexusbox/var/info.log")
	coreWorkDir = getEnv("CORE_WORK_DIR", "/opt/config")
	tcpAddr = getEnv("FLUXOR_ADDR", "0.0.0.0:18080")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

const (
	metaConfigFile = "config.js"
)

var (
	indexTmpl *template.Template
	upgrader  = websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func main() {
	// === 解析命令行参数 ===
	openwrtMode := false
	customAddr := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-w", "--openwrt":
			openwrtMode = true
		case "-a", "--addr":
			if i+1 < len(args) {
				customAddr = args[i+1]
				i++ // 跳过下一个参数
			} else {
				fmt.Println("错误：-a 或 --addr 需要指定地址")
				os.Exit(1)
			}
		default:
			// 忽略未知参数
		}
	}

	// === 若为 OpenWrt 模式，设置默认值并允许环境变量覆盖 ===
	if openwrtMode {
		// 1. 设置 OpenWrt 默认值
		socketPath = ""
		baseURL = "/"
		tcpAddr = "0.0.0.0:18080"
		nexusboxPidFile = "/var/run/nexusbox.pid"
		nexusboxBinDir = "/etc/nexusbox/"
		corePidFile = "/var/run/core.pid"
		coreBin = "/etc/nexusbox/mihomo"
		coreSocket = "/etc/nexusbox/core.sock"
		metaDir = "/etc/nexusbox/ui/meta"
		zashDir = "/etc/nexusbox/ui/zash"
		nexusboxConfigFile = "/etc/nexusbox/nexusbox.json"
		configTarget = "/etc/nexusbox/config.yaml"
		infoLogFile = "/etc/nexusbox//info.log"
		coreWorkDir = "/etc/nexusbox"

		// 2. 环境变量覆盖（如果设置了非空值）
		if v := os.Getenv("SOCKET_PATH"); v != "" { socketPath = v }
		if v := os.Getenv("BASE_URL"); v != "" { baseURL = v }
		if v := os.Getenv("FLUXOR_ADDR"); v != "" { tcpAddr = v }
		if v := os.Getenv("FLUXOR_PID_FILE"); v != "" { nexusboxPidFile = v }
		if v := os.Getenv("FLUXOR_BIN_DIR"); v != "" { nexusboxBinDir = v }
		if v := os.Getenv("CORE_PID_FILE"); v != "" { corePidFile = v }
		if v := os.Getenv("CORE_BIN"); v != "" { coreBin = v }
		if v := os.Getenv("CORE_SOCKET"); v != "" { coreSocket = v }
		if v := os.Getenv("META_DIR"); v != "" { metaDir = v }
		if v := os.Getenv("ZASH_DIR"); v != "" { zashDir = v }
		if v := os.Getenv("FLUXOR_CONFIG_FILE"); v != "" { nexusboxConfigFile = v }
		if v := os.Getenv("CONFIG_TARGET"); v != "" { configTarget = v }
		if v := os.Getenv("INFO_LOG_FILE"); v != "" { infoLogFile = v }
		if v := os.Getenv("CORE_WORK_DIR"); v != "" { coreWorkDir = v }

		// 3. 命令行 -a 覆盖 TCP 地址（最高优先级）
		if customAddr != "" {
			tcpAddr = customAddr
		}

		originalBaseURL = baseURL

		fmt.Printf("NexusBox 运行于 OpenWrt 模式，监听 TCP %s\n", tcpAddr)
	} else {
		// 非 OpenWrt 模式：若指定了 -a，则覆盖 tcpAddr（仍可配合 Unix socket 共存）
		if customAddr != "" {
			tcpAddr = customAddr
		}
		// 其余变量保持 init 中从环境变量读取的值
	}

	// === 检查和准备 ===
	loadSubscribeConfig()
	initCoreLogger()
	startAllTimers()
	loadTproxySrcExceptions()
	loadTproxyDstExceptions()
	loadTproxyProxyLocal()

	// DNS 故障切换（如果已启用）
	if subscribeConfig.DnsFailover {
		startDnsFailover()
	}

	if _, err := os.Stat(configTarget); os.IsNotExist(err) {
		if err := generateConfig(subscribeConfig); err != nil {
			fmt.Printf("生成基本配置文件失败: %v\n", err)
		} else {
			fmt.Println("已生成基本配置文件 (config.yaml)")
		}
	}

	if err := os.MkdirAll(filepath.Dir(nexusboxPidFile), 0755); err != nil {
		fmt.Printf("无法创建 PID 目录: %v\n", err)
	} else {
		pidData := []byte(fmt.Sprintf("%d", os.Getpid()))
		if err := os.WriteFile(nexusboxPidFile, pidData, 0644); err != nil {
			fmt.Printf("写入 PID 文件失败: %v\n", err)
		} else {
			defer func() {
				if err := os.Remove(nexusboxPidFile); err != nil {
					fmt.Printf("删除 PID 文件失败: %v\n", err)
				}
			}()
		}
	}

	var err error
	indexTmpl, err = template.ParseFS(staticFS, "static/html/index.html")
	if err != nil {
		fmt.Printf("加载主页模板失败: %v\n", err)
		os.Exit(1)
	}

	// === 检查监听方式 ===
	if socketPath == "" && tcpAddr == "" {
		fmt.Println("错误：未配置任何监听地址（SOCKET_PATH 和 FLUXOR_ADDR 均为空）")
		os.Exit(1)
	}

	// === 创建 Unix socket 监听器（若启用）===
	var listener net.Listener
	if socketPath != "" {
		if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
			fmt.Printf("无法创建 socket 目录: %v\n", err)
			os.Exit(1)
		}
		os.Remove(socketPath)

		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			fmt.Printf("监听 Unix socket 失败: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		if err := os.Chmod(socketPath, 0666); err != nil {
			fmt.Printf("设置 socket 权限失败: %v\n", err)
		}
		fmt.Printf("Unix socket 监听: %s\n", socketPath)
	} else {
		fmt.Println("Unix socket 已禁用")
	}

	// === 创建 TCP 监听器（若启用）===
	var tcpListener net.Listener
	if tcpAddr != "" {
		if err := validateTCPAddr(tcpAddr); err != nil {
			fmt.Printf("无效的 FLUXOR_ADDR 格式: %v，将禁用 TCP 监听\n", err)
			tcpAddr = ""
		}
		if tcpAddr != "" {
			tcpListener, err = net.Listen("tcp", tcpAddr)
			if err != nil {
				fmt.Printf("无法监听 TCP 地址 %s: %v\n", tcpAddr, err)
			} else {
				defer tcpListener.Close()
				fmt.Printf("TCP 监听: %s\n", tcpAddr)
			}
		}
	}

    if baseURL == "/" {
        baseURL = ""
    } else {
        baseURL = strings.TrimSuffix(baseURL, "/")
    }

	// === 创建路由 ===
	mux := http.NewServeMux()

	// 外部静态面板
	mux.Handle(baseURL+"/meta/", http.StripPrefix(baseURL+"/meta/", http.FileServer(http.Dir(metaDir))))
	mux.Handle(baseURL+"/zash/", http.StripPrefix(baseURL+"/zash/", http.FileServer(http.Dir(zashDir))))

	// 内嵌静态文件（直接使用 staticFS，并重写路径前缀以匹配内部目录结构）
    staticFileServer := http.FileServer(http.FS(staticFS))
    mux.Handle(baseURL+"/static/", http.StripPrefix(baseURL+"/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.URL.Path = "/static/" + r.URL.Path
        staticFileServer.ServeHTTP(w, r)
    })))

	// 页面路由
	if baseURL == "" {
        // 根路径直接渲染首页
        mux.HandleFunc("/", handleIndex)
    } else {
        // 根路径重定向到实际前缀
        mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
            if r.URL.Path == "/" {
                redirectTo := baseURL
                if !strings.HasSuffix(redirectTo, "/") {
                    redirectTo += "/"
                }
                http.Redirect(w, r, redirectTo, http.StatusFound)
                return
            }
            http.NotFound(w, r)
        })
        // 实际页面路由
        mux.HandleFunc(baseURL+"/", handleIndex)
    }
	mux.HandleFunc(baseURL+"/whoami", handleWhoAmI)

	// 认证路由（无需认证）
	mux.HandleFunc(baseURL+"/login", handleLogin)
	mux.HandleFunc(baseURL+"/logout", handleLogout)
	mux.HandleFunc(baseURL+"/auth-status", handleAuthStatus)
	mux.HandleFunc(baseURL+"/auth-config", authMiddleware(handleAuthConfig))

	// 内核控制
	mux.HandleFunc(baseURL+"/core/status", authMiddleware(handleCoreStatus))
	mux.HandleFunc(baseURL+"/core/start", authMiddleware(handleCoreStart))
	mux.HandleFunc(baseURL+"/core/stop", authMiddleware(handleCoreStop))
	mux.HandleFunc(baseURL+"/core/restart", authMiddleware(handleCoreRestart))
	mux.HandleFunc(baseURL+"/upgrade", authMiddleware(handleUpgrade))

	// 订阅中心 API
	mux.HandleFunc(baseURL+"/subscribe/config", authMiddleware(handleSubscribeConfigAPI))
	mux.HandleFunc(baseURL+"/subscribe/generate", authMiddleware(handleGenerateConfig))
	mux.HandleFunc(baseURL+"/subscribe/update/", authMiddleware(handleSubscribeUpdate))
	mux.HandleFunc(baseURL+"/subscribe/update-info/", authMiddleware(handleUpdateSubscriptionInfo))

	mux.HandleFunc(baseURL+"/providers/proxies/", authMiddleware(handleProviderProxies))

	// WebSocket 代理
	mux.HandleFunc(baseURL+"/traffic", wsProxyHandler("/traffic"))
	mux.HandleFunc(baseURL+"/memory", wsProxyHandler("/memory"))

	// HTTP 代理
	mux.HandleFunc(baseURL+"/version", authMiddleware(handleVersion))
	mux.HandleFunc(baseURL+"/configs", authMiddleware(handleConfigsAPI))
	mux.HandleFunc(baseURL+"/configs/raw", authMiddleware(handleConfigsRaw))
	mux.HandleFunc(baseURL+"/interfaces", authMiddleware(handleInterfaces))
	mux.HandleFunc(baseURL+"/configs/geo", authMiddleware(handleConfigsGeo))
	mux.HandleFunc(baseURL+"/providers/geo", authMiddleware(handleProvidersGeo))
	mux.HandleFunc(baseURL+"/cache/fakeip/flush", authMiddleware(handleFlushFakeIP))
	mux.HandleFunc(baseURL+"/cache/dns/flush", authMiddleware(handleFlushDNS))
	mux.HandleFunc(baseURL+"/dns/query", authMiddleware(handleDNSQuery))
	mux.HandleFunc(baseURL+"/restart", authMiddleware(handleRestart))
	mux.HandleFunc(baseURL+"/config/tproxy", authMiddleware(handleTproxyState))
	mux.HandleFunc(baseURL+"/config/tproxy/exceptions", authMiddleware(handleTproxyExceptions))
	mux.HandleFunc(baseURL+"/config/tproxy/proxy-local", authMiddleware(handleTproxyProxyLocal))
	mux.HandleFunc(baseURL+"/config/dns-failover", authMiddleware(handleDnsFailover))

	mux.HandleFunc(baseURL+"/ipinfo/local/v4", authMiddleware(handleLocalIPv4))
	mux.HandleFunc(baseURL+"/ipinfo/local/v6", authMiddleware(handleLocalIPv6))
	mux.HandleFunc(baseURL+"/ipinfo/proxy/v4", authMiddleware(handleProxyIPv4))
	mux.HandleFunc(baseURL+"/ipinfo/proxy/v6", authMiddleware(handleProxyIPv6))

	mux.HandleFunc(baseURL+"/delaytest/google", authMiddleware(handleDelayTestGoogle))
	mux.HandleFunc(baseURL+"/delaytest/youtube", authMiddleware(handleDelayTestYouTube))
	mux.HandleFunc(baseURL+"/delaytest/github", authMiddleware(handleDelayTestGitHub))
	mux.HandleFunc(baseURL+"/delaytest/baidu", authMiddleware(handleDelayTestBaidu))
	mux.HandleFunc(baseURL+"/delaytest/bilibili", authMiddleware(handleDelayTestBilibili))
	mux.HandleFunc(baseURL+"/delaytest/custom", authMiddleware(handleDelayTestCustom))

	// 代理 API
	mux.HandleFunc(baseURL+"/proxies", authMiddleware(handleProxies))
	mux.HandleFunc(baseURL+"/proxies/", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") || strings.Contains(r.URL.Path, "/delay?") {
			handleProxyDelay(w, r)
		} else {
			handleProxySwitch(w, r)
		}
	}))

	// nexusbox 版本更新
	mux.HandleFunc(baseURL+"/check-update", authMiddleware(handleCheckUpdate))
	mux.HandleFunc(baseURL+"/update-self", authMiddleware(handleSelfUpdate))

	// 质量分数
	mux.HandleFunc(baseURL+"/proxies/quality", authMiddleware(handleQualityScores))

	// 日志 WebSocket
	mux.HandleFunc(baseURL+"/logs", wsProxyHandler("/logs"))

	// 规则 API
	mux.HandleFunc(baseURL+"/rules", authMiddleware(handleRules))
	mux.HandleFunc(baseURL+"/rules/disable", authMiddleware(handleRulesDisable))
	mux.HandleFunc(baseURL+"/providers/rules", authMiddleware(handleRuleProviders))
	mux.HandleFunc(baseURL+"/providers/rules/", authMiddleware(handleUpdateRuleProvider))

	// 连接管理
	mux.HandleFunc(baseURL+"/connections", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handleConnectionsClose(w, r)
		} else {
			wsProxyHandler("/connections")(w, r)
		}
	}))
	mux.HandleFunc(baseURL+"/connections/", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handleConnectionsClose(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}))

	// 自动启动内核
	if !isCoreRunning() {
		if err := startCore(); err != nil {
			fmt.Printf("自动启动内核失败: %v\n", err)
		}
	} else {
		fmt.Println("内核已在运行，跳过自动启动")
	}

	// === 启动服务 ===
	if listener != nil {
		go func() {
			err := http.Serve(listener, mux)
			if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				fmt.Printf("Unix HTTP 服务错误: %v\n", err)
			}
		}()
	}
	if tcpListener != nil {
		go func() {
			fmt.Printf("NexusBox TCP 服务已启动，监听: %s\n", tcpAddr)
			if err := http.Serve(tcpListener, mux); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				fmt.Printf("TCP HTTP 服务错误: %v\n", err)
			}
		}()
	}

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Printf("收到退出信号，正在关闭 NexusBox...\n")
	stopAllTimers()
	disableTProxyRules()
	if isCoreRunning() {
		if err := stopCore(); err != nil {
			fmt.Printf("停止内核失败: %v\n", err)
		}
	} else {
		fmt.Printf("内核未运行，无需停止\n")
	}
	fmt.Printf("NexusBox 已安全退出\n")
}

// wsProxyHandler 保持不变
func wsProxyHandler(targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[WS] 升级失败 (路径 %s): %v", targetPath, err)
			return
		}
		defer conn.Close()

		subscribeMu.RLock()
		secret := subscribeConfig.PanelSecret
		subscribeMu.RUnlock()

		dialer := &websocket.Dialer{
			NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", coreSocket)
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		header := http.Header{}
		if secret != "" {
			header.Set("Authorization", "Bearer "+secret)
		}
		path := targetPath
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		coreConn, _, err := dialer.Dial("ws://localhost"+path, header)
		if err != nil {
			// 内核未运行或连接失败是预期情况，不记录日志
			return
		}
		defer coreConn.Close()

		errChan := make(chan error, 2)

		go func() {
			for {
				msgType, msg, err := coreConn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := conn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		go func() {
			for {
				msgType, msg, err := conn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}
				if err := coreConn.WriteMessage(msgType, msg); err != nil {
					errChan <- err
					return
				}
			}
		}()

		<-errChan
	}
}