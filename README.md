# 🚀 NexusBox

> 轻量级 Mihomo (Clash.Meta) 内核管理面板 | 基于 [Fluxor](https://github.com/shuangji66/fluxor) 二次开发

NexusBox 是一个前后端一体化的 Web 管理面板，专为 Mihomo 内核设计。在 Fluxor 的基础上新增了 **账户认证**、**YAML 在线编辑器** 等实用功能，让裸核管理更加安全便捷。

---

## ✨ 新特性（vs Fluxor）

- 🔐 **账密登录认证** — Cookie Session 机制，防止面板被外网扫描滥用
- 📝 **YAML 在线编辑器** — Web 端直接查看/编辑 `config.yaml`，保存即热重载
- 🎨 **精简主题** — 浅色 / 深色 / 跟随系统
- 🔓 **解除互斥** — TUN 与 TProxy 可同时启用
- 📦 **一键安装脚本** — 自动拉取 Mihomo 内核 + 注册 systemd 服务

---

## 📂 目录结构

```
/opt/nexusbox/        # NexusBox 面板程序
  ├── nexusbox        # 主二进制
  └── var/            # 运行时数据
/opt/mihomo/          # Mihomo 内核
  └── mihomo
/opt/config/          # 配置文件
  ├── config.yaml     # 内核配置
  └── nexusbox.json   # 面板配置（订阅/账户）
```

---

## 🚀 快速安装

```bash
curl -fsSL https://raw.githubusercontent.com/Ladavian/NexusBox/main/install.sh | bash
```

安装后访问 **`http://<服务器IP>:18080`**

**默认账户：`admin` / `admin`**

---

## 🔧 手动构建

```bash
# 1. 前端
cd web && npm install && npm run build && cd ..

# 2. 后端（嵌入 Vue 前端）
go build -tags vue -o nexusbox

# 3. 启动
./nexusbox
```

---

## 🧩 核心特性

- **内核生命周期管理**：启动、停止、状态查询、配置热重载
- **订阅中心**：订阅链接的 CRUD 管理，自动生成 `config.yaml` 并应用
- **实时监控**：WebSocket 实时推送上/下载速度、内存、日志流、连接历史
- **透明代理 (TProxy)**：nftables 透明代理，端口/源地址例外过滤
- **双前端**：Vue 3 (推荐) + 原生 Vanilla JS
- **YAML 编辑器**：暗色终端风格，保存即热重载
- **登录认证**：Cookie Session，24h 有效期

---

## 🛠 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.26 (标准库) + gorilla/websocket |
| 前端 | Vue 3 + TypeScript + Pinia + Vite + Tailwind CSS |
| 国际化 | vue-i18n (中文/English) |

---

## 🙏 致敬

本项目基于 [**shuangji66/fluxor**](https://github.com/shuangji66/fluxor) 二次开发。感谢原作者提供的优秀面板框架！

---

## 📄 License

MIT
