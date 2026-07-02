# Lingqiao 项目接手与交接报告

更新时间：2026-06-28

## 1. 项目定位与架构总览

Lingqiao 是一个由 Windows 客户端、Go 后台、Web 管理端及一键发布体系组成的综合控制台平台。
目前架构已经进入 **V2 交付稳定化** 阶段。

核心业务流程：
1. 用户启动 `CX_LingQiao.exe` 客户端。
2. 客户端向服务端发起 HMAC 签名的安全激活与登录验证。
3. 服务端基于底层的 **SQLite 存储引擎** 完成会话核验，下发后续指令与动态加密的 `CefHook.dll` 模块。
4. 客户端将模块注入到指定目标进程中，并通过 Hook CEF/V8 生命周期注入 JS 脚本控制流。
5. 服务端全权负责设备鉴权、限流、更新包下发、脚本模块下发及审计日志记录，所有业务逻辑的错误响应均已统一为 **RFC 9457 (Problem Details)** 标准。

## 2. 当前代码库状态 (Baseline)

当前工作区已完成了一系列技术债清理与目录治理。**项目目前处于高度干净、可随时构建的基线状态。**

- 服务端已经从臃肿的根目录迁移到了 `server/` 独立目录中。
- 底层存储已从不具备并发能力的 `data.json` 全量写回，无损切换至 `modernc.org/sqlite` 数据源。
- 代码库内已**剔除所有历史遗留的预构建产物**（如 `server.exe`, `web-dist/` 等），新接手者不会被历史垃圾文件干扰。
- 不存在任何需要手动解决的 Git 冲突 (merge conflict)；可直接迁出分支进行特性开发。

## 3. 目录树职责

| 路径 | 核心职责 |
| --- | --- |
| `client/src/` | C++ 客户端、注入器、CEF Hook DLL、Qt UI 界面。 |
| `server/` | Go 后端。包含所有 HTTP API (`httpapi/`)，领域特性 (`features/`) 与 SQLite 持久化模块。 |
| `server/web/` | Vite + TypeScript 编写的 Web 管理端与代理控制台。 |
| `server/web-dist/`| 编译后前端文件的归宿。注意：源码中仅保留一个 `.gitkeep`，真正的网页文件需要开发者手动构建。 |
| `installer/` | WiX Toolset 安装包与 Bundle 的配置文件目录。 |
| `scripts/` | 发版流水线等基建脚本。 |
| `cef/` & `chromium/`| 客户端编译所依赖的原生库头文件。 |

## 4. 开发者构建指南

### 4.1 初次克隆的完整环境准备
由于项目采取了前后端分离再嵌入 (`go:embed`) 的部署模式，以及彻底的 Git 净化。在初次拉取代码后，**必须按照以下顺序构建**才能让服务端顺利跑起来：

```powershell
# 1. 构建 Web 静态资源 (必须执行，否则服务端无法挂载网页)
cd server/web
npm install
npm run build
cd ../..

# 2. 运行 Go 服务端测试与编译
cd server
go test ./...
go build -o ../dist/server/lingqiao-server.exe .
cd ..

# 3. 构建 C++ 客户端
# 注意：C++ 端的单独编译可以直接使用批处理脚本
.\build.bat
```

### 4.2 生产级自动化发布
当你开发完成一个新特性并准备推送生产环境时，切勿手动分步构建！我们已经提供了标准化的流水线脚本：

在根目录下执行：
```powershell
.\scripts\release.ps1
```
该脚本将依次完成：
1. `npm run build` 打包前端
2. `go build` 交叉编译含静态资源的服务端
3. `cmake --build` 编译 Release 版本的 C++ 客户端和 DLL
4. 调用 WiX Toolset 生成 `.msi` 安装包与 `.exe` Bundle

## 5. 待办的未来技术路线

项目基线已经稳定，未来的主干升级路线建议如下：
1. **服务端模块隔离**：将业务逻辑进一步按照 Clean Architecture 从入口移入 `internal/features/*`，分离 Handler、Service 和 Repository。
2. **客户端解耦**：拆分客户端巨型头文件（如 `ui/impl_chat.h`），分离 UI 表现层与核心业务控制器，增加防重打包校验。
3. **安全规范增强**：引入服务端证书热加载，以及针对所有发布包（Manifest、MSI、Bundle）的完整数字签名链条。
4. **管理控制平面**：实现细粒度的 RBAC 权限系统，增加版本分发大盘与代理提卡风控。

## Sessions Module

`server/internal/features/sessions` owns activation sessions, heartbeats, logout, forced logout, session validation, and expired-session cleanup.

Runtime flow:

1. Client signed request enters root compatibility handler or sessions handler.
2. Handler delegates to `sessions.Service`.
3. Service executes business rules and calls `sessions.Store`.
4. SQLite repository updates existing `cards` and `sessions` tables in one transaction.
5. Consumers such as chat, releases, payloads, scripts, and DLL download validate sessions through service methods.

## Agents Module

`server/internal/features/agents` owns agent authentication, registration, invite codes management, status setting, password resets, and account audits.

Runtime flow:

1. Admin or agent requests enter the admin handlers or agent handlers.
2. Handler validates parameters (e.g. password limits) and delegates to `agents.Service`.
3. Service executes business rules and coordinates transactional database operations (such as registration using atomic transaction on both `agents` and `invites` tables).
4. SQLite repository executes queries or updates on SQLite tables.
5. Verification logic (e.g. checking whether an agent is disabled) is queried from the SQLite database on each request to prevent session leaks.

Legacy `internal/agents` directory was completely removed. All domain models (`Account`) and invitation logic (`InviteCode`, `InviteService`) have been consolidated directly into `internal/features/agents`, ensuring zero legacy footprint and strict package encapsulation. Invite codes now persist in SQLite `invites` table; JSON invites are import-only compatibility data.

## 6. 数据存储与迁移策略 (Database & Migration)

服务端当前以 SQLite 为唯一的数据持久化层，所有核心域（Cards, Sessions, Agents, Audit, Blacklist, Invites）均已全量迁移至 `modernc.org/sqlite`。

**旧版 JSON 到 SQLite 的自动迁移：**
- 服务端启动时，会自动检测数据库是否为空。如果为空且存在旧版的 `data.json` / `cards.json` 等文件，服务会**自动、同步地将全部数据（包括黑名单、卡密、代理商账户、历史会话、审计日志）装载并写入 SQLite**。
- 迁移过程中内置了严格的冲突防御（例如：自动解决老版本中重复的代理商 `username` 冲突，为其分配唯一的 `_dupN` 后缀）。
- 任何无法修复的致命错误都会触发 Panic 从而阻止不一致的服务启动（Fail-Fast）。

**备份与回滚：**
- **备份**：SQLite 的全部数据存放于 `data/lingqiao.db` 中。备份仅需停止服务后直接复制该单文件即可。
- **回滚**：如遇毁灭性问题，可删掉 `lingqiao.db` 文件，确保旧版的 JSON 文件存在于对应目录中，再次重启服务便会重新触发历史迁移逻辑。

## 7. 测试与验证 (Testing & Verification)

项目中包含单元测试（Unit Tests）与端到端黑盒演练（E2E Tests）：

- **常规单测**：
  验证各领域模块的独立逻辑。
  ```powershell
  cd server
  go test ./... -count=1
  ```
- **真实 E2E 迁移演练**：
  在隔离沙盒内，完全模拟“构造极端脏数据的旧 JSON” -> “冷启动 Server 触发无损迁移” -> “HTTPS 接口调用拉起核心业务链” -> “底层 SQLite 数据行级断言”的全生命周期。
  ```powershell
  cd e2e_test
  go test -v .
  ```
