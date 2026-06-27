# Lingqiao 项目接手报告

更新时间：2026-06-27

## 1. 项目定位

Lingqiao 是一个 Windows 客户端 + Go 后台 + Web 管理端 + 安装包发布体系组成的项目。

客户端侧是 Qt5 写的 Windows GUI，主要负责卡密激活、会话保持、下载服务端下发的核心 DLL、选择目标 EXE 并完成 DLL 注入。DLL 被注入目标进程后，会在 CEF/Chromium 环境里安装 Hook，监听浏览器生命周期，并向页面注入 AutoExam JavaScript 脚本。服务端侧负责卡密、会话、聊天、公告、脚本模块、DLL 下发、更新发布、代理账号和后台管理。

整体可以按这条主线理解：

1. 用户打开 `CX_LingQiao.exe`。
2. 客户端通过 HMAC 签名请求服务端激活卡密。
3. 激活成功后保存 session，并下载加密的 `CefHook.dll`。
4. 用户选择目标程序，客户端创建目标进程并通过 `LoadLibraryW` 注入 DLL。
5. DLL 在目标进程内 Hook CEF/V8，并注入 AutoExam JS。
6. 客户端持续心跳、检查公告/更新、同步聊天室和状态。
7. 服务端后台用于发卡、禁用、踢下线、上传 DLL/脚本/更新包和查看审计。

## 2. 当前仓库状态

交接前必须注意：当前工作区不是干净状态，存在多个未解决冲突和大量已修改文件。新接手者不要直接在此状态上发布版本。

当前 `git status --short` 显示的关键风险：

- `UU README.md`
- `UU src/core/crypto.h`
- `UU src/hook/script_loader.h`
- `UU src/ui/impl_builder.h`
- 多个 `M`/`MM` 文件，覆盖服务端、客户端、UI、网络、Hook 等模块。
- `docs/V8_Signature_Research_Report.md` 显示为删除。

建议交接流程：

1. 先备份整个目录和服务端 `DATA_DIR`。
2. 新建一个干净分支或单独 clone。
3. 决定当前冲突文件以哪一侧为准，再逐个解决。
4. 解决后先跑服务端、前端和客户端构建验证。
5. 再考虑打包和发布。

本报告是独立新增文档，不依赖解决现有冲突。

## 3. 顶层目录说明

| 路径 | 作用 |
| --- | --- |
| `src/` | Windows C++ 客户端、注入器、CEF Hook DLL、Qt UI、网络/安全模块。 |
| `injector-server/` | Go 服务端，包含客户端 API、后台 API、代理后台、聊天、发卡、更新发布、脚本模块、payload 管理。 |
| `injector-server/web/` | Vite + TypeScript 源码，生成管理端和代理端前端页面。 |
| `injector-server/web-dist/` | 前端构建产物，通过 Go `embed` 打进服务端。 |
| `cef/`、`chromium/` | CEF/Chromium 头文件，供 Hook DLL 编译和 CEF C API 类型使用。 |
| `installer/` | WiX 和便携安装包构建脚本。 |
| `scripts/` | 项目辅助脚本，目前包含 AutoExam 多模型提供商回归脚本。 |
| `build/` | CMake/MSBuild 构建产物。不要手动维护，不应作为源码交接重点。 |
| `AutoExam.js`、`AutoExam.no-dry-run.js` | AutoExam 注入脚本源/变体，最终会被内嵌或由服务端脚本模块下发。 |
| `build.bat` | Windows 一键构建脚本，构建 `CefHook.dll` 和 `CX_LingQiao.exe`。 |

## 4. 构建产物

主要产物：

- `build/src/Release/CX_LingQiao.exe`：Qt 客户端主程序。
- `build/src/Release/CefHook.dll`：注入到目标进程的核心 Hook DLL。
- `build/src/Release/ManualDllInjector.exe`：命令行调试注入工具。
- `build/src/Release/ManualDllInjectorGui.exe`：GUI 调试注入工具。
- `dist/installer/LingqiaoSetup-<version>.exe`：安装包或便携安装器。
- `injector-server` 编译出的 Go 服务端二进制。

## 5. C++ 客户端结构

### 5.1 构建入口

| 文件 | 作用 |
| --- | --- |
| `CMakeLists.txt` | 顶层 CMake。设置 C++17、APP_VERSION、CRT 静态链接、安全链接参数，并引入 `src/`。 |
| `src/CMakeLists.txt` | 定义 `CefHook`、`Injector`、`ManualDllInjector`、`ManualDllInjectorGui`。处理 Qt 自动 MOC/RCC/UIC，构建后运行 `windeployqt`。 |
| `build.bat` | 调用 VS `vcvarsall.bat`，默认 Win32 平台，依次构建 DLL 和客户端 EXE。 |

默认 Qt 路径在 `src/CMakeLists.txt` 和 `build.bat` 中都是 `C:/Qt/5.15.2/msvc2019`。如果接手机器不同，需要修改 `QT_PREFIX_PATH` 或 CMake 参数。

### 5.2 客户端启动入口

主入口：`src/injector_qt.cpp`

启动时做这些事：

- 检查调试器，但当前逻辑只记录日志，不阻断 UI。
- 设置 DPI 和 Qt 全局属性。
- 调用 `InitSecrets()` 解密服务端 host、client id、HMAC secret、API path。
- 创建 `QApplication`，设置名称、版本和字体。
- 安装崩溃处理器，崩溃日志写入临时目录 `LingQiao_crash.log`。
- 支持 `--reinject --target "<path>"` 参数，用于 UAC 提权后的二次注入。
- 创建并显示 `MainWindow`。

### 5.3 主窗口和 UI

核心文件：

| 文件 | 作用 |
| --- | --- |
| `src/ui/main_window.h` | `MainWindow` 主类，持有主要状态、控件、定时器、事件处理和 UI 缩放。 |
| `src/ui/impl_builder.h` | 构建首页/社区页/卡密区/状态栏/API key 区/公告/更新提示等 UI。 |
| `src/ui/impl_actions.h` | 激活、浏览目标、注入等用户操作逻辑。 |
| `src/ui/impl_network.h` | DLL 下载、余额查询、心跳、公告获取、自动重连。 |
| `src/ui/impl_session.h` | 本地 session 保存、恢复、注销、离线待注销队列。 |
| `src/ui/impl_update.h` | 旧更新、新 release manifest 更新、安装包下载和替换。 |
| `src/ui/impl_chat.h` | 社区聊天室 UI、轮询、消息渲染、昵称、回复、表情。 |
| `src/ui/theme.h` | 全局 Qt 样式表、色彩、按钮样式、弹窗样式。 |
| `src/ui/title_bar.h` | 自定义无边框标题栏。 |
| `src/ui/inline_status.h` | 状态条中的圆点、文案、到期时间组件。 |
| `src/ui/animated_button.h` | 自定义按钮类。 |

重要状态：

- `m_activated`：是否已激活。
- `m_sessionToken`：服务端 session token。
- `m_machineID`：机器指纹。
- `m_cardExpiresAt`：卡密到期时间。
- `m_forceUpdateBlocked`：强制更新阻断状态。
- `m_currentProviderId`：当前 AI provider。
- `m_chatTimer` 和聊天相关字段：社区页轮询状态。

近期 UI 改动：

- 状态栏 `QFrame#sessionStatus` 现在直接在 `buildSessionStatusSection()` 里设置浅灰半透明背景：
  - `background: rgba(255, 255, 255, 0.35)`
  - `border: 1px solid rgba(200, 200, 210, 0.40)`
  - `border-radius: 8px`
  - `setContentsMargins(14, 7, 14, 7)`
- 这个样式是为了匹配社区聊天室消息区域的浅灰半透明效果。

### 5.4 激活和会话流程

客户端文件：

- `src/ui/impl_actions.h`
- `src/ui/impl_network.h`
- `src/ui/impl_session.h`
- `src/network/workers.h`
- `src/network/http_client.h`

服务端接口：

- `POST /api/v1/activate`
- `POST /api/v1/heartbeat`
- `POST /api/v1/deactivate`

激活请求包含：

- `client_id`
- `card`
- `machine_id`
- `fingerprint`
- `client_version`

所有客户端内部 API 请求都通过 HMAC 签名：

- `X-Client-ID`
- `X-HMAC-Signature`
- `X-Timestamp`
- `X-Nonce`

服务端要求 timestamp 在约 30 秒内，并用 nonce 防重放。

本地会话保存：

- 使用 `QSettings("LingQiao", "Injector")`。
- `session_token` 用机器指纹派生的简单 XOR 方案保存。
- `machine_id`、`card_expires_at`、`card_code` 也会保存。
- 启动后 `tryRestoreSession()` 会异步 heartbeat 验证，成功后恢复激活态、下载 DLL、启动聊天和更新检查。

### 5.5 网络和安全

| 文件 | 作用 |
| --- | --- |
| `src/core/config.h` | 解密配置、保存 `SERVER_HOST`、`CLIENT_ID`、HMAC key、API path、窗口尺寸、版本。 |
| `src/core/crypto.h` | HMAC、PBKDF2、AES-GCM、TLS 证书 pin、DPAPI 工具。 |
| `src/network/http_client.h` | WinHTTP 封装，支持 HMAC JSON POST、签名 GET、文件下载、外部 Bearer GET。 |
| `src/security/machine_fp.h` | 机器指纹采集：卷序列号、MAC、计算机名、用户、BIOS、CPU、主板、MachineGuid、硬盘。 |
| `src/security/dll_extractor.h` | 下载加密 DLL、AES-GCM 解密、PE 校验、写入随机临时目录。 |
| `src/security/antidebug.h` | 反调试检测。当前客户端启动和运行时只记录日志，不直接退出。 |

证书 pin：

- 客户端 `crypto.h` 内有 `LQ_CERT_PIN_SHA256_LITERAL`。
- DLL 内的 `script_loader.h` 引用同一个 pin。
- 如果服务端 TLS 证书更换，必须同步更新客户端 pin 并重新构建。

HMAC key：

- 服务端默认从 `HMAC_SECRET` 环境变量读取。
- 如果没有设置，会生成并保存到 `DATA_DIR/hmac_secret.key`。
- 客户端的 secret 通过 `encrypted_data.h` 内嵌和混淆。
- 两边 secret 不一致时，激活、心跳、DLL 下载、脚本下发都会失败。

### 5.6 DLL 下载和注入

DLL 下载：

- 客户端激活后调用 `downloadDllAsync()`。
- 实际下载函数为 `DownloadDll()`，请求 `/api/v1/dll`。
- 服务端读取 `DATA_DIR/CefHook.dll`，用客户端兼容密钥 AES-GCM 加密后返回。
- 客户端解密后检查 PE 头：`MZ`、`PE`、Optional Header magic。
- 成功后写入随机隐藏临时目录，并设置 `g_dllReady = true`。

注入流程：

1. 用户选择目标 EXE。
2. 客户端验证激活、DLL 是否 ready、API key 是否填写。
3. 把 AI provider、API key、session token 写入目标进程可继承的环境变量。
4. 使用 `CreateProcessW(..., CREATE_SUSPENDED, ...)` 启动目标。
5. `VirtualAllocEx` 分配远程内存。
6. `WriteProcessMemory` 写入 DLL 路径。
7. 优先 `NtCreateThreadEx` 调用 `LoadLibraryW`，失败则回退 `CreateRemoteThread`。
8. 注入完成后 `ResumeThread`，清理临时 DLL。

UAC 情况：

- 如果 `CreateProcessW` 返回 740，客户端通过 `ShellExecuteExW(..., runas, ...)` 提权重启自己。
- 新进程带 `--reinject --target "<path>"`，启动后自动继续注入。

### 5.7 Hook DLL 内部逻辑

主要文件：

| 文件 | 作用 |
| --- | --- |
| `src/hook/dllmain.cpp` | DLL 入口，进程附加时创建 `ThreadProc`，卸载时 Detach Hook。 |
| `src/hook/thread.cpp` | Hook 安装主逻辑，处理 CEF 回调、脚本注入、GPU 禁用、窗口截图保护 Hook。 |
| `src/hook/hook_engine.c` | MinHook 封装，提供 `HookAttach`/`HookDetach`/`HookFindFunction`。 |
| `src/hook/hook_safety.h` | Hook 辅助校验和 DLL 加密长度计算。 |
| `src/hook/v8_hooks.cpp` | x86 V8 签名 Hook，记录 V8 命中诊断。x64 下跳过。 |
| `src/hook/v8_signatures.cpp` | libcef.dll 模式扫描签名定义和诊断输出。 |
| `src/hook/pattern_scan.cpp` | IDA 风格签名解析和模块扫描。 |
| `src/hook/script_loader.h` | 从服务端 `/api/v1/script` 拉取当前 JS 脚本，失败时使用 DLL 内嵌 fallback。 |
| `src/hook/process_identity.*` | 目标进程识别辅助。 |

Hook 主线：

- Hook `kernel32!GetCommandLineW`，给 CEF 进程附加 `--disable-gpu --disable-gpu-compositing --use-angle=warp`。
- Hook `libcef!cef_browser_host_create_browser`。
- 从 CEF client vtable 探测 LoadHandler 和 LifeSpanHandler 的候选 slot。
- Hook `on_loading_state_change`、`on_load_end`、`on_after_created`、`on_before_close`。
- 页面主 frame `on_load_end` 后执行 JS 注入。
- 注入前把 API key、provider、model、base URL、vision 支持情况写到 `window.__AUTOEXAM_AI_CONFIG`。
- 优先从服务端取版本化脚本；失败时解密内嵌 JS 分片 fallback。

脚本拉取：

- 由 DLL 内 `FetchVersionedScript()` 发起。
- 请求 `/api/v1/script`，带 HMAC 和 `X-Session-Token`。
- 响应里包含 `content`、`sha256`、`version`。
- DLL 会本地计算 `content` SHA-256，匹配才执行。

## 6. Go 服务端结构

### 6.1 启动入口

主入口：`injector-server/main.go`

启动流程：

1. `config.Load()` 读取环境变量。
2. `ConfigureRuntime()` 设置 `DATA_DIR`、session TTL，并初始化 invite、update、release、auth 配置。
3. `NewJSONStorage()` 创建 JSON 存储。
4. `NewCardManager()` 加载卡密、session、黑名单、代理、审计。
5. 初始化 API、Admin、Agent、Payload handler。
6. 支持 CLI `--generate-card` 生成测试卡。
7. 构建两个 HTTPS Server：
   - Admin + Client API：默认 `:48901`
   - Agent panel：默认 `:38472`
8. 使用自签 TLS 证书，证书默认在 `certs/server.crt` 和 `certs/server.key`。

环境变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `PORT` | `48901` | 管理后台和客户端 API 端口。 |
| `AGENT_PORT` | `38472` | 代理后台端口。 |
| `DATA_DIR` | `data` | JSON/SQLite/文件持久化目录。 |
| `SESSION_TTL` | `4h` | 后台 session、客户端 session 等 TTL 默认值。 |
| `ADMIN_PASSWORD` | 自动生成 | 初始管理员密码；无持久 hash 时才使用。 |
| `HMAC_SECRET` | 自动生成 | 客户端 API HMAC secret。 |
| `ADMIN_ORIGIN` | 空 | 管理后台跨域允许源。 |
| `AGENT_ORIGIN` | 空 | 代理后台跨域允许源。 |
| `CERT_HOST` | `127.0.0.1` | 自签证书主机名/IP。 |
| `UPLOAD_KEY` | 自动生成 | payload 上传鉴权 key。 |
| `UPDATE_SIGNING_SEED_HEX` | 文件或自动生成 | 新 release manifest 的 Ed25519 seed。 |

### 6.2 路由分组

客户端 API：

| 路径 | 作用 |
| --- | --- |
| `POST /api/v1/activate` | 卡密激活，创建客户端 session。 |
| `POST /api/v1/heartbeat` | 心跳，续期 session，返回卡密到期和更新信息。 |
| `POST /api/v1/deactivate` | 注销 session。 |
| `GET /api/v1/announcement` | 公告和旧更新提示。 |
| `GET /api/v1/dll` | 下发加密 `CefHook.dll`。 |
| `GET /api/v1/script` | 下发当前 JS 脚本模块。 |
| `POST /api/v1/update/check` | 新 release 更新检查，返回签名 manifest。 |
| `POST /api/v1/update/events` | 客户端上报更新事件。 |
| `GET /api/v1/update/package/{id}` | 下载更新包。 |
| `POST /api/v1/chat/*` | 聊天 profile、messages、presence、react、send。 |
| `POST /api/v1/key-exchange` | payload key exchange。 |
| `GET /api/v1/payload/{id}/info` | payload 元信息。 |
| `GET /api/v1/payload/{id}/chunk/{n}` | payload chunk 下载。 |

管理后台 API：

- `/admin/api/login`
- `/admin/api/password`
- `/admin/api/dashboard`
- `/admin/api/cards`
- `/admin/api/card/generate`
- `/admin/api/card/batch-generate`
- `/admin/api/sessions`
- `/admin/api/force-logout`
- `/admin/api/blacklist`
- `/admin/api/audit`
- `/admin/api/announcement`
- `/admin/api/chat/*`
- `/admin/api/agents`
- `/admin/api/invites`
- `/admin/api/update/*`
- `/admin/api/releases/*`
- `/admin/api/script`
- `/admin/api/payloads`
- `/admin/api/payload/upload`

代理后台 API：

- `/api/register`
- `/api/login`
- `/api/dashboard`
- `/api/cards`
- `/api/card/generate`
- `/api/card/batch-generate`
- `/api/info`
- `/api/password`

### 6.3 鉴权和中间件

相关文件：

- `injector-server/auth.go`
- `injector-server/httputil.go`
- `injector-server/middleware.go`
- `injector-server/admin.go`
- `injector-server/agent.go`

客户端 API：

- HMAC 签名格式为 `timestamp|nonce|body`。
- GET 请求的 body 用 URL path 代替。
- timestamp 允许约 30 秒偏移。
- nonce 记录约 2 分钟，防重放。
- HMAC secret 经 PBKDF2 派生后使用。

后台/代理：

- Admin 登录成功后写 `admin_token` cookie，HttpOnly、Secure、SameSite Strict。
- Agent 登录成功后写 `agent_token` cookie。
- 密码支持 legacy SHA-256 到 bcrypt 的自动迁移。

中间件：

- 管理端 credentialed CORS 默认 same-origin，只有配置 `ADMIN_ORIGIN` 才允许跨域。
- 代理端同理使用 `AGENT_ORIGIN`。
- 上传接口 body limit 放宽到 200 MB，其余默认 1 MB。
- 设置 HSTS、X-Frame-Options、CSP 等安全响应头。

### 6.4 卡密、会话、黑名单、审计

核心文件：

- `injector-server/card_types.go`
- `injector-server/card_ops.go`
- `injector-server/card_helpers.go`
- `injector-server/internal/cards/*.go`

核心模型：

- `Card`
  - `Code`
  - `MachineID`
  - `AgentID`
  - `CreatedAt`
  - `ActivatedAt`
  - `ExpiresAt`
  - `Status`
  - `Note`
  - `MaxSessions`
- `Session`
  - `Token`
  - `CardCode`
  - `MachineID`
  - `Fingerprint`
  - `ClientVersion`
  - `CreatedAt`
  - `LastSeen`
  - `ExpiresAt`
  - `RemoteAddr`
- `BlacklistEntry`
- `AuditEntry`
- `Agent`

卡密规则：

- 卡密内部规范化会去掉 `-` 和空格，并转大写。
- 生成规则使用 Crockford Base32 字符集。
- 第一次激活时绑定机器，并把到期时间从“创建时间 + 时长”转换为“激活时间 + 时长”。
- `MaxSessions <= 1` 时不允许不同机器复用。
- 心跳会检查机器匹配、session 未过期、卡密未禁用/未过期。

### 6.5 持久化文件

默认在 `DATA_DIR` 下。

| 文件/目录 | 内容 |
| --- | --- |
| `data.json` | 卡密、客户端 session、黑名单、代理、审计主数据。 |
| `data.json.bak` | JSONStorage 保存时自动备份的上一版。 |
| `chat.json` | 聊天消息、昵称 profile、禁言记录。 |
| `admin_password.hash` | 管理员密码 hash。 |
| `admin_sessions.json` | 管理后台 session。 |
| `agent_sessions.json` | 代理后台 session。 |
| `agent_token_map.json` | agent token 到 agent ID 的映射。 |
| `hmac_secret.key` | 客户端 HMAC secret。必须和客户端内嵌 secret 匹配。 |
| `upload_key` | payload 上传密钥。 |
| `CefHook.dll` | 服务端下发给客户端的原始 DLL，响应时会加密。 |
| `scripts/` | JS 脚本模块、`index.json`、`active.json`。 |
| `updates/` | 旧更新机制文件、`index.json`、安装包/EXE。 |
| `releases.db` | 新 release 系统 SQLite 数据库。 |
| `update_manifest_seed.key` | Ed25519 manifest 签名 seed。 |
| `payloads.json` | payload 索引。 |
| `payloads/{payload_id}/chunk_0000.bin` | payload 分片数据。 |
| `audit_archive.json` | 超过内存审计上限后轮转出来的审计归档。 |

交接时必须备份：

- 整个 `DATA_DIR`
- `certs/server.crt` 和 `certs/server.key`
- 客户端源码中生成的 `encrypted_data.h`，如果它没有纳入仓库
- 服务端上正在使用的 `CefHook.dll`
- 发布用安装包和 release DB

### 6.6 聊天系统

核心文件：

- `injector-server/chat.go`
- `injector-server/admin_chat.go`
- `src/ui/impl_chat.h`

功能：

- 最近 24 小时或最多 200 条消息。
- 每个用户最多每 5 秒发送一条。
- 单条最多 300 个 Unicode rune。
- 支持系统消息、回复、表情反应、在线 presence、昵称、禁言。
- 客户端每 3 秒轮询消息和 presence。
- 管理员可以删除消息、发系统消息、禁言。

聊天身份：

- 由客户端 session 关联的机器/卡密派生 author ID。
- 昵称保存到 `chat.json` 的 profiles。

### 6.7 脚本模块

核心文件：

- `injector-server/script_module.go`
- `src/hook/script_loader.h`
- `src/hook/thread.cpp`

服务端脚本模块字段：

- `id`
- `name`
- `version`
- `sha256`
- `content`
- `size`
- `note`
- `active`
- `created_at`
- `updated_at`

管理端可以保存草稿、发布、启用、删除非 active 脚本。客户端 DLL 注入页面时优先请求 active 脚本，验证 SHA-256 后执行。如果服务端不可用或校验失败，DLL 使用内嵌 fallback JS。

注意：

- 单个脚本最大 2 MB。
- 脚本版本会 sanitize，非法字符替换为 `-`。
- `scripts/active.json` 兼容旧版 active 脚本。

### 6.8 更新发布体系

代码里并存两套更新机制。

旧机制：

- 文件：`injector-server/update.go`
- 数据：`DATA_DIR/updates/index.json` 和 current update metadata。
- 管理端上传 `.exe`，客户端通过公告/激活/心跳得到 `latest_version`、`download_url`、`sha256`。
- 客户端可下载 zip 或 exe 并替换。

新 release 机制：

- 文件：
  - `injector-server/release_service.go`
  - `injector-server/release_handlers.go`
  - `injector-server/internal/releases/*`
  - `src/ui/impl_update.h`
- 数据：
  - `DATA_DIR/releases.db`
  - `DATA_DIR/update_manifest_seed.key`
  - release package 文件
- 特点：
  - release 有 draft/published/paused/rolled_back 状态。
  - 支持 stable/beta/canary channel。
  - 支持 rollout percent。
  - 支持 allow/deny card、machine、agent targeting。
  - manifest 使用 Ed25519 签名，同时客户端还验证 HMAC。
  - 客户端可校验 package SHA-256。

客户端更新检查：

- `CheckEnterpriseUpdateAsync()` 请求 `/api/v1/update/check`。
- 服务端选择 release 后返回 `manifest_payload`、`manifest_hmac`、`signature`、`public_key`。
- 客户端当前会校验 HMAC，并检查 pinned public key（如果 CMake 设置了 `UPDATE_MANIFEST_PUBLIC_KEY_HEX`）。
- 随后进入 `handleInstallerUpdateCheck()` 和 `ApplyInstallerUpdate()`。

### 6.9 Payload 系统

核心文件：

- `injector-server/payload.go`
- `injector-server/payload_handlers.go`

功能：

- 管理端通过 upload key 上传加密 payload 分片。
- 服务端保存 payload 元信息和分片。
- 客户端/下游通过 key-exchange 获取加密 key material。
- 下载分片需要 HMAC。

重要字段：

- `payload_id`
- `aes_key`
- `hmac_key`
- `iv`
- `exe_hash`
- `chunk_count`
- `chunk_size`
- `total_size`
- `active`

### 6.10 管理端和代理端前端

路径：

- `injector-server/web/admin/src/main.ts`
- `injector-server/web/agent/src/main.ts`
- `injector-server/web/shared/src/*`
- `injector-server/web/vite.config.js`
- `injector-server/web-dist/*`

技术栈：

- TypeScript
- Vite
- 无大型前端框架，主要是 DOM helper + fetch API。

构建：

```powershell
cd injector-server\web
npm install
npm run typecheck
npm run build
```

构建产物写到 `injector-server/web-dist/`，由 `main.go` 通过 `//go:embed web-dist/*` 嵌入。

## 7. 安装包和发布

### 7.1 WiX 安装包

路径：

- `installer/Product.wxs`
- `installer/Bundle.wxs`
- `installer/build_installer.ps1`

命令：

```powershell
powershell -ExecutionPolicy Bypass -File installer\build_installer.ps1 -Version "3.0.0"
```

输出：

- `dist/installer/Lingqiao-<version>.msi`
- `dist/installer/LingqiaoSetup-<version>.exe`
- `dist/installer/release-info.json`

可选签名环境变量：

- `SIGNTOOL_PATH`
- `SIGN_CERT_SHA1`
- `SIGN_TIMESTAMP_URL`

### 7.2 便携安装器

路径：

- `installer/build_portable_setup.ps1`

命令：

```powershell
powershell -ExecutionPolicy Bypass -File installer\build_portable_setup.ps1 -Version "3.0.0"
```

脚本会把 `build/src/Release` 下的客户端和 Qt runtime 压入一个 C# 自解压安装器，安装到 `%LOCALAPPDATA%\Lingqiao` 并启动 `CX_LingQiao.exe`。

## 8. 开发和验证命令

### 8.1 客户端构建

推荐：

```powershell
.\build.bat
```

或手动：

```powershell
cmake -S . -B build -G "Visual Studio 17 2022" -A Win32 -DCMAKE_PREFIX_PATH="C:\Qt\5.15.2\msvc2019"
cmake --build build --config Release --target CefHook
cmake --build build --config Release --target Injector
```

单独构建客户端：

```powershell
cmake --build build --config Release --target Injector
```

如果出现 `LNK1104 cannot open CX_LingQiao.exe`，通常是客户端正在运行，关闭后重试。

### 8.2 服务端

```powershell
cd injector-server
$env:GOCACHE="$PWD\.gocache"
go test ./...
```

运行：

```powershell
cd injector-server
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

生成测试卡：

```powershell
cd injector-server
go run . --generate-card --duration 720 --max-sessions 1 --note "handoff-test"
```

### 8.3 前端

```powershell
cd injector-server\web
npm install
npm run check
```

### 8.4 AutoExam 脚本回归

```powershell
powershell -ExecutionPolicy Bypass -File scripts\autoexam_multimodal_provider_regression_test.ps1
```

## 9. 常见排障

### 9.1 客户端无法连接服务器

优先检查：

- 服务端是否启动在客户端内嵌的 `SERVER_HOST:SERVER_PORT`。
- `HMAC_SECRET` 是否和客户端 `encrypted_data.h` 内的 secret 一致。
- 客户端证书 pin 是否匹配服务端 `certs/server.crt`。
- 客户端机器时间是否和服务端相差超过约 30 秒。
- 服务端日志是否有 `[AUTH] HMAC verification failed`。

### 9.2 激活成功但 DLL 下载失败

检查：

- `DATA_DIR/CefHook.dll` 是否存在。
- 服务端 `/api/v1/dll` 是否能读取该文件。
- session token、machine id、card code 是否匹配。
- 客户端和服务端 DLL AES-GCM 派生 key 是否一致。
- 客户端是否报 “DLL 解密失败” 或 “PE 文件校验失败”。

### 9.3 注入失败

检查：

- 目标程序是否为 32 位。当前默认构建是 Win32，DLL 和目标架构必须匹配。
- 是否需要管理员权限。
- 目标进程是否已有注入或锁定。
- `g_dllReady` 是否为 true。
- 临时 DLL 是否被杀软删除。
- 客户端日志：临时目录下 `LingQiao_injector.log`。

### 9.4 页面没有注入脚本

检查：

- 目标进程是否加载 `libcef.dll`。
- `cef_browser_host_create_browser` 是否存在。
- Hook 日志里是否有 `LoadHandler hooked` 和 `AutoExam script injected`。
- 服务端 `/api/v1/script` 是否返回 active 脚本。
- active 脚本 `sha256` 是否与 content 匹配。
- fallback 内嵌 JS 是否正确生成到 `encrypted_data.h`。

### 9.5 更新不生效

检查：

- 旧机制：公告里的 `latest_version`、`min_version`、`force_update`、`download_url`、`sha256`。
- 新机制：release 是否 `published`，channel 是否匹配，rollout 是否命中，targeting 是否排除该机器/卡密。
- 客户端 `UPDATE_MANIFEST_PUBLIC_KEY_HEX` 是否和服务端 release public key 匹配。
- 包文件 SHA-256 是否匹配。

### 9.6 后台登录问题

检查：

- `DATA_DIR/admin_password.hash` 是否存在。
- 如果丢失且未设置 `ADMIN_PASSWORD`，服务端会生成随机初始密码，只在日志打印前几位。
- cookie 需要 HTTPS 和 Secure，使用 HTTP 访问会导致登录态异常。

## 10. 交接给下一位开发者的建议顺序

1. 先只跑服务端：`go run .`，确认后台能打开。
2. 配置固定 `HMAC_SECRET`、固定 TLS 证书，避免每次启动改变客户端兼容性。
3. 生成或确认客户端 `encrypted_data.h`，确保 host/client id/secret/path 与服务端一致。
4. 构建 `CefHook.dll` 并放到服务端 `DATA_DIR/CefHook.dll`。
5. 构建 `CX_LingQiao.exe`，用测试卡激活。
6. 验证 DLL 下载、选择目标、注入、JS 注入。
7. 验证后台发卡、禁用、踢下线、黑名单、公告、脚本发布。
8. 验证更新发布链路。
9. 最后打安装包和做发布。

## 11. 高风险点和维护注意事项

- 当前工作区有 merge conflict，必须先处理。
- 客户端是 Win32 构建，Hook DLL 与目标进程位数必须一致。
- TLS 证书 pin 和 HMAC secret 是客户端连通性的核心，任何一边变化都要同步发版。
- `DATA_DIR` 是事实数据库，发布/迁移前必须备份。
- 服务端 JSON 存储有 `.bak`，但不能替代完整备份。
- 新旧更新机制并存，后续建议统一到 release manifest 机制，减少维护复杂度。
- 脚本模块有服务端 active 版本和 DLL 内嵌 fallback，两者都要纳入发布流程。
- 管理端前端源码在 `web/`，运行服务端时实际使用 `web-dist/`，改前端后必须重新 build。
- `injector-server/web/node_modules/` 不应作为源码审查重点，也不建议提交。
- 如果换服务器域名/IP，至少要处理：TLS 证书、证书 pin、客户端内嵌 host、服务端 CORS/反代配置。
- 如果换发布密钥，客户端 `UPDATE_MANIFEST_PUBLIC_KEY_HEX` 也要更新。

## 12. 推荐后续整理工作

1. 解决当前 `UU` 冲突，恢复干净工作区。
2. 把 `encrypted_data.h`、生成脚本和密钥管理流程文档化，明确哪些不能入库。
3. 给服务端补一套 `_test.go`，当前仓库未发现 Go 测试文件。
4. 把 release 系统和旧 update 系统二选一，减少双路径。
5. 给客户端注入链路补最小 smoke test 或人工验收清单。
6. 给 `DATA_DIR` 设计备份/恢复脚本。
7. 给管理端增加“当前 DLL SHA-256 / 当前脚本 SHA-256 / 当前 release 公钥”诊断页。
8. 梳理 AutoExam 脚本的发布流程：源码、压缩/混淆、内嵌 fallback、服务端 active 脚本保持一致。

