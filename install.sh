#!/usr/bin/env bash
set -e

# ============================================================
# NexusBox 一键安装脚本
# 目录结构：
#   /opt/nexusbox/        - NexusBox 面板程序
#     ├── nexusbox        - 主程序
#     ├── var/          - 运行时数据 (pid/sock/log)
#     └── ui/           - 外置面板静态文件
#   /opt/mihomo/        - Mihomo 内核
#     └── mihomo        - 内核二进制
#   /opt/config/        - 配置文件
#     ├── config.yaml   - 内核配置
#     └── nexusbox.json   - 面板配置（订阅/账户）
# ============================================================

M_API="https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
F_API="https://api.github.com/repos/Ladavian/NexusBox/releases/latest"
INSTALL_DIR="/opt/nexusbox"
MIHOMO_DIR="/opt/mihomo"
CONFIG_DIR="/opt/config"
SERVICE="/etc/systemd/system/nexusbox.service"

msg(){ echo -e "\033[1;32m[INFO]\033[0m $*"; }
warn(){ echo -e "\033[1;33m[WARN]\033[0m $*"; }
err(){ echo -e "\033[1;31m[ERR ]\033[0m $*"; exit 1; }

[ "$EUID" -eq 0 ] || err "请用 root 权限运行"

# ---------- 架构检测 ----------
arch() {
 case "$(uname -m)" in
  x86_64|amd64)  echo "amd64" ;;
  aarch64|arm64) echo "arm64" ;;
  armv7l)        echo "armv7" ;;
  *) err "不支持的架构: $(uname -m)" ;;
 esac
}

# ---------- 安装依赖 ----------
install_deps() {
 msg "安装系统依赖..."
 if command -v apt-get &>/dev/null; then
  apt-get update -qq
  apt-get install -y -qq curl wget gzip ca-certificates nftables procps
 elif command -v yum &>/dev/null; then
  yum install -y -q curl wget gzip ca-certificates nftables procps-ng
 elif command -v apk &>/dev/null; then
  apk add --no-cache curl wget gzip ca-certificates nftables procps
 else
  warn "未识别包管理器，跳过依赖安装"
 fi
}

# ---------- 创建目录 ----------
create_dirs() {
 msg "创建目录结构..."
 mkdir -p "$INSTALL_DIR/var" "$INSTALL_DIR/ui/meta" "$INSTALL_DIR/ui/zash"
 mkdir -p "$MIHOMO_DIR"
 mkdir -p "$CONFIG_DIR"
}

# ---------- 下载 Mihomo 内核 ----------
install_mihomo() {
 ARCH=$(arch)
 VER=$(curl -fsSL "$M_API" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
 URL="https://github.com/MetaCubeX/mihomo/releases/download/${VER}/mihomo-linux-${ARCH}-${VER}.gz"

 msg "下载 Mihomo ${VER} (${ARCH})..."
 cd /tmp
 rm -f mihomo mihomo.gz
 curl -fSL --connect-timeout 30 --retry 3 "$URL" -o mihomo.gz
 gunzip -f mihomo.gz

 # 备份旧版本
 [ -f "$MIHOMO_DIR/mihomo" ] && cp "$MIHOMO_DIR/mihomo" "$MIHOMO_DIR/mihomo.bak" 2>/dev/null || true

 mv mihomo "$MIHOMO_DIR/mihomo"
 chmod +x "$MIHOMO_DIR/mihomo"
 msg "Mihomo 安装完成: $MIHOMO_DIR/mihomo"
}

# ---------- 下载 GEO 数据 ----------
install_geo() {
 msg "下载 GEO 数据库..."
 BASE="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
 for f in geoip.dat geosite.dat country.mmdb; do
  curl -fSL --connect-timeout 30 --retry 3 "$BASE/$f" -o "$CONFIG_DIR/$f" && msg "  $f OK" || warn "  $f 下载失败"
 done
}

# ---------- 安装 NexusBox ----------
install_nexusbox() {
 ARCH=$(arch)
 INSTALL_BIN="$INSTALL_DIR/nexusbox"

 # 尝试1：通过 GitHub API 获取最新版
 msg "获取最新版本信息..."
 VER=$(curl -fsSL --connect-timeout 15 "$F_API" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4 || echo "")
 
 if [ -n "$VER" ]; then
  BIN_NAME="nexusbox-${ARCH}"
  URL="https://github.com/Ladavian/NexusBox/releases/download/${VER}/${BIN_NAME}"
  msg "下载 NexusBox ${VER} (${ARCH})..."
  if curl -fSL --connect-timeout 60 --retry 3 "$URL" -o "$INSTALL_BIN"; then
   chmod +x "$INSTALL_BIN"
   msg "下载完成: $INSTALL_BIN"
   return
  fi
  warn "GitHub Release 下载失败，尝试镜像加速..."
 fi

 # 尝试2：ghproxy 镜像加速
 for MIRROR in "https://gh-proxy.com/" "https://gh-proxy.org/" "https://mirror.ghproxy.com/"; do
  if [ -z "$VER" ]; then
   VER=$(curl -fsSL --connect-timeout 15 "${MIRROR}${F_API}" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4 || echo "")
  fi
  if [ -n "$VER" ]; then
   BIN_NAME="nexusbox-${ARCH}"
   URL="${MIRROR}https://github.com/Ladavian/NexusBox/releases/download/${VER}/${BIN_NAME}"
   msg "通过镜像下载 ${VER}..."
   if curl -fSL --connect-timeout 60 --retry 2 "$URL" -o "$INSTALL_BIN"; then
    chmod +x "$INSTALL_BIN"
    msg "下载完成: $INSTALL_BIN"
    return
   fi
  fi
 done

 # 回退：从源码编译
 warn "预编译包下载失败，尝试从源码编译..."
 if command -v go &>/dev/null; then
  msg "编译 NexusBox (需要 Go + Node.js)..."
  BUILD_DIR="/tmp/nexusbox-build"
  rm -rf "$BUILD_DIR"
  git clone --depth 1 https://github.com/Ladavian/NexusBox.git "$BUILD_DIR" || {
   err "克隆仓库失败，请检查网络"
  }
  cd "$BUILD_DIR/web" && npm install --silent 2>/dev/null && npm run build 2>/dev/null || warn "前端构建失败，将编译纯后端版本"
  cd "$BUILD_DIR" && go build -tags vue -o "$INSTALL_BIN" . 2>/dev/null || go build -o "$INSTALL_BIN" . || {
   err "Go 编译失败，请手动安装"
  }
  chmod +x "$INSTALL_BIN"
  rm -rf "$BUILD_DIR"
  msg "编译完成: $INSTALL_BIN"
  return
 fi

 err "无法安装: 下载和编译均失败。请检查网络或手动安装"
}

# ---------- GEO 数据自动更新定时器 ----------
setup_geo_timer() {
 msg "配置 GEO 自动更新..."

 cat >/etc/systemd/system/nexusbox-geo.service <<'EOF'
[Unit]
Description=NexusBox GEO Database Update
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/bin/bash -c '\
 BASE="https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"; \
 for f in geoip.dat geosite.dat country.mmdb; do \
   curl -fSL --connect-timeout 30 --retry 3 "$BASE/$f" -o "/opt/config/$f" 2>/dev/null; \
 done'
EOF

 cat >/etc/systemd/system/nexusbox-geo.timer <<'EOF'
[Unit]
Description=Daily GEO update for NexusBox

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
EOF

 systemctl daemon-reload
 systemctl enable nexusbox-geo.timer 2>/dev/null || true
 systemctl start nexusbox-geo.timer 2>/dev/null || true
}

# ---------- 生成默认配置 ----------
generate_config() {
 msg "生成默认配置..."

 # 内核配置
 if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
  cat >"$CONFIG_DIR/config.yaml" <<'EOF'
mixed-port: 7890
allow-lan: true
mode: rule
log-level: info
ipv6: true
unified-delay: true
external-controller: '0.0.0.0:9090'
external-controller-unix: '/opt/nexusbox/var/core.sock'
secret: ''
external-ui: ui/meta
profile:
  store-selected: true
  store-fake-ip: true
dns:
  enable: true
  listen: 0.0.0.0:1053
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  nameserver:
    - https://223.5.5.5/dns-query
    - https://120.53.53.53/dns-query
EOF
  msg "  config.yaml 已生成"
 else
  msg "  config.yaml 已存在，跳过"
 fi

 # 面板配置
 if [ ! -f "$CONFIG_DIR/nexusbox.json" ]; then
  cat >"$CONFIG_DIR/nexusbox.json" <<'EOF'
{
  "proxy_port": 7890,
  "tproxy_port": 7898,
  "panel_port": 9090,
  "panel_secret": "",
  "username": "admin",
  "password": "admin",
  "rule_group": "base",
  "ui_panel": "metacubexd",
  "meta_backend_url": "",
  "mode": "merge",
  "subscriptions": []
}
EOF
  msg "  nexusbox.json 已生成（默认账户: admin / admin）"
 else
  msg "  nexusbox.json 已存在，跳过"
 fi
}

# ---------- 注册 systemd 服务 ----------
install_service() {
 msg "注册 systemd 服务..."

 cat >"$SERVICE" <<EOF
[Unit]
Description=NexusBox - Mihomo Management Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/nexusbox
ExecStop=/bin/kill -SIGTERM \$MAINPID
Restart=always
RestartSec=5

# 环境变量（默认值，可按需修改）
Environment="SOCKET_PATH=$INSTALL_DIR/var/app.sock"
Environment="FLUXOR_ADDR=0.0.0.0:18080"
Environment="BASE_URL=/"
Environment="FLUXOR_PID_FILE=$INSTALL_DIR/var/nexusbox.pid"
Environment="FLUXOR_BIN_DIR=$INSTALL_DIR/"
Environment="CORE_PID_FILE=$INSTALL_DIR/var/core.pid"
Environment="CORE_BIN=$MIHOMO_DIR/mihomo"
Environment="CORE_SOCKET=$INSTALL_DIR/var/core.sock"
Environment="CONFIG_TARGET=$CONFIG_DIR/config.yaml"
Environment="INFO_LOG_FILE=$INSTALL_DIR/var/info.log"
Environment="CORE_WORK_DIR=$CONFIG_DIR"
Environment="FLUXOR_CONFIG_FILE=$CONFIG_DIR/nexusbox.json"
Environment="META_DIR=$INSTALL_DIR/ui/meta"
Environment="ZASH_DIR=$INSTALL_DIR/ui/zash"

[Install]
WantedBy=multi-user.target
EOF

 systemctl daemon-reload
 systemctl enable nexusbox 2>/dev/null || true
 msg "服务已注册"
}

# ---------- 完整安装 ----------
install_all() {
 msg "========== NexusBox 一键安装 =========="
 install_deps
 create_dirs
 install_mihomo
 install_geo
 install_nexusbox
 generate_config
 install_service
 setup_geo_timer

 msg ""
 msg "===================================="
 msg "  NexusBox 安装完成！"
 msg "===================================="
 msg "  面板地址: http://<本机IP>:18080"
 msg "  默认账户: admin / admin"
 msg "  配置文件: $CONFIG_DIR/config.yaml"
 msg "  面板配置: $CONFIG_DIR/nexusbox.json"
 msg ""
 msg "  启动: systemctl start nexusbox"
 msg "  状态: systemctl status nexusbox"
 msg "  卸载: $0 uninstall"
 msg "===================================="

 read -rp "是否立即启动 NexusBox？[Y/n] " START
 case "$START" in
  [Nn]*) msg "跳过启动，请手动执行 systemctl start nexusbox" ;;
  *)
   systemctl start nexusbox 2>/dev/null || warn "启动失败，请检查日志"
   sleep 2
   systemctl --no-pager status nexusbox 2>/dev/null || true
   ;;
 esac
}

# ---------- 更新 ----------
do_update() {
 msg "更新 NexusBox + Mihomo..."
 install_mihomo
 install_nexusbox
 systemctl restart nexusbox 2>/dev/null || true
 msg "更新完成"
}

# ---------- 卸载 ----------
do_uninstall() {
 msg "卸载 NexusBox..."
 systemctl disable --now nexusbox 2>/dev/null || true
 systemctl disable --now nexusbox-geo.timer 2>/dev/null || true
 rm -f "$SERVICE" /etc/systemd/system/nexusbox-geo.service /etc/systemd/system/nexusbox-geo.timer
 systemctl daemon-reload
 rm -rf "$INSTALL_DIR"
 msg "NexusBox 已卸载（保留 $MIHOMO_DIR 和 $CONFIG_DIR）"
 msg "如需彻底清除: rm -rf $MIHOMO_DIR $CONFIG_DIR"
}

# ---------- 入口 ----------
case "${1:-install}" in
 install) install_all ;;
 update)  do_update ;;
 geo)     install_geo ;;
 start)   systemctl start nexusbox ;;
 stop)    systemctl stop nexusbox ;;
 restart) systemctl restart nexusbox ;;
 status)  systemctl status nexusbox ;;
 uninstall) do_uninstall ;;
 *)
  echo "用法: $0 {install|update|geo|start|stop|restart|status|uninstall}"
  echo ""
  echo "  install   - 完整安装 NexusBox + Mihomo 内核 + GEO 数据"
  echo "  update    - 更新 NexusBox 和 Mihomo 到最新版"
  echo "  geo       - 仅更新 GEO 数据库"
  echo "  start     - 启动服务"
  echo "  stop      - 停止服务"
  echo "  restart   - 重启服务"
  echo "  status    - 查看服务状态"
  echo "  uninstall - 卸载 NexusBox"
  ;;
esac
