# NexusBox

> 轻量级 Mihomo 内核管理面板 — 让裸核配置像订阅客户端一样简单

NexusBox 是一个**前后端一体化**的 Web 管理面板，为 [Mihomo](https://github.com/MetaCubeX/mihomo)（Clash.Meta）内核提供开箱即用的可视化管理。你只需要填入订阅链接，系统会自动生成完整的 `config.yaml`，无需手写一行规则。

---

## 截图预览

| 仪表盘 | 代理管理 | YAML 编辑器 |
|:---:|:---:|:---:|
| 实时速度、流量、内存监控 | 节点切换、延迟测速、分组筛选 | CodeMirror 6 语法高亮编辑器 |

---

## 快速安装

```bash
curl -fsSL https://raw.githubusercontent.com/Ladavian/NexusBox/main/install.sh | bash
```

安装完成后访问 **`http://<服务器IP>:18080`**，默认账户 **`admin / admin`**。

---

## 手动构建

```bash
# 前端
cd web && npm install && npm run build && cd ..

# 后端（带 Vue 前端）
go build -tags vue -ldflags="-s -w" -o nexusbox

# 启动
./nexusbox
```

---

## 功能

| 模块 | 说明 |
|------|------|
| **订阅中心** | 支持多订阅链接，自动生成 config.yaml，仅需填入链接即可使用 |
| **代理管理** | 节点延迟测速、分组切换、规则模式 (Rule/Global/Direct) |
| **规则管理** | 规则列表查看、启用/禁用、规则提供商更新 |
| **连接监控** | 实时活跃连接、已关闭历史、速率计算、搜索筛选 |
| **日志查看** | 实时日志流、级别过滤、暂停/自动滚动 |
| **YAML 编辑器** | CodeMirror 6 在线编辑，一键还原默认配置 |
| **TProxy** | nftables 透明代理，DNS 自动劫持 |
| **账户认证** | Cookie Session 登录，防止面板被扫描滥用 |
| **国际化** | 中文 / English 切换 |

---

## 目录结构

```
/opt/nexusbox/        # 面板程序
  └── nexusbox        # 主二进制
/opt/mihomo/          # Mihomo 内核
  └── mihomo
/opt/config/          # 配置文件
  ├── config.yaml     # 内核配置
  └── nexusbox.json   # 面板配置
```

---

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.26 标准库 + gorilla/websocket |
| 前端 | Vue 3 + TypeScript + Pinia + Vite + Tailwind CSS |
| 编辑器 | CodeMirror 6 + YAML 语法高亮 |
| 国际化 | vue-i18n（中文 / English） |

---

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `FLUXOR_ADDR` | `0.0.0.0:18080` | 面板监听地址 |
| `BASE_URL` | `/` | 反向代理路径前缀 |
| `CORE_BIN` | `/opt/mihomo/mihomo` | 内核二进制路径 |
| `CONFIG_TARGET` | `/opt/config/config.yaml` | 内核配置文件路径 |

---

## 致谢

本项目基于 [Fluxor](https://github.com/shuangji66/fluxor) 二次开发，感谢原作者的开源贡献。

## License

MIT
