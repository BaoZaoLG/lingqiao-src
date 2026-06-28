# Lingqiao V2 架构改造方�?
更新时间�?026-06-28

本文以当前已固定版本为基线，目标不是推倒重写，而是把现有项目从“工具式堆叠”整理成“模块边界清楚、可持续演进的平台型项目”�?
## 1. 当前核心问题

Lingqiao 已经包含 Windows 客户端、Hook DLL、Go 服务端、管理后台、代理后台、AutoExam 脚本、更新发布、安装包和运行时数据。实际复杂度已经接近一个平台，但代码组织仍保留早期工具项目的形态�?
主要问题�?
1. 服务端根目录 `main` 包承担太多职责，`CardManager` 成为事实上的中心对象�?2. 服务端旧更新机制、新 release manifest 机制、公告里的版本推送同时存在�?3. 客户�?`MainWindow` 同时承担 UI、会话、聊天、更新、注入、设置和网络协调�?4. Hook DLL、AutoExam 脚本、服务端脚本模块之间缺少明确的发布链路�?5. 生成物、运行时数据、源码和外部依赖边界不够醒目�?6. 测试体系偏弱，重构时缺少安全网�?
这些问题会让后续开发呈现“每加一个功能就补一块”的状态。V2 的重点是先建立边界，再逐步迁移�?
## 2. V2 目标

V2 架构目标�?
1. 每个子系统有清晰职责�?2. 服务端业务模块可以独立理解、测试和替换�?3. 客户�?UI 与业务控制逻辑分离�?4. 更新发布、脚本发布、DLL 发布走统一可追踪流程�?5. 运行时数据和密钥不混入源码�?6. 新功能优先进入新模块，旧代码只做兼容和迁移�?
非目标：

1. 不一次性重写整个项目�?2. 不为了目录漂亮而移动所有文件�?3. 不在没有测试的情况下删除旧逻辑�?4. 不改变当前可运行版本的核心行为�?
## 3. 业界标准与架构模式对�?
V2 应该对齐业界通用模式，但不盲目套微服务。Lingqiao 当前更适合先做“模块化单体”，也就是一个部署单元内有清晰的业务边界、接口、测试和数据访问规则。等业务规模、团队规模、部署规模真的需要拆分时，再从模块化单体演进到服务拆分�?
### 3.1 推荐采用的架构模�?
| 模式 | �?Lingqiao 中的落地 | 作用 |
| --- | --- | --- |
| Modular Monolith | 服务端按 `cards`、`sessions`、`agents`、`chat`、`releases`、`scripts`、`payloads` �?feature�?| 先建立模块边界，避免过早微服务�?|
| Clean Architecture | `handler -> service/usecase -> repository -> storage`，依赖从外向内�?| 业务规则不被 HTTP、JSON、数据库绑定�?|
| Hexagonal Architecture | �?HTTP、JSON、SQLite、文件系统、签名服务都作为 adapter�?| 以后换存储、换协议、加 CLI 不会重写业务�?|
| Domain-Driven Design Lite | �?bounded context 管住卡密、会话、代理、发布等领域概念�?| 让代码结构贴近业务语言，减少“大杂烩管理器”�?|
| Dependency Inversion | service 依赖接口，不依赖具体 JSON/SQLite 实现�?| 提高测试和替换能力�?|
| Repository Pattern | 所有数据读写经�?repository�?| 收敛 `os.ReadFile`、JSON、SQLite、迁移逻辑�?|
| ADR | 重要架构决定写进 `docs/adr/`�?| 让后续接手者知道为什么这样设计�?|

这里�?“DDD Lite�?是有意控制复杂度：不引入过多抽象术语，只保留 bounded context、entity、value object、domain service、repository 这些对当前项目真正有价值的概念�?
### 3.2 标准对齐清单

| 领域 | 推荐标准/规范 | 项目落地 |
| --- | --- | --- |
| API 描述 | OpenAPI 3.2.0 | �?`contracts/openapi.yaml` 描述 Client/Admin/Agent API；如工具链暂未支�?3.2，可阶段性降�?3.1.x�?|
| API 错误 | RFC 9457 Problem Details | 用标�?`type/title/status/detail/instance` 错误结构替换散落的中文错误字符串，并用扩展字段承载业务错误码�?|
| 版本管理 | Semantic Versioning 2.0.0 | 客户端、服务端、API、脚本和 release manifest 都遵�?`MAJOR.MINOR.PATCH`�?|
| 配置与部�?| Twelve-Factor App | 配置进环境变量，构建/发布/运行分离，日志走 stdout 或统一日志流�?|
| 应用安全 | OWASP ASVS | 管理端、代理端、Client API 至少�?ASVS L1/L2 做检查清单�?|
| Web 安全 | OWASP Cheat Sheet Series | CORS、CSP、认证、密码存储、上传、日志脱敏按清单执行�?|
| 发布安全 | 签名制品 + 可验�?manifest | 安装包、DLL、脚本和 manifest �?hash、签名、版本和渠道�?|
| 供应�?| SBOM: CycloneDX �?SPDX | 记录 Go、npm、CEF、Qt、MinHook 等依赖来源和版本�?|
| 日志审计 | Structured Logging + Audit Trail | 业务日志、审计日志、请�?ID 分离，敏感字段脱敏�?|
| 测试金字�?| Unit -> Integration -> E2E | service 层优先单测，HTTP 和构建流程做集成测试�?|

### 3.3 �?Lingqiao 的标准化原则

1. 一个业务概念只属于一�?bounded context。例如卡密归 `cards`，会话归 `sessions`，代理账号归 `agents`，发布归 `releases`�?2. Handler 不写业务规则，只做请求解析、鉴权入口、调�?usecase、返回响应�?3. Service 不知�?HTTP，也不直接读写文件�?4. Repository 不做业务决策，只负责持久化�?5. API 请求/响应必须�?OpenAPI 中声明�?6. 错误码必须稳定，中文提示只作为展示文案�?7. 所�?release artifact 必须可追踪来源、版本、hash 和签名�?8. 任何新架构决定必须写 ADR，避免半年后没人知道为什么这么做�?
### 3.4 建议新增的规范文�?
```text
docs/
├─ adr/
�? ├─ 0001-use-modular-monolith.md
�? ├─ 0002-use-clean-architecture-boundaries.md
�? ├─ 0003-use-release-manifest-as-only-update-protocol.md
�? └─ 0004-use-sqlite-as-next-storage.md
├─ api/
�? ├─ openapi.yaml
�? ├─ error-codes.md
�? └─ versioning.md
├─ security/
�? ├─ asvs-checklist.md
�? ├─ secrets-management.md
�? └─ release-signing.md
└─ operations/
   ├─ config.md
   ├─ backup-restore.md
   └─ release-runbook.md
```

ADR 模板�?
```text
# ADR-0001: 使用模块化单体作�?V2 服务端架�?
### 状�?Accepted

### 背景
当前项目部署规模和团队规模不需要微服务，但需要清晰模块边界�?
### 决策
服务端按业务 feature 拆分模块，保持单进程部署�?
### 后果
优点：部署简单、边界清楚、测试容易�?代价：模块边界需要代码审查持续维护�?```

### 3.5 学习顺序

如果你想借这个项目系统学习标准化架构，建议按这个顺序�?
1. Modular Monolith：先学如何在一个项目里建立边界�?2. Clean Architecture / Hexagonal Architecture：再学依赖方向和 adapter 思维�?3. DDD Lite：学�?bounded context、entity、value object、repository�?4. OpenAPI + RFC 9457：学�?API 契约和标准错误模型�?5. Twelve-Factor：学习配置、部署、日志和发布分离�?6. OWASP ASVS：学习安全需求如何变成可检查清单�?7. SemVer + Release Manifest：学习版本兼容和可验证发布�?8. ADR：学习如何记录架构决策，而不是只留下代码结果�?
参考入口：

- OpenAPI Specification: https://spec.openapis.org/oas/latest.html
- RFC 9457 Problem Details: https://www.rfc-editor.org/rfc/rfc9457.html
- Semantic Versioning: https://semver.org/
- Twelve-Factor App: https://12factor.net/
- OWASP ASVS: https://owasp.org/www-project-application-security-verification-standard/

## 4. 推荐顶层结构

长期目标结构�?
```text
Lingqiao/
├─ apps/
�? ├─ client-qt/          # Windows Qt 客户�?�? ├─ hook-runtime/       # CefHook.dll
�? ├─ server/             # Go 服务�?�? ├─ admin-web/          # 管理后台源码
�? └─ agent-web/          # 代理后台源码
├─ packages/
�? ├─ autoexam/           # AutoExam 脚本源码、构建、测�?�? ├─ contracts/          # API 契约、错误码、版本策�?�? └─ release-tools/      # 打包、签名、manifest、发布工�?├─ docs/
�? ├─ PROJECT_HANDOFF_REPORT.md
�? └─ ARCHITECTURE_V2_PROPOSAL.md
├─ installer/
├─ scripts/
└─ data/                  # 运行时数据，不入�?```

短期不需要马上移动到这个结构。建议先在现有目录内完成模块化，再做顶层目录迁移�?
当前�?V2 的映射：

| 当前路径 | V2 归属 | 说明 |
| --- | --- | --- |
| `src/ui/` | `apps/client-qt/ui` | Qt 界面和控件�?|
| `src/network/` | `apps/client-qt/services/api` | 客户�?API 通信�?|
| `src/security/` | `apps/client-qt/platform/security` | 机器码、DLL 下载、安全辅助�?|
| `src/hook/` | `apps/hook-runtime` | CEF/V8 Hook 和脚本注入�?|
| `src/injector/` | `apps/client-qt/injection` | 注入实现和调试工具�?|
| `server/` | `apps/server` | 服务端�?|
| `server/web/admin` | `apps/admin-web` | 管理后台�?|
| `server/web/agent` | `apps/agent-web` | 代理后台�?|
| `AutoExam.js` | `packages/autoexam` | 脚本源码�?|
| `installer/` | `packages/release-tools` 或保留顶�?| 安装包构建�?|

## 5. 服务端目标架�?
### 5.1 目标目录

```text
server/
├─ cmd/server/main.go
├─ internal/bootstrap/
�? ├─ app.go
�? ├─ routes.go
�? └─ server.go
├─ internal/features/
�? ├─ cards/
�? �? ├─ model.go
�? �? ├─ service.go
�? �? ├─ repository.go
�? �? ├─ handler_admin.go
�? �? └─ handler_client.go
�? ├─ sessions/
�? ├─ agents/
�? ├─ chat/
�? ├─ announcements/
�? ├─ releases/
�? ├─ scripts/
�? └─ payloads/
├─ internal/platform/
�? ├─ auth/
�? ├─ crypto/
�? ├─ httpapi/
�? ├─ storage/
�? └─ audit/
└─ web/
```

### 5.2 服务端分�?
服务端建议使用四层：

| �?| 职责 | 不应该做 |
| --- | --- | --- |
| `transport/http` | 解析请求、校验方法、返�?JSON | 不写业务规则�?|
| `feature/service` | 业务规则、状态流转、权限判�?| 不直接操�?HTTP�?|
| `repository` | 数据读写、事务、迁�?| 不写 UI/API 返回结构�?|
| `platform` | 存储、认证、配置、审计、日�?| 不依赖具体业务模块�?|

这样可以让每个功能模块独立测试。例如卡密激活流程只测试 `cards.Service.Activate()`，不必启�?HTTP 服务�?
### 5.3 逐步替换 `CardManager`

当前 `CardManager` 同时管理卡密、会话、黑名单、代理、审计和存储。建议拆成：

```text
CardService       # 发卡、延期、禁用、解绑、查�?SessionService    # 激活、心跳、下线、会话过�?MachineService    # 机器绑定、风险机器、黑名单
AgentService      # 代理账号、邀请码、代理卡�?AuditService      # 操作日志
```

迁移原则�?
1. 先为现有 `CardManager` 补测试�?2. 新建 service，但先复用现�?JSON 数据结构�?3. 每迁移一个方法，就让�?handler 调用�?service�?4. `CardManager` 最后退化成兼容适配层�?5. 等所有调用点迁走，再删除适配层�?
### 5.4 统一更新发布系统

当前存在三条更新线：

1. 公告里的 `latest_version`、`download_url`、`force_update`�?2. �?`update.go` �?`updates/index.json`�?3. �?release manifest + package 下载�?
V2 建议只保�?release manifest�?
```text
Release
├─ Version
├─ Channel
├─ Status
├─ RolloutPercent
├─ ForceUpdate
├─ Packages
�? ├─ kind: exe/msi/bundle
�? ├─ sha256
�? ├─ size
�? └─ url
└─ Signature
```

公告只负责展示内容，不再承担更新协议。客户端只调�?`/api/v1/update/check`，并只信任签�?manifest�?
## 6. 客户端目标架�?
### 6.1 当前问题

当前客户端用 `MainWindow` 包含多个 `impl_*.h`，表面拆分了文件，但状态仍集中在一个类里。后续修改聊天、更新、状态栏、会话、注入时，容易互相影响�?
### 6.2 目标分层

```text
src/
├─ app/
�? ├─ main.cpp
�? └─ app_context.h
├─ ui/
�? ├─ main_window.h/.cpp
�? ├─ widgets/
�? └─ theme/
├─ controllers/
�? ├─ session_controller.h/.cpp
�? ├─ chat_controller.h/.cpp
�? ├─ update_controller.h/.cpp
�? ├─ injection_controller.h/.cpp
�? └─ settings_controller.h/.cpp
├─ services/
�? ├─ api_client.h/.cpp
�? ├─ update_service.h/.cpp
�? ├─ dll_service.h/.cpp
�? └─ machine_fingerprint_service.h/.cpp
├─ platform/
�? ├─ winhttp_client.h/.cpp
�? ├─ dpapi.h/.cpp
�? └─ process_launcher.h/.cpp
└─ injection/
   └─ manual_injector_core.h/.cpp
```

### 6.3 职责边界

| 模块 | 职责 |
| --- | --- |
| `MainWindow` | 创建界面、展示状态、连接按钮事件�?|
| `SessionController` | 卡密激活、恢复会话、心跳、过期处理�?|
| `ChatController` | 聊天消息、昵称、回复、在线状态�?|
| `UpdateController` | 检查更新、强制更新锁定、下载和启动安装包�?|
| `InjectionController` | 目标 EXE 选择、DLL 准备、注入流程�?|
| `ApiClient` | 与服务端通信，封装路径、签名、错误�?|
| `SettingsController` | 本地配置、API Key、模型供应商�?|

客户端优先迁移顺序：

1. `ApiClient`
2. `UpdateController`
3. `ChatController`
4. `SessionController`
5. `InjectionController`
6. 精简 `MainWindow`

原因：API 和更新最适合独立测试；聊天和会话状态较多，但边界清楚；注入涉及 Windows API 和风险更高，最后迁移�?
## 7. Hook Runtime �?AutoExam

### 7.1 目标

Hook DLL 应该只负责运行时能力�?
1. 找到 CEF/V8�?2. 安装 Hook�?3. 获取脚本�?4. 注入脚本�?5. 输出诊断�?
AutoExam 应该是单独包�?
```text
packages/autoexam/
├─ src/autoexam.js
├─ tests/
├─ build/
�? ├─ bundle.js
�? └─ embedded_header.h
└─ README.md
```

服务端脚本模块、客户端内嵌 fallback、回归测试都应该来自同一�?`src/autoexam.js`�?
### 7.2 发布链路

推荐链路�?
```text
AutoExam source
  -> lint/test
  -> bundle/minify
  -> generate embedded fallback header
  -> upload script module to server
  -> release package references script version
```

这样可以避免“本�?JS、DLL 内嵌 JS、服务端 active JS 三者不一致”�?
## 8. 数据与存�?
### 8.1 短期

短期继续使用 JSON 文件，降低改造风险。但要把数据读写收敛�?repository，不要让业务代码到处直接 `os.ReadFile`�?
### 8.2 中期

推荐迁移�?SQLite�?
1. 单文件部署，适合当前项目规模�?2. 支持事务，减�?JSON 并发写风险�?3. 更容易做审计、查询、分页和统计�?4. Go 已经引入 `modernc.org/sqlite`，release 模块也已使用 SQLite 思路�?
### 8.3 长期

如果未来有多实例部署、远程管理、多管理员同时操作，再考虑 PostgreSQL�?
迁移策略�?
1. 定义 repository 接口�?2. 先实�?JSON repository�?3. 再实�?SQLite repository�?4. 写一次性迁移命令�?5. 保留 JSON 只读导入能力�?
## 9. API 契约

建议新增 `packages/contracts` �?`docs/api`�?
```text
contracts/
├─ client-api.md
├─ admin-api.md
├─ agent-api.md
├─ errors.md
└─ versioning.md
```

重点约定�?
1. 成功响应保持统一业务结构�?2. 错误响应使用 RFC 9457 Problem Details，不只靠中文错误消息判断�?3. 客户�?API 有版本号�?4. 更新 manifest 有签名和 schema 版本�?5. Admin/Agent API �?Client API 分开演进�?
推荐成功响应格式�?
```json
{
  "status": "ok",
  "data": {},
  "meta": {}
}
```

推荐错误响应格式�?
```json
{
  "type": "https://lingqiao.example/problems/card-expired",
  "title": "Card expired",
  "status": 403,
  "detail": "卡密已过�?,
  "instance": "/api/v1/activate",
  "code": "CARD_EXPIRED",
  "request_id": "req_..."
}
```

## 10. 构建与发�?
推荐把发布流程固定成一条命令链�?
```text
1. npm run check
2. go test ./...
3. cmake --build build --config Release --target Injector
4. cmake --build build --config Release --target CefHook
5. build AutoExam bundle
6. generate encrypted headers
7. build installer
8. sign artifacts
9. create release manifest
10. upload package
```

未来可以把这些步骤放�?CI。即使暂时不用云 CI，也建议做本�?`scripts/release.ps1`�?
## 11. 测试策略

优先补这些测试：

| 模块 | 测试重点 |
| --- | --- |
| 卡密 | 生成、激活、过期、延期、禁用、解绑�?|
| 会话 | 心跳、过期、并发数量、踢下线�?|
| 代理 | 注册、邀请码、代理发卡、密码变更�?|
| 更新 | release 选择、强制更新、灰度比例、签名校验�?|
| 脚本 | active 脚本迁移、版本切换、fallback 一致性�?|
| 客户�?| URL 解析、manifest 验签、配置迁移�?|
| AutoExam | 题目识别、图片题、多模型 provider、缓存键�?|

重构前至少先补服务端 service 层测试。客户端可以先从纯函数和�?UI 服务测起�?
## 12. 分阶段迁移计�?
### 阶段 0：冻结基�?
目标：保证当前版本可回退�?
任务�?
1. 保持当前提交作为稳定基线�?2. 不直接删除旧代码�?3. 建立 `ARCHITECTURE_V2_PROPOSAL.md`�?4. 给后续迁移建分支�?
验收�?
1. 当前版本能构建�?2. 文档清楚说明改造方向�?
### 阶段 1：服务端安全�?
目标：先让服务端可测�?
任务�?
1. 给卡密生命周期补 Go 测试�?2. �?release selector 补测试�?3. �?admin/agent 登录和权限补测试�?4. 统一 `writeJSON/writeError` �?`internal/httpapi` 的使用方式�?
验收�?
1. `go test ./...` 有真实测试覆盖�?2. 后续拆模块时能发现行为变化�?
### 阶段 2：统一更新系统

目标：减少双路径维护�?
任务�?
1. 将客户端更新检查统一�?release manifest�?2. 后台只保�?release 管理入口�?3. 公告去掉更新协议职责，只保留展示�?4. �?`update.go` 保留迁移读取能力，不再作为主路径�?
验收�?
1. 新版本发布只需要创�?release 和上�?package�?2. 客户端只信任签名 manifest�?
### 阶段 3：服务端模块�?
目标：拆掉中心大对象�?
任务�?
1. 新建 feature service 目录�?2. 先迁�?cards/session�?3. 再迁�?agents/chat�?4. 最后迁�?scripts/payloads/releases�?5. 根目�?handler 逐步移动�?feature handler�?
验收�?
1. `CardManager` 不再是所有业务的入口�?2. 每个 feature 有独�?service 测试�?
### 阶段 4：客户端控制器拆�?
目标：让 `MainWindow` 回到 UI 职责�?
任务�?
1. 抽出 `ApiClient`�?2. 抽出 `UpdateController`�?3. 抽出 `ChatController`�?4. 抽出 `SessionController`�?5. 抽出 `InjectionController`�?
验收�?
1. `MainWindow` 不再直接处理业务请求细节�?2. UI 改动不会影响更新和会话逻辑�?
### 阶段 5：AutoExam 发布链路

目标：统一脚本来源�?
任务�?
1. 合并 `AutoExam.js` �?`AutoExam.no-dry-run.js`�?2. 建立 AutoExam 构建脚本�?3. 内嵌 fallback、服务端 active 脚本、回归测试都来自同一源码�?4. 文档化脚本版本和发布流程�?
验收�?
1. 不再手工维护多个脚本副本�?2. 每个 release 可以追踪对应脚本版本�?
## 13. 推荐优先�?
最高优先级�?
1. 统一更新系统�?2. 服务端补测试�?3. 拆服务端 `CardManager`�?
中优先级�?
1. 客户�?`ApiClient` �?`UpdateController`�?2. AutoExam 单源码链路�?3. Admin 前端模块化�?
低优先级�?
1. 顶层目录大迁移�?2. 完整 UI 组件库化�?3. PostgreSQL 多实例部署�?
## 14. 不建议做的事

1. 不建议马上全量重写�?2. 不建议同时迁移服务端、客户端、前端和脚本�?3. 不建议在没有测试的情况下删除旧更新逻辑�?4. 不建议为了“目录好看”先大规模移动文件�?5. 不建议继续让公告承担更新协议�?6. 不建议继续新增功能到 `MainWindow` �?`CardManager`�?
## 15. 最终判�?
Lingqiao 当前不是不能维护，而是缺少平台化边界。最好的改造方式是�?
```text
先补安全�?再收敛双路径
再拆中心对象
最后整理目�?```

短期不要追求一次性变漂亮。只要接下来所有新功能都进入清晰模块，旧逻辑逐步退场，项目会从“修修补补”慢慢恢复成一套有主干的系统�?
