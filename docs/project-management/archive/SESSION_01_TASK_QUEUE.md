# SESSION 01 Task Queue

会话职责：项目总控、全局架构审计、任务队列协调、Java 后端迁移总方案。

生成规则：本队列先于正式修改阶段生成；任务均绑定到具体文件、目录、配置、入口、依赖、调用链、构建链路或语义范围；业务代码大规模修改不属于本会话默认范围。

任务总数：1200

## S01-0001
- 文件/目录范围：`.`
- 检查目标：盘点根目录一级目录与文件，确认源码、构建产物、运行数据、文档的归属边界
- 潜在问题：根目录存在 build/build64/dist/data/certs/server.log 等运行或构建产物，可能污染源码边界
- 修改策略：在审计地图中标记源码目录、生成目录、运行数据目录；仅对文档和全局配置提出收敛策略
- 验证方式：Get-ChildItem -Force 与 git status --short 对照
- 完成状态：已完成

## S01-0002
- 文件/目录范围：`README.md`
- 检查目标：核对项目定位、技术栈、运行入口和测试命令是否覆盖当前实际结构
- 潜在问题：README 可能遗漏 server/web、e2e_test、installer、packages/autoexam 等入口
- 修改策略：补充文档建议到审计报告，避免直接改业务代码
- 验证方式：读取 README.md 并与 go.mod/package.json/CMakeLists/scripts/check.ps1 对照
- 完成状态：已完成

## S01-0003
- 文件/目录范围：`.gitignore`
- 检查目标：确认构建产物、运行数据库、证书、日志、Go cache、前端输出是否全部排除
- 潜在问题：git status 显示 data、server 可执行文件、.superpowers 等未跟踪项，忽略规则可能不足
- 修改策略：列入全局风险并建议最小 .gitignore 调整
- 验证方式：git status --short 与 .gitignore 内容对照
- 完成状态：已完成

## S01-0004
- 文件/目录范围：`build.bat`
- 检查目标：核对根目录 Windows CMake 构建入口是否仍匹配 client/CMakeLists.txt 和 README 命令
- 潜在问题：脚本可能固定 Win32/Release 或路径，和 build64/x64 产物并存导致架构混乱
- 修改策略：记录构建链路入口和需要客户端会话处理的具体问题
- 验证方式：读取 build.bat 并对照 client/CMakeLists.txt
- 完成状态：已完成

## S01-0005
- 文件/目录范围：`scripts/check.ps1`
- 检查目标：核对统一质量门禁是否串联 web typecheck/build、Go test、CMake configure
- 潜在问题：脚本使用 build 目录并可能覆盖现有构建缓存；C++ 仅配置不编译测试
- 修改策略：在风险登记中标记验证覆盖不足，必要时建议拆分输出目录
- 验证方式：运行或静态检查 scripts/check.ps1
- 完成状态：已完成

## S01-0006
- 文件/目录范围：`server/go.mod`
- 检查目标：核对 Go 模块名、Go 版本、SQLite/crypto 依赖是否适合迁移 Java
- 潜在问题：go 1.25.0 可能高于部分 CI/开发机，modernc sqlite 迁移到 Java JDBC 需要重新评估
- 修改策略：纳入 Java 迁移总方案的依赖映射
- 验证方式：go test ./...
- 完成状态：已完成

## S01-0007
- 文件/目录范围：`server/main.go`
- 检查目标：标记 Go 后端总入口、服务组装、路由注册、任务调度和优雅关闭入口
- 潜在问题：main.go 同时承担 DI、迁移、路由、TLS、后台任务，Java 迁移时组合根过大
- 修改策略：制定 Spring Boot composition root 拆分计划，不直接改业务代码
- 验证方式：rg HandleFunc/NewHandler/go func/ListenAndServeTLS server/main.go
- 完成状态：已完成

## S01-0008
- 文件/目录范围：`server/internal/config/config.go`
- 检查目标：标记环境变量入口 PORT、AGENT_PORT、DATA_DIR、SESSION_TTL、ADMIN_PASSWORD、HMAC_SECRET、CORS/TLS 配置
- 潜在问题：配置分散在 internal/config、server/config.go、middleware.go、README/docs 中，可能文档不一致
- 修改策略：建立环境变量矩阵并列入跨模块报告
- 验证方式：读取 config.go 与 docs/operations/config.md 对照
- 完成状态：已完成

## S01-0009
- 文件/目录范围：`server/config.go`
- 检查目标：标记 TLS 证书生成入口和 CERT_HOST 行为
- 潜在问题：自签证书生成在运行时写 certs/server.key，可能和源码目录、部署目录边界混乱
- 修改策略：建议迁移到独立 runtime data/certs 或部署配置
- 验证方式：rg CERT_HOST/certs/server server/config.go docs scripts
- 完成状态：已完成

## S01-0010
- 文件/目录范围：`server/db.go`
- 检查目标：标记 SQLite 入口、schema 迁移、JSON 到 SQLite 迁移路径
- 潜在问题：schema 与迁移逻辑集中在 main 包，Java 迁移需要可重复迁移模型
- 修改策略：纳入数据库迁移阶段和风险登记
- 验证方式：go test db_test.go 与 rg CREATE TABLE server
- 完成状态：已完成

## S01-0011
- 文件/目录范围：`server/handler.go`
- 检查目标：标记客户端 API、HMAC 验签、DLL 加密、payload 读取和会话校验入口
- 潜在问题：APIHandler 仍在 main 包，跨 sessions/payloads/releases/auth 依赖密集
- 修改策略：记录为跨模块依赖热点，建议后端会话继续拆分
- 验证方式：rg VerifyRequest/ValidateBound/HandleDllDownload server/handler.go
- 完成状态：已完成

## S01-0012
- 文件/目录范围：`server/middleware.go`
- 检查目标：标记 CORS、安全头、CSP、请求体限制入口
- 潜在问题：CORS 对客户端 API 使用 *，管理端/代理端走环境变量，策略分裂需安全复核
- 修改策略：写入安全风险并交接安全会话
- 验证方式：rg CORS/CSP/MaxBytesReader server/middleware.go
- 完成状态：已完成

## S01-0013
- 文件/目录范围：`server/admin.go`
- 检查目标：检查旧 admin handler 是否与 internal/features/admin 存在重复职责
- 潜在问题：根 main 包 admin 与 internal/features/admin 并存，可能造成重复模块或迁移半完成
- 修改策略：写入重复模块风险并交由后端模块会话拆分
- 验证方式：rg NewAdminHandler/HandleAdmin server/admin.go server/internal/features/admin
- 完成状态：已完成

## S01-0014
- 文件/目录范围：`server/internal/auth/auth.go`
- 检查目标：标记 HMAC 请求认证、密钥派生、客户端签名兼容入口
- 潜在问题：Go HMAC 语义必须和 C++ ApiClient 完全一致，Java 迁移风险高
- 修改策略：建立协议兼容测试要求
- 验证方式：rg VerifyRequest/GetDLLKey/HMAC server/internal/auth client/src/services
- 完成状态：已完成

## S01-0015
- 文件/目录范围：`server/internal/auth/session.go`
- 检查目标：标记管理端会话、cookie 或 token 入口
- 潜在问题：管理端权限和 agent/client 会话可能使用不同模型，迁移时易混淆
- 修改策略：纳入鉴权入口清单和 Java 安全模型设计
- 验证方式：读取 session.go 与 admin handler 权限调用对照
- 完成状态：已完成

## S01-0016
- 文件/目录范围：`server/internal/httpapi`
- 检查目标：检查 HTTP 统一响应、错误码、安全头、限流工具边界
- 潜在问题：响应格式、安全头和限流分散调用，Java 迁移需保持错误语义
- 修改策略：映射为 Java common/httpapi 包或 Spring advice/filter
- 验证方式：go test ./internal/httpapi 与 rg WriteError/WriteOK
- 完成状态：已完成

## S01-0017
- 文件/目录范围：`server/internal/features/sessions`
- 检查目标：标记客户端激活、心跳、解绑、过期清理、session token 入口
- 潜在问题：sessions 同时被 chat/releases/payloads/exam 依赖，迁移顺序决定系统兼容性
- 修改策略：列为 Java 第一批核心域迁移
- 验证方式：go test ./internal/features/sessions
- 完成状态：已完成

## S01-0018
- 文件/目录范围：`server/internal/features/cards`
- 检查目标：标记卡密/授权底层实体、生成、绑定、批量操作和仓储入口
- 潜在问题：cards 与 authorizations 语义并存，存在领域命名迁移期重叠
- 修改策略：定义 cards 为底层兼容域，authorizations 为管理聚合域
- 验证方式：go test ./internal/features/cards
- 完成状态：已完成

## S01-0019
- 文件/目录范围：`server/internal/features/authorizations`
- 检查目标：检查授权控制面聚合是否只依赖窄接口，不复制 cards/sessions 持久化
- 潜在问题：聚合层若直接耦合多个仓库会放大 Java 迁移复杂度
- 修改策略：列为跨模块聚合，要求接口边界和集成测试
- 验证方式：go test ./internal/features/authorizations
- 完成状态：已完成

## S01-0020
- 文件/目录范围：`server/internal/features/agents`
- 检查目标：标记代理端账号、邀请码、机器卡密管理、代理登录入口
- 潜在问题：agent 服务器独立端口但共享数据库和权限模型，部署和 CORS 风险较高
- 修改策略：在模块图中单独标记 agent portal 与 admin portal 边界
- 验证方式：go test ./internal/features/agents
- 完成状态：已完成

## S01-0021
- 文件/目录范围：`server/internal/features/chat`
- 检查目标：标记聊天消息、在线状态和客户端会话依赖入口
- 潜在问题：chat 使用 JSON storage 而非纯 SQLite，Java 迁移需要先统一存储策略
- 修改策略：写入 Java 迁移先决条件
- 验证方式：rg NewService storage chat server/internal/features/chat
- 完成状态：已完成

## S01-0022
- 文件/目录范围：`server/internal/features/releases`
- 检查目标：标记更新检查、发布包、事件、manifest 签名入口
- 潜在问题：release manifest 与客户端 update_controller 强耦合，Java 迁移必须保持签名协议
- 修改策略：交接发布会话维护协议兼容测试
- 验证方式：go test ./internal/features/releases ./internal/releases
- 完成状态：已完成

## S01-0023
- 文件/目录范围：`server/internal/features/payloads`
- 检查目标：标记 payload 上传、分片下载、密钥交换、AES/HMAC 加解密入口
- 潜在问题：payloads 含大文件上传和自定义加密，迁移风险最高
- 修改策略：列为 Java 后期迁移并要求黄金向量测试
- 验证方式：go test ./internal/features/payloads
- 完成状态：已完成

## S01-0024
- 文件/目录范围：`server/internal/features/scripts`
- 检查目标：标记脚本仓库、发布、激活、下载和客户端 script_loader 关联入口
- 潜在问题：脚本单一事实源可能仍处迁移期，需确认旧端点兼容
- 修改策略：写入跨模块脚本链路报告
- 验证方式：go test ./internal/features/scripts 与 client/src/hook/script_loader.h 对照
- 完成状态：已完成

## S01-0025
- 文件/目录范围：`server/internal/features/exam`
- 检查目标：标记考试记录上报、采集、管理统计入口
- 潜在问题：exam 同时有 HMAC+session 与 page-context token-only 两种鉴权入口
- 修改策略：列入鉴权入口清单并交接安全会话复核
- 验证方式：go test ./internal/features/exam
- 完成状态：已完成

## S01-0026
- 文件/目录范围：`server/internal/features/security/blacklist`
- 检查目标：标记黑名单服务和机器/卡密阻断入口
- 潜在问题：黑名单被 sessions/chat/APIHandler 调用，迁移需统一规则优先级
- 修改策略：纳入 Java rule service 设计
- 验证方式：go test ./internal/features/security/blacklist
- 完成状态：已完成

## S01-0027
- 文件/目录范围：`server/internal/audit`
- 检查目标：标记审计日志仓储和审计写入入口
- 潜在问题：审计由多个 handler 以 callback 写入，失败处理和字段一致性需统一
- 修改策略：设计 Java audit event publisher 或 service
- 验证方式：go test ./internal/audit
- 完成状态：已完成

## S01-0028
- 文件/目录范围：`server/internal/storage`
- 检查目标：标记 JSON storage 兼容层和剩余 JSON 数据读写入口
- 潜在问题：SQLite 已是主存储但 chat/payload/releases 仍可能依赖 JSON storage
- 修改策略：写入 Java 迁移阻塞项：先收敛存储抽象
- 验证方式：rg storage.NewJSONStorage/Load/Save server
- 完成状态：已完成

## S01-0029
- 文件/目录范围：`server/internal/releases`
- 检查目标：标记 manifest signer/store/selector 核心算法入口
- 潜在问题：签名、hash、选择算法要与客户端更新逻辑兼容
- 修改策略：要求迁移前生成跨语言 golden tests
- 验证方式：go test ./internal/releases
- 完成状态：已完成

## S01-0030
- 文件/目录范围：`server/internal/platform/cardcode`
- 检查目标：标记卡密编码算法入口
- 潜在问题：卡密格式如果迁移实现不一致会破坏存量授权
- 修改策略：加入 Java 迁移 golden vector 清单
- 验证方式：go test ./internal/platform/cardcode
- 完成状态：已完成

## S01-0031
- 文件/目录范围：`server/web/package.json`
- 检查目标：标记前端包管理和构建入口 npm run check/build/typecheck
- 潜在问题：前端构建输出嵌入 Go web-dist，Java 迁移需要改为 resources/static 或外置部署
- 修改策略：写入构建/部署入口清单
- 验证方式：npm run check
- 完成状态：已完成

## S01-0032
- 文件/目录范围：`server/web/vite.config.js`
- 检查目标：核对 admin/agent 双入口 Vite 输出路径和 asset base
- 潜在问题：错误 base/outDir 会导致 Go embed 或 Java static resources 路径不一致
- 修改策略：纳入 Java 静态资源迁移方案
- 验证方式：npm run build 后检查 server/web-dist
- 完成状态：已完成

## S01-0033
- 文件/目录范围：`server/web/admin/index.html`
- 检查目标：标记管理端 HTML 入口和挂载点
- 潜在问题：Java 静态资源路径可能改变 /admin/ 前缀行为
- 修改策略：记录为前端入口
- 验证方式：读取 HTML 与 Vite 配置对照
- 完成状态：已完成

## S01-0034
- 文件/目录范围：`server/web/admin/src/main.ts`
- 检查目标：标记管理端 TypeScript 应用入口和 API 调用链
- 潜在问题：前端 API base 与 Go route 前缀强耦合，迁移需要代理/路径兼容
- 修改策略：列为前端会话交接
- 验证方式：rg fetch/api server/web/admin/src/main.ts server/web/shared/src/api.ts
- 完成状态：已完成

## S01-0035
- 文件/目录范围：`server/web/agent/index.html`
- 检查目标：标记代理端 HTML 入口和挂载点
- 潜在问题：agent 独立端口/根路径和 /agent/ 静态路径需迁移兼容
- 修改策略：记录为前端入口
- 验证方式：读取 HTML 与 buildAgentServer 对照
- 完成状态：已完成

## S01-0036
- 文件/目录范围：`server/web/agent/src/main.ts`
- 检查目标：标记代理端 TypeScript 入口、登录和业务 API 调用
- 潜在问题：agent API 与 admin API 权限边界可能混杂
- 修改策略：列为前端/后端交界检查
- 验证方式：npm run typecheck 与 rg /agent/api
- 完成状态：已完成

## S01-0037
- 文件/目录范围：`server/web/shared/src/api.ts`
- 检查目标：标记前端共享 API 封装和错误处理入口
- 潜在问题：共享 API 若写死路径或响应格式，Java 迁移需保持兼容
- 修改策略：纳入 API 合约矩阵
- 验证方式：npm run typecheck
- 完成状态：已完成

## S01-0038
- 文件/目录范围：`docs/api/openapi.yaml`
- 检查目标：核对 OpenAPI 是否覆盖 admin、agent、client、payload、release、exam 路由
- 潜在问题：实际 main.go 路由可能超过 OpenAPI，造成迁移合约缺口
- 修改策略：写入风险并交接 API 文档会话
- 验证方式：rg HandleFunc server/main.go 与 openapi paths 对照
- 完成状态：已完成

## S01-0039
- 文件/目录范围：`docs/adr`
- 检查目标：核对 ADR 是否覆盖当前模块边界、SQLite、release manifest、脚本事实源
- 潜在问题：ADR 可能编码异常或未覆盖 Java 迁移决策
- 修改策略：建立 Java 迁移 ADR 待办
- 验证方式：读取 docs/adr/*.md
- 完成状态：已完成

## S01-0040
- 文件/目录范围：`docs/operations/config.md`
- 检查目标：核对环境变量文档是否覆盖代码实际读取项
- 潜在问题：UPLOAD_KEY、SIGNING_CERT、CERT_HOST 等可能遗漏或命名不一致
- 修改策略：写入配置风险并建议环境变量矩阵
- 验证方式：rg os.Getenv/envOr 与 docs/operations/config.md 对照
- 完成状态：已完成

## S01-0041
- 文件/目录范围：`docs/security`
- 检查目标：核对密钥、签名、ASVS 文档是否覆盖 HMAC、DLL 加密、payload key exchange、TLS cert
- 潜在问题：安全文档可能覆盖发布签名但未覆盖 runtime secret 生命周期
- 修改策略：交接安全会话补齐威胁模型
- 验证方式：读取 docs/security/*.md 与 rg crypto 对照
- 完成状态：已完成

## S01-0042
- 文件/目录范围：`client/CMakeLists.txt`
- 检查目标：标记 C++ 根构建入口、CMake policy、全局安全编译选项和 CEF/Chromium include
- 潜在问题：第三方 include 直接从根路径引用，Java 后端迁移仍需保持客户端协议而非重建客户端
- 修改策略：记录为客户端边界，不进行业务代码大改
- 验证方式：cmake -S client -B build -G Visual Studio...
- 完成状态：已完成

## S01-0043
- 文件/目录范围：`client/src/CMakeLists.txt`
- 检查目标：标记 CefHook、Injector、ManualDllInjector、测试目标和 Qt/MinHook 依赖
- 潜在问题：构建目标和源码文件列表手工维护，新增 hook 文件容易漏注册
- 修改策略：写入客户端会话交接和构建验证要求
- 验证方式：cmake configure + cmake --build 或目标检查
- 完成状态：已完成

## S01-0044
- 文件/目录范围：`client/src/injector_qt.cpp`
- 检查目标：标记 Windows Qt 客户端运行入口
- 潜在问题：UI 入口与 controllers/services/hook/network 依赖可能仍未完全分层
- 修改策略：纳入模块边界地图并交接客户端会话
- 验证方式：CMake target Injector 编译
- 完成状态：已完成

## S01-0045
- 文件/目录范围：`client/src/services/api_client.cpp`
- 检查目标：标记客户端 HTTP、HMAC、证书 pinning、会话/更新 API 调用入口
- 潜在问题：Go/Java 迁移必须保持 ApiClient 请求签名和响应字段兼容
- 修改策略：写入 Java 迁移最高优先级协议兼容项
- 验证方式：客户端回归脚本和 API golden tests
- 完成状态：已完成

## S01-0046
- 文件/目录范围：`client/src/controllers/session_controller.cpp`
- 检查目标：标记客户端会话状态机、激活、心跳、退出、恢复入口
- 潜在问题：与 server sessions 服务强耦合，迁移需保持错误码和字段语义
- 修改策略：交接客户端会话与后端 sessions 会话联合验证
- 验证方式：scripts/client_runtime_binding_regression_test.ps1
- 完成状态：已完成

## S01-0047
- 文件/目录范围：`client/src/controllers/update_controller.cpp`
- 检查目标：标记客户端更新检查与 manifest 处理入口
- 潜在问题：release manifest 签名和 URL 路径改动会直接影响更新
- 修改策略：写入 release 跨模块报告
- 验证方式：客户端 CMake 测试 + release handler tests
- 完成状态：已完成

## S01-0048
- 文件/目录范围：`client/src/core/config.cpp`
- 检查目标：标记客户端服务器地址、路径、版本、更新公钥等全局配置入口
- 潜在问题：配置分散在 CMake definitions、headers、运行环境，迁移时易遗漏
- 修改策略：建立配置矩阵，避免业务代码大改
- 验证方式：rg APP_VERSION/UPDATE_MANIFEST_PUBLIC_KEY_HEX/client config
- 完成状态：已完成

## S01-0049
- 文件/目录范围：`client/src/core/crypto.h`
- 检查目标：标记客户端加解密/HMAC/哈希兼容入口
- 潜在问题：跨语言迁移如果字节序、编码、base64/hex 处理不一致会破坏协议
- 修改策略：要求 golden vector
- 验证方式：C++ tests + Go auth tests
- 完成状态：已完成

## S01-0050
- 文件/目录范围：`client/src/hook/dllmain.cpp`
- 检查目标：标记 CefHook DLL 注入入口
- 潜在问题：客户端 hook 与服务端 DLL 分发/加密链路相连，后端迁移不可改变下载语义
- 修改策略：记录为客户端安全边界
- 验证方式：CMake CefHook target 编译
- 完成状态：已完成

## S01-0051
- 文件/目录范围：`client/src/hook/thread.cpp`
- 检查目标：标记 Hook 后台线程、桥接初始化和运行时绑定入口
- 潜在问题：线程启动/环境变量读取与 server script/payload 交互可能隐式耦合
- 修改策略：交接客户端 hook 会话
- 验证方式：CMake CefHook target + runtime binding regression
- 完成状态：已完成

## S01-0052
- 文件/目录范围：`client/src/hook/script_loader.h`
- 检查目标：标记脚本下载/加载入口和 server scripts API 兼容点
- 潜在问题：脚本路径、签名或响应格式改动会破坏客户端注入
- 修改策略：写入跨模块 scripts 报告
- 验证方式：scripts/client_runtime_binding_regression_test.ps1
- 完成状态：已完成

## S01-0053
- 文件/目录范围：`client/src/hook/bridge_protocol.h`
- 检查目标：标记 CEF/V8 bridge 协议入口
- 潜在问题：协议字段若无文档，Java 后端迁移虽不直接触碰仍需避免误判边界
- 修改策略：纳入模块归属，不归后端迁移修改
- 验证方式：C++ bridge tests/compile
- 完成状态：已完成

## S01-0054
- 文件/目录范围：`client/src/security/machine_fp.h`
- 检查目标：标记机器指纹入口和 server sessions 绑定语义
- 潜在问题：机器 ID 生成规则变更会影响卡密绑定、黑名单、审计
- 修改策略：要求服务端迁移保持字段语义
- 验证方式：客户端编译与 server sessions tests
- 完成状态：已完成

## S01-0055
- 文件/目录范围：`packages/autoexam/src/autoexam.js`
- 检查目标：标记 AutoExam 包入口和 hook/exam 上报链路
- 潜在问题：根 AutoExam.js 与 packages/autoexam 可能重复模块
- 修改策略：写入重复模块风险并交接脚本/考试会话
- 验证方式：rg AutoExam packages AutoExam.js client/src/hook
- 完成状态：已完成

## S01-0056
- 文件/目录范围：`AutoExam.js`
- 检查目标：检查根目录 AutoExam.js 是否为发布产物、源码副本或遗留脚本
- 潜在问题：根脚本与 packages/autoexam/src/autoexam.js 重复，可能产生事实源混乱
- 修改策略：标记为重复模块风险，不直接删除
- 验证方式：文件 hash/内容对照 packages/autoexam
- 完成状态：已完成

## S01-0057
- 文件/目录范围：`installer`
- 检查目标：标记 WiX 安装入口、便携包构建、签名环境变量和客户端产物输入
- 潜在问题：安装脚本可能引用旧产物路径或缺签名校验
- 修改策略：交接部署/发布会话
- 验证方式：读取 installer/*.ps1/*.wxs 与 scripts/release.ps1
- 完成状态：已完成

## S01-0058
- 文件/目录范围：`scripts/release.ps1`
- 检查目标：标记 release 打包、签名、产物收集和版本入口
- 潜在问题：发布脚本与 Go release manifest、installer、client CMake 版本可能不一致
- 修改策略：写入构建链路风险
- 验证方式：静态检查 release.ps1 与 CMake APP_VERSION
- 完成状态：已完成

## S01-0059
- 文件/目录范围：`e2e_test`
- 检查目标：标记端到端测试模块、Go 测试入口和证书依赖
- 潜在问题：e2e 测试可能未被 scripts/check.ps1 调用，迁移后缺少系统级保障
- 修改策略：建议纳入质量门禁或单独 CI
- 验证方式：go test ./... in e2e_test
- 完成状态：已完成

## S01-0060
- 文件/目录范围：`data`
- 检查目标：标记运行数据库、上传密钥、seed key 等本地数据入口
- 潜在问题：data 目录含数据库/密钥，若进入仓库会造成泄露和环境污染
- 修改策略：列为高风险并建议隔离 runtime data
- 验证方式：git status --short data 与 docs/security 对照
- 完成状态：已完成

## S01-0061
- 文件/目录范围：`certs`
- 检查目标：标记根证书目录和 server TLS 证书入口
- 潜在问题：证书私钥可能在源码根目录生成或提交
- 修改策略：列为高风险并建议 runtime/certs 隔离与 .gitignore 强化
- 验证方式：Get-ChildItem certs 与 server/config.go 对照
- 完成状态：已完成

## S01-0062
- 文件/目录范围：`server/certs`
- 检查目标：标记 server 目录证书副本或运行输出
- 潜在问题：根 certs 与 server/certs 双位置可能造成部署混乱
- 修改策略：写入跨模块报告
- 验证方式：目录清单和 TLS 加载路径对照
- 完成状态：已完成

## S01-0063
- 文件/目录范围：`server/web-dist`
- 检查目标：标记 Go embed 静态资源输出入口
- 潜在问题：Java 迁移需决定内嵌资源或外置 CDN/静态服务；当前路径是 web-dist/admin 和 web-dist/agent
- 修改策略：纳入 Java 迁移静态资源阶段
- 验证方式：npm run build 与 go build
- 完成状态：已完成

## S01-0064
- 文件/目录范围：`build/build64/client/build64/build-status-review`
- 检查目标：检查构建产物目录是否与源码、文档、发布脚本存在错误引用
- 潜在问题：多个 build 目录并存，可能造成脚本读取旧产物或误提交
- 修改策略：记录为清理/忽略策略，不删除用户产物
- 验证方式：rg build64/build-status-review scripts installer README
- 完成状态：已完成

## S01-0065
- 文件/目录范围：`server/*.exe and server/injector-server`
- 检查目标：检查后端目录下可执行产物是否为误放置构建输出
- 潜在问题：可执行文件位于源码目录，可能影响包管理、发布和安全扫描
- 修改策略：写入风险与 handoff，不直接删除
- 验证方式：git status --short server/*.exe server/injector-server
- 完成状态：已完成

## S01-0066
- 文件/目录范围：`.github/workflows`
- 检查目标：标记 CI/CD 入口和质量门禁是否存在
- 潜在问题：若 CI 未覆盖 Go/web/C++/e2e，重构和 Java 迁移缺乏回归保障
- 修改策略：写入全局风险并建议分阶段 CI
- 验证方式：读取 workflow yaml
- 完成状态：已完成

## S01-0067
- 文件/目录范围：`.vscode`
- 检查目标：检查本地 IDE 配置是否含硬编码路径、密钥或构建参数
- 潜在问题：开发者本地路径可能污染团队构建
- 修改策略：记录到风险，不直接删除
- 验证方式：读取 .vscode 文件
- 完成状态：已完成

## S01-0068
- 文件/目录范围：`.superpowers`
- 检查目标：标记 agent 工作流产物目录归属
- 潜在问题：会话过程产物若进入仓库，会污染项目交付结构
- 修改策略：建议加入忽略或迁移到 docs/superpowers 规范目录
- 验证方式：git status --short .superpowers
- 完成状态：已完成

## S01-0069
- 文件/目录范围：`docs/superpowers`
- 检查目标：核对既有设计/计划是否与当前会话输出冲突或重复
- 潜在问题：多会话并行可能产生重复计划和边界冲突
- 修改策略：在 handoff 中指明本会话总控文档优先级
- 验证方式：读取 docs/superpowers/plans specs 文件名和主题
- 完成状态：已完成

## S01-0070
- 文件/目录范围：`docs/PROJECT_HANDOFF_REPORT.md`
- 检查目标：核对既有交接报告与本次 PARALLEL_HANDOFF 的差异
- 潜在问题：历史交接可能与最新 git 状态不一致
- 修改策略：保留历史文档，新增本会话交接报告
- 验证方式：读取并比对模块清单
- 完成状态：已完成

## S01-0071
- 文件/目录范围：`docs/ARCHITECTURE_V2_PROPOSAL.md`
- 检查目标：核对 V2 架构提案编码、模块边界和当前实现差距
- 潜在问题：文档出现编码异常，且当前实现可能偏离提案
- 修改策略：写入风险并建议文档编码修复/架构状态更新
- 验证方式：读取文档并与目录结构对照
- 完成状态：已完成

## S01-0072
- 文件/目录范围：`.github`
- 检查目标：确认目录 .github 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.github' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0073
- 文件/目录范围：`.github/workflows`
- 检查目标：确认目录 .github/workflows 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.github/workflows' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0074
- 文件/目录范围：`.superpowers`
- 检查目标：确认目录 .superpowers 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.superpowers' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0075
- 文件/目录范围：`.superpowers/brainstorm`
- 检查目标：确认目录 .superpowers/brainstorm 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.superpowers/brainstorm' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0076
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235`
- 检查目标：确认目录 .superpowers/brainstorm/admin-architecture-20260702-2235 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.superpowers/brainstorm/admin-architecture-20260702-2235' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0077
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content`
- 检查目标：确认目录 .superpowers/brainstorm/admin-architecture-20260702-2235/content 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.superpowers/brainstorm/admin-architecture-20260702-2235/content' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0078
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/state`
- 检查目标：确认目录 .superpowers/brainstorm/admin-architecture-20260702-2235/state 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.superpowers/brainstorm/admin-architecture-20260702-2235/state' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0079
- 文件/目录范围：`.vscode`
- 检查目标：确认目录 .vscode 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem '.vscode' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0080
- 文件/目录范围：`build`
- 检查目标：确认目录 build 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0081
- 文件/目录范围：`build-status-review`
- 检查目标：确认目录 build-status-review 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0082
- 文件/目录范围：`build-status-review/ALL_BUILD.dir`
- 检查目标：确认目录 build-status-review/ALL_BUILD.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ALL_BUILD.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0083
- 文件/目录范围：`build-status-review/ALL_BUILD.dir/Debug`
- 检查目标：确认目录 build-status-review/ALL_BUILD.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ALL_BUILD.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0084
- 文件/目录范围：`build-status-review/ALL_BUILD.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/ALL_BUILD.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ALL_BUILD.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0085
- 文件/目录范围：`build-status-review/ALL_BUILD.dir/Release`
- 检查目标：确认目录 build-status-review/ALL_BUILD.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ALL_BUILD.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0086
- 文件/目录范围：`build-status-review/ALL_BUILD.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/ALL_BUILD.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ALL_BUILD.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0087
- 文件/目录范围：`build-status-review/CMakeFiles`
- 检查目标：确认目录 build-status-review/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0088
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0089
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdC 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdC' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0090
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0091
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0092
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/tmp`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdC/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdC/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0093
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdCXX 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdCXX' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0094
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0095
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0096
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/tmp`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0097
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/Debug`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0098
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/VCTargetsPath 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/VCTargetsPath' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0099
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0100
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog`
- 检查目标：确认目录 build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0101
- 文件/目录范围：`build-status-review/CMakeFiles/468707a8d1ff2952380d1f29a6746929`
- 检查目标：确认目录 build-status-review/CMakeFiles/468707a8d1ff2952380d1f29a6746929 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/468707a8d1ff2952380d1f29a6746929' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0102
- 文件/目录范围：`build-status-review/CMakeFiles/50bdb25440a67914ffc6b48028979208`
- 检查目标：确认目录 build-status-review/CMakeFiles/50bdb25440a67914ffc6b48028979208 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/50bdb25440a67914ffc6b48028979208' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0103
- 文件/目录范围：`build-status-review/CMakeFiles/CMakeScratch`
- 检查目标：确认目录 build-status-review/CMakeFiles/CMakeScratch 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/CMakeScratch' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0104
- 文件/目录范围：`build-status-review/CMakeFiles/pkgRedirects`
- 检查目标：确认目录 build-status-review/CMakeFiles/pkgRedirects 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/CMakeFiles/pkgRedirects' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0105
- 文件/目录范围：`build-status-review/Release`
- 检查目标：确认目录 build-status-review/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0106
- 文件/目录范围：`build-status-review/src`
- 检查目标：确认目录 build-status-review/src 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0107
- 文件/目录范围：`build-status-review/src/CefHook.dir`
- 检查目标：确认目录 build-status-review/src/CefHook.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CefHook.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0108
- 文件/目录范围：`build-status-review/src/CefHook.dir/Debug`
- 检查目标：确认目录 build-status-review/src/CefHook.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CefHook.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0109
- 文件/目录范围：`build-status-review/src/CefHook.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/src/CefHook.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CefHook.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0110
- 文件/目录范围：`build-status-review/src/CefHook.dir/Release`
- 检查目标：确认目录 build-status-review/src/CefHook.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CefHook.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0111
- 文件/目录范围：`build-status-review/src/CefHook.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/src/CefHook.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CefHook.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0112
- 文件/目录范围：`build-status-review/src/CMakeFiles`
- 检查目标：确认目录 build-status-review/src/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0113
- 文件/目录范围：`build-status-review/src/CMakeFiles/CefHook_autogen.dir`
- 检查目标：确认目录 build-status-review/src/CMakeFiles/CefHook_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CMakeFiles/CefHook_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0114
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir`
- 检查目标：确认目录 build-status-review/src/CMakeFiles/Injector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CMakeFiles/Injector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0115
- 文件/目录范围：`build-status-review/src/CMakeFiles/ManualDllInjector_autogen.dir`
- 检查目标：确认目录 build-status-review/src/CMakeFiles/ManualDllInjector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CMakeFiles/ManualDllInjector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0116
- 文件/目录范围：`build-status-review/src/CMakeFiles/ManualDllInjectorGui_autogen.dir`
- 检查目标：确认目录 build-status-review/src/CMakeFiles/ManualDllInjectorGui_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/CMakeFiles/ManualDllInjectorGui_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0117
- 文件/目录范围：`build-status-review/src/Injector_autogen`
- 检查目标：确认目录 build-status-review/src/Injector_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0118
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release`
- 检查目标：确认目录 build-status-review/src/Injector_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0119
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/5SSUPNRDSB`
- 检查目标：确认目录 build-status-review/src/Injector_autogen/include_Release/5SSUPNRDSB 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector_autogen/include_Release/5SSUPNRDSB' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0120
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ`
- 检查目标：确认目录 build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0121
- 文件/目录范围：`build-status-review/src/Injector_autogen/UYX5XTB5RZ`
- 检查目标：确认目录 build-status-review/src/Injector_autogen/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector_autogen/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0122
- 文件/目录范围：`build-status-review/src/Injector.dir`
- 检查目标：确认目录 build-status-review/src/Injector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0123
- 文件/目录范围：`build-status-review/src/Injector.dir/Debug`
- 检查目标：确认目录 build-status-review/src/Injector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0124
- 文件/目录范围：`build-status-review/src/Injector.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/src/Injector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0125
- 文件/目录范围：`build-status-review/src/Injector.dir/Release`
- 检查目标：确认目录 build-status-review/src/Injector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0126
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog`
- 检查目标：确认目录 build-status-review/src/Injector.dir/Release/Injector.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir/Release/Injector.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0127
- 文件/目录范围：`build-status-review/src/Injector.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/src/Injector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Injector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0128
- 文件/目录范围：`build-status-review/src/ManualDllInjector.dir`
- 检查目标：确认目录 build-status-review/src/ManualDllInjector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0129
- 文件/目录范围：`build-status-review/src/ManualDllInjector.dir/Debug`
- 检查目标：确认目录 build-status-review/src/ManualDllInjector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0130
- 文件/目录范围：`build-status-review/src/ManualDllInjector.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/src/ManualDllInjector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0131
- 文件/目录范围：`build-status-review/src/ManualDllInjector.dir/Release`
- 检查目标：确认目录 build-status-review/src/ManualDllInjector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0132
- 文件/目录范围：`build-status-review/src/ManualDllInjector.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/src/ManualDllInjector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0133
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.dir`
- 检查目标：确认目录 build-status-review/src/ManualDllInjectorGui.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjectorGui.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0134
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.dir/Debug`
- 检查目标：确认目录 build-status-review/src/ManualDllInjectorGui.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjectorGui.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0135
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/src/ManualDllInjectorGui.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjectorGui.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0136
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.dir/Release`
- 检查目标：确认目录 build-status-review/src/ManualDllInjectorGui.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjectorGui.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0137
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/src/ManualDllInjectorGui.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/ManualDllInjectorGui.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0138
- 文件/目录范围：`build-status-review/src/Release`
- 检查目标：确认目录 build-status-review/src/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0139
- 文件/目录范围：`build-status-review/src/Release/iconengines`
- 检查目标：确认目录 build-status-review/src/Release/iconengines 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Release/iconengines' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0140
- 文件/目录范围：`build-status-review/src/Release/imageformats`
- 检查目标：确认目录 build-status-review/src/Release/imageformats 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Release/imageformats' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0141
- 文件/目录范围：`build-status-review/src/Release/platforms`
- 检查目标：确认目录 build-status-review/src/Release/platforms 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Release/platforms' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0142
- 文件/目录范围：`build-status-review/src/Release/styles`
- 检查目标：确认目录 build-status-review/src/Release/styles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/src/Release/styles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0143
- 文件/目录范围：`build-status-review/Win32`
- 检查目标：确认目录 build-status-review/Win32 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/Win32' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0144
- 文件/目录范围：`build-status-review/Win32/Release`
- 检查目标：确认目录 build-status-review/Win32/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/Win32/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0145
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK`
- 检查目标：确认目录 build-status-review/Win32/Release/ZERO_CHECK 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/Win32/Release/ZERO_CHECK' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0146
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog`
- 检查目标：确认目录 build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0147
- 文件/目录范围：`build-status-review/ZERO_CHECK.dir`
- 检查目标：确认目录 build-status-review/ZERO_CHECK.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ZERO_CHECK.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0148
- 文件/目录范围：`build-status-review/ZERO_CHECK.dir/Debug`
- 检查目标：确认目录 build-status-review/ZERO_CHECK.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ZERO_CHECK.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0149
- 文件/目录范围：`build-status-review/ZERO_CHECK.dir/MinSizeRel`
- 检查目标：确认目录 build-status-review/ZERO_CHECK.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ZERO_CHECK.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0150
- 文件/目录范围：`build-status-review/ZERO_CHECK.dir/Release`
- 检查目标：确认目录 build-status-review/ZERO_CHECK.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ZERO_CHECK.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0151
- 文件/目录范围：`build-status-review/ZERO_CHECK.dir/RelWithDebInfo`
- 检查目标：确认目录 build-status-review/ZERO_CHECK.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build-status-review/ZERO_CHECK.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0152
- 文件/目录范围：`build/.cmake`
- 检查目标：确认目录 build/.cmake 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0153
- 文件/目录范围：`build/.cmake/api`
- 检查目标：确认目录 build/.cmake/api 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake/api' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0154
- 文件/目录范围：`build/.cmake/api/v1`
- 检查目标：确认目录 build/.cmake/api/v1 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake/api/v1' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0155
- 文件/目录范围：`build/.cmake/api/v1/query`
- 检查目标：确认目录 build/.cmake/api/v1/query 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake/api/v1/query' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0156
- 文件/目录范围：`build/.cmake/api/v1/query/client-vscode`
- 检查目标：确认目录 build/.cmake/api/v1/query/client-vscode 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake/api/v1/query/client-vscode' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0157
- 文件/目录范围：`build/.cmake/api/v1/reply`
- 检查目标：确认目录 build/.cmake/api/v1/reply 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/.cmake/api/v1/reply' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0158
- 文件/目录范围：`build/ALL_BUILD.dir`
- 检查目标：确认目录 build/ALL_BUILD.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ALL_BUILD.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0159
- 文件/目录范围：`build/ALL_BUILD.dir/Debug`
- 检查目标：确认目录 build/ALL_BUILD.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ALL_BUILD.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0160
- 文件/目录范围：`build/ALL_BUILD.dir/MinSizeRel`
- 检查目标：确认目录 build/ALL_BUILD.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ALL_BUILD.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0161
- 文件/目录范围：`build/ALL_BUILD.dir/Release`
- 检查目标：确认目录 build/ALL_BUILD.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ALL_BUILD.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0162
- 文件/目录范围：`build/ALL_BUILD.dir/RelWithDebInfo`
- 检查目标：确认目录 build/ALL_BUILD.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ALL_BUILD.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0163
- 文件/目录范围：`build/CMakeFiles`
- 检查目标：确认目录 build/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0164
- 文件/目录范围：`build/CMakeFiles/1c7bb12b6d416d4773566c17420a44f4`
- 检查目标：确认目录 build/CMakeFiles/1c7bb12b6d416d4773566c17420a44f4 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/1c7bb12b6d416d4773566c17420a44f4' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0165
- 文件/目录范围：`build/CMakeFiles/4.3.0`
- 检查目标：确认目录 build/CMakeFiles/4.3.0 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0166
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdC 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdC' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0167
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdC/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdC/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0168
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0169
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/tmp`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdC/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdC/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0170
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdCXX 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdCXX' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0171
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdCXX/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdCXX/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0172
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0173
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/tmp`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/CompilerIdCXX/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/CompilerIdCXX/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0174
- 文件/目录范围：`build/CMakeFiles/4.3.0/Debug`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0175
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/VCTargetsPath 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/VCTargetsPath' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0176
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath/Debug`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/VCTargetsPath/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/VCTargetsPath/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0177
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog`
- 检查目标：确认目录 build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0178
- 文件/目录范围：`build/CMakeFiles/e7e6f86041512cc3686f61202aeab8e9`
- 检查目标：确认目录 build/CMakeFiles/e7e6f86041512cc3686f61202aeab8e9 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/e7e6f86041512cc3686f61202aeab8e9' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0179
- 文件/目录范围：`build/CMakeFiles/pkgRedirects`
- 检查目标：确认目录 build/CMakeFiles/pkgRedirects 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/CMakeFiles/pkgRedirects' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0180
- 文件/目录范围：`build/Release`
- 检查目标：确认目录 build/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0181
- 文件/目录范围：`build/src`
- 检查目标：确认目录 build/src 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0182
- 文件/目录范围：`build/src/CefHook_autogen`
- 检查目标：确认目录 build/src/CefHook_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0183
- 文件/目录范围：`build/src/CefHook_autogen/include_Release`
- 检查目标：确认目录 build/src/CefHook_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0184
- 文件/目录范围：`build/src/CefHook.dir`
- 检查目标：确认目录 build/src/CefHook.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0185
- 文件/目录范围：`build/src/CefHook.dir/Debug`
- 检查目标：确认目录 build/src/CefHook.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0186
- 文件/目录范围：`build/src/CefHook.dir/MinSizeRel`
- 检查目标：确认目录 build/src/CefHook.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0187
- 文件/目录范围：`build/src/CefHook.dir/Release`
- 检查目标：确认目录 build/src/CefHook.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0188
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog`
- 检查目标：确认目录 build/src/CefHook.dir/Release/CefHook.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir/Release/CefHook.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0189
- 文件/目录范围：`build/src/CefHook.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/CefHook.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CefHook.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0190
- 文件/目录范围：`build/src/ClientTests_autogen`
- 检查目标：确认目录 build/src/ClientTests_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0191
- 文件/目录范围：`build/src/ClientTests_autogen/include_Release`
- 检查目标：确认目录 build/src/ClientTests_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0192
- 文件/目录范围：`build/src/ClientTests.dir`
- 检查目标：确认目录 build/src/ClientTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0193
- 文件/目录范围：`build/src/ClientTests.dir/Debug`
- 检查目标：确认目录 build/src/ClientTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0194
- 文件/目录范围：`build/src/ClientTests.dir/MinSizeRel`
- 检查目标：确认目录 build/src/ClientTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0195
- 文件/目录范围：`build/src/ClientTests.dir/Release`
- 检查目标：确认目录 build/src/ClientTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0196
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog`
- 检查目标：确认目录 build/src/ClientTests.dir/Release/ClientTests.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir/Release/ClientTests.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0197
- 文件/目录范围：`build/src/ClientTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/ClientTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ClientTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0198
- 文件/目录范围：`build/src/CMakeFiles`
- 检查目标：确认目录 build/src/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0199
- 文件/目录范围：`build/src/CMakeFiles/CefHook_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/CefHook_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/CefHook_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0200
- 文件/目录范围：`build/src/CMakeFiles/ClientTests_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/ClientTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/ClientTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0201
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/Injector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/Injector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0202
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjector_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/ManualDllInjector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/ManualDllInjector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0203
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0204
- 文件/目录范围：`build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0205
- 文件/目录范围：`build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0206
- 文件/目录范围：`build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0207
- 文件/目录范围：`build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir`
- 检查目标：确认目录 build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0208
- 文件/目录范围：`build/src/Injector_autogen`
- 检查目标：确认目录 build/src/Injector_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0209
- 文件/目录范围：`build/src/Injector_autogen/include_Release`
- 检查目标：确认目录 build/src/Injector_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0210
- 文件/目录范围：`build/src/Injector_autogen/include_Release/5SSUPNRDSB`
- 检查目标：确认目录 build/src/Injector_autogen/include_Release/5SSUPNRDSB 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen/include_Release/5SSUPNRDSB' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0211
- 文件/目录范围：`build/src/Injector_autogen/include_Release/7CL2LH2DRO`
- 检查目标：确认目录 build/src/Injector_autogen/include_Release/7CL2LH2DRO 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen/include_Release/7CL2LH2DRO' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0212
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ`
- 检查目标：确认目录 build/src/Injector_autogen/include_Release/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen/include_Release/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0213
- 文件/目录范围：`build/src/Injector_autogen/UYX5XTB5RZ`
- 检查目标：确认目录 build/src/Injector_autogen/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector_autogen/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0214
- 文件/目录范围：`build/src/Injector.dir`
- 检查目标：确认目录 build/src/Injector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0215
- 文件/目录范围：`build/src/Injector.dir/Debug`
- 检查目标：确认目录 build/src/Injector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0216
- 文件/目录范围：`build/src/Injector.dir/MinSizeRel`
- 检查目标：确认目录 build/src/Injector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0217
- 文件/目录范围：`build/src/Injector.dir/Release`
- 检查目标：确认目录 build/src/Injector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0218
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog`
- 检查目标：确认目录 build/src/Injector.dir/Release/Injector.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir/Release/Injector.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0219
- 文件/目录范围：`build/src/Injector.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/Injector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Injector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0220
- 文件/目录范围：`build/src/ManualDllInjector_autogen`
- 检查目标：确认目录 build/src/ManualDllInjector_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0221
- 文件/目录范围：`build/src/ManualDllInjector_autogen/include_Release`
- 检查目标：确认目录 build/src/ManualDllInjector_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0222
- 文件/目录范围：`build/src/ManualDllInjector.dir`
- 检查目标：确认目录 build/src/ManualDllInjector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0223
- 文件/目录范围：`build/src/ManualDllInjector.dir/Debug`
- 检查目标：确认目录 build/src/ManualDllInjector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0224
- 文件/目录范围：`build/src/ManualDllInjector.dir/MinSizeRel`
- 检查目标：确认目录 build/src/ManualDllInjector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0225
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release`
- 检查目标：确认目录 build/src/ManualDllInjector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0226
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog`
- 检查目标：确认目录 build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0227
- 文件/目录范围：`build/src/ManualDllInjector.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/ManualDllInjector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0228
- 文件/目录范围：`build/src/ManualDllInjectorGui_autogen`
- 检查目标：确认目录 build/src/ManualDllInjectorGui_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0229
- 文件/目录范围：`build/src/ManualDllInjectorGui_autogen/include_Release`
- 检查目标：确认目录 build/src/ManualDllInjectorGui_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0230
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0231
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Debug`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0232
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/MinSizeRel`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0233
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0234
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0235
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/ManualDllInjectorGui.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/ManualDllInjectorGui.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0236
- 文件/目录范围：`build/src/Release`
- 检查目标：确认目录 build/src/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0237
- 文件/目录范围：`build/src/Release/iconengines`
- 检查目标：确认目录 build/src/Release/iconengines 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Release/iconengines' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0238
- 文件/目录范围：`build/src/Release/imageformats`
- 检查目标：确认目录 build/src/Release/imageformats 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Release/imageformats' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0239
- 文件/目录范围：`build/src/Release/platforms`
- 检查目标：确认目录 build/src/Release/platforms 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Release/platforms' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0240
- 文件/目录范围：`build/src/Release/styles`
- 检查目标：确认目录 build/src/Release/styles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/Release/styles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0241
- 文件/目录范围：`build/src/V8CompileSamplerTests_autogen`
- 检查目标：确认目录 build/src/V8CompileSamplerTests_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0242
- 文件/目录范围：`build/src/V8CompileSamplerTests_autogen/include_Release`
- 检查目标：确认目录 build/src/V8CompileSamplerTests_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0243
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0244
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Debug`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0245
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/MinSizeRel`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0246
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0247
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0248
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/V8CompileSamplerTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8CompileSamplerTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0249
- 文件/目录范围：`build/src/V8FastNeutralizeTests_autogen`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0250
- 文件/目录范围：`build/src/V8FastNeutralizeTests_autogen/include_Release`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0251
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0252
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Debug`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0253
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/MinSizeRel`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0254
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0255
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0256
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/V8FastNeutralizeTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8FastNeutralizeTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0257
- 文件/目录范围：`build/src/V8RawSourceProbeTests_autogen`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0258
- 文件/目录范围：`build/src/V8RawSourceProbeTests_autogen/include_Release`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0259
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0260
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir/Debug`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0261
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir/MinSizeRel`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0262
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir/Release`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0263
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir/Release/V8RawSou.44F91E7F.tlog`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir/Release/V8RawSou.44F91E7F.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir/Release/V8RawSou.44F91E7F.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0264
- 文件/目录范围：`build/src/V8RawSourceProbeTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/V8RawSourceProbeTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8RawSourceProbeTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0265
- 文件/目录范围：`build/src/V8SourceNeutralizerTests_autogen`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0266
- 文件/目录范围：`build/src/V8SourceNeutralizerTests_autogen/include_Release`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0267
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0268
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir/Debug`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0269
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir/MinSizeRel`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0270
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir/Release`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0271
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir/Release/V8Source.ACA8BD6A.tlog`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir/Release/V8Source.ACA8BD6A.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir/Release/V8Source.ACA8BD6A.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0272
- 文件/目录范围：`build/src/V8SourceNeutralizerTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build/src/V8SourceNeutralizerTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/src/V8SourceNeutralizerTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0273
- 文件/目录范围：`build/Win32`
- 检查目标：确认目录 build/Win32 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0274
- 文件/目录范围：`build/Win32/Release`
- 检查目标：确认目录 build/Win32/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0275
- 文件/目录范围：`build/Win32/Release/ALL_BUILD`
- 检查目标：确认目录 build/Win32/Release/ALL_BUILD 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32/Release/ALL_BUILD' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0276
- 文件/目录范围：`build/Win32/Release/ALL_BUILD/ALL_BUILD.tlog`
- 检查目标：确认目录 build/Win32/Release/ALL_BUILD/ALL_BUILD.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32/Release/ALL_BUILD/ALL_BUILD.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0277
- 文件/目录范围：`build/Win32/Release/ZERO_CHECK`
- 检查目标：确认目录 build/Win32/Release/ZERO_CHECK 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32/Release/ZERO_CHECK' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0278
- 文件/目录范围：`build/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog`
- 检查目标：确认目录 build/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0279
- 文件/目录范围：`build/ZERO_CHECK.dir`
- 检查目标：确认目录 build/ZERO_CHECK.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ZERO_CHECK.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0280
- 文件/目录范围：`build/ZERO_CHECK.dir/Debug`
- 检查目标：确认目录 build/ZERO_CHECK.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ZERO_CHECK.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0281
- 文件/目录范围：`build/ZERO_CHECK.dir/MinSizeRel`
- 检查目标：确认目录 build/ZERO_CHECK.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ZERO_CHECK.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0282
- 文件/目录范围：`build/ZERO_CHECK.dir/Release`
- 检查目标：确认目录 build/ZERO_CHECK.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ZERO_CHECK.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0283
- 文件/目录范围：`build/ZERO_CHECK.dir/RelWithDebInfo`
- 检查目标：确认目录 build/ZERO_CHECK.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build/ZERO_CHECK.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0284
- 文件/目录范围：`build64`
- 检查目标：确认目录 build64 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0285
- 文件/目录范围：`build64/ALL_BUILD.dir`
- 检查目标：确认目录 build64/ALL_BUILD.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ALL_BUILD.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0286
- 文件/目录范围：`build64/ALL_BUILD.dir/Debug`
- 检查目标：确认目录 build64/ALL_BUILD.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ALL_BUILD.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0287
- 文件/目录范围：`build64/ALL_BUILD.dir/MinSizeRel`
- 检查目标：确认目录 build64/ALL_BUILD.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ALL_BUILD.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0288
- 文件/目录范围：`build64/ALL_BUILD.dir/Release`
- 检查目标：确认目录 build64/ALL_BUILD.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ALL_BUILD.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0289
- 文件/目录范围：`build64/ALL_BUILD.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/ALL_BUILD.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ALL_BUILD.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0290
- 文件/目录范围：`build64/CMakeFiles`
- 检查目标：确认目录 build64/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0291
- 文件/目录范围：`build64/CMakeFiles/0b1afa429b2355741d6920c8f9600ee6`
- 检查目标：确认目录 build64/CMakeFiles/0b1afa429b2355741d6920c8f9600ee6 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/0b1afa429b2355741d6920c8f9600ee6' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0292
- 文件/目录范围：`build64/CMakeFiles/4.3.0`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0293
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdC`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdC 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdC' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0294
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdC/Debug`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdC/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdC/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0295
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0296
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdC/tmp`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdC/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdC/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0297
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdCXX`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdCXX 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdCXX' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0298
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0299
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0300
- 文件/目录范围：`build64/CMakeFiles/4.3.0/CompilerIdCXX/tmp`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/CompilerIdCXX/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/CompilerIdCXX/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0301
- 文件/目录范围：`build64/CMakeFiles/4.3.0/VCTargetsPath`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/VCTargetsPath 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/VCTargetsPath' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0302
- 文件/目录范围：`build64/CMakeFiles/4.3.0/VCTargetsPath/x64`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/VCTargetsPath/x64 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/VCTargetsPath/x64' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0303
- 文件/目录范围：`build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0304
- 文件/目录范围：`build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug/VCTargetsPath.tlog`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug/VCTargetsPath.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/VCTargetsPath/x64/Debug/VCTargetsPath.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0305
- 文件/目录范围：`build64/CMakeFiles/4.3.0/x64`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/x64 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/x64' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0306
- 文件/目录范围：`build64/CMakeFiles/4.3.0/x64/Debug`
- 检查目标：确认目录 build64/CMakeFiles/4.3.0/x64/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/4.3.0/x64/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0307
- 文件/目录范围：`build64/CMakeFiles/fa44e05b66203fca8fde4fbddcd57b8a`
- 检查目标：确认目录 build64/CMakeFiles/fa44e05b66203fca8fde4fbddcd57b8a 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/fa44e05b66203fca8fde4fbddcd57b8a' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0308
- 文件/目录范围：`build64/CMakeFiles/pkgRedirects`
- 检查目标：确认目录 build64/CMakeFiles/pkgRedirects 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/CMakeFiles/pkgRedirects' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0309
- 文件/目录范围：`build64/src`
- 检查目标：确认目录 build64/src 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0310
- 文件/目录范围：`build64/src/CefHook_autogen`
- 检查目标：确认目录 build64/src/CefHook_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0311
- 文件/目录范围：`build64/src/CefHook_autogen/include_Release`
- 检查目标：确认目录 build64/src/CefHook_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0312
- 文件/目录范围：`build64/src/CefHook.dir`
- 检查目标：确认目录 build64/src/CefHook.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0313
- 文件/目录范围：`build64/src/CefHook.dir/Debug`
- 检查目标：确认目录 build64/src/CefHook.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0314
- 文件/目录范围：`build64/src/CefHook.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/CefHook.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0315
- 文件/目录范围：`build64/src/CefHook.dir/Release`
- 检查目标：确认目录 build64/src/CefHook.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0316
- 文件/目录范围：`build64/src/CefHook.dir/Release/CefHook.tlog`
- 检查目标：确认目录 build64/src/CefHook.dir/Release/CefHook.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir/Release/CefHook.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0317
- 文件/目录范围：`build64/src/CefHook.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/CefHook.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CefHook.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0318
- 文件/目录范围：`build64/src/ClientTests.dir`
- 检查目标：确认目录 build64/src/ClientTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ClientTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0319
- 文件/目录范围：`build64/src/ClientTests.dir/Debug`
- 检查目标：确认目录 build64/src/ClientTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ClientTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0320
- 文件/目录范围：`build64/src/ClientTests.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/ClientTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ClientTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0321
- 文件/目录范围：`build64/src/ClientTests.dir/Release`
- 检查目标：确认目录 build64/src/ClientTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ClientTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0322
- 文件/目录范围：`build64/src/ClientTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/ClientTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ClientTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0323
- 文件/目录范围：`build64/src/CMakeFiles`
- 检查目标：确认目录 build64/src/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0324
- 文件/目录范围：`build64/src/CMakeFiles/CefHook_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/CefHook_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/CefHook_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0325
- 文件/目录范围：`build64/src/CMakeFiles/ClientTests_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/ClientTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/ClientTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0326
- 文件/目录范围：`build64/src/CMakeFiles/Injector_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/Injector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/Injector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0327
- 文件/目录范围：`build64/src/CMakeFiles/ManualDllInjector_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/ManualDllInjector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/ManualDllInjector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0328
- 文件/目录范围：`build64/src/CMakeFiles/ManualDllInjectorGui_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/ManualDllInjectorGui_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/ManualDllInjectorGui_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0329
- 文件/目录范围：`build64/src/CMakeFiles/V8CompileSamplerTests_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/V8CompileSamplerTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/V8CompileSamplerTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0330
- 文件/目录范围：`build64/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0331
- 文件/目录范围：`build64/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0332
- 文件/目录范围：`build64/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir`
- 检查目标：确认目录 build64/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0333
- 文件/目录范围：`build64/src/Injector_autogen`
- 检查目标：确认目录 build64/src/Injector_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0334
- 文件/目录范围：`build64/src/Injector_autogen/include_Release`
- 检查目标：确认目录 build64/src/Injector_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0335
- 文件/目录范围：`build64/src/Injector_autogen/include_Release/5SSUPNRDSB`
- 检查目标：确认目录 build64/src/Injector_autogen/include_Release/5SSUPNRDSB 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen/include_Release/5SSUPNRDSB' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0336
- 文件/目录范围：`build64/src/Injector_autogen/include_Release/7CL2LH2DRO`
- 检查目标：确认目录 build64/src/Injector_autogen/include_Release/7CL2LH2DRO 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen/include_Release/7CL2LH2DRO' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0337
- 文件/目录范围：`build64/src/Injector_autogen/include_Release/UYX5XTB5RZ`
- 检查目标：确认目录 build64/src/Injector_autogen/include_Release/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen/include_Release/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0338
- 文件/目录范围：`build64/src/Injector_autogen/UYX5XTB5RZ`
- 检查目标：确认目录 build64/src/Injector_autogen/UYX5XTB5RZ 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector_autogen/UYX5XTB5RZ' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0339
- 文件/目录范围：`build64/src/Injector.dir`
- 检查目标：确认目录 build64/src/Injector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0340
- 文件/目录范围：`build64/src/Injector.dir/Debug`
- 检查目标：确认目录 build64/src/Injector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0341
- 文件/目录范围：`build64/src/Injector.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/Injector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0342
- 文件/目录范围：`build64/src/Injector.dir/Release`
- 检查目标：确认目录 build64/src/Injector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0343
- 文件/目录范围：`build64/src/Injector.dir/Release/Injector.tlog`
- 检查目标：确认目录 build64/src/Injector.dir/Release/Injector.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir/Release/Injector.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0344
- 文件/目录范围：`build64/src/Injector.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/Injector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Injector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0345
- 文件/目录范围：`build64/src/ManualDllInjector.dir`
- 检查目标：确认目录 build64/src/ManualDllInjector.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjector.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0346
- 文件/目录范围：`build64/src/ManualDllInjector.dir/Debug`
- 检查目标：确认目录 build64/src/ManualDllInjector.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjector.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0347
- 文件/目录范围：`build64/src/ManualDllInjector.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/ManualDllInjector.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjector.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0348
- 文件/目录范围：`build64/src/ManualDllInjector.dir/Release`
- 检查目标：确认目录 build64/src/ManualDllInjector.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjector.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0349
- 文件/目录范围：`build64/src/ManualDllInjector.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/ManualDllInjector.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjector.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0350
- 文件/目录范围：`build64/src/ManualDllInjectorGui.dir`
- 检查目标：确认目录 build64/src/ManualDllInjectorGui.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjectorGui.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0351
- 文件/目录范围：`build64/src/ManualDllInjectorGui.dir/Debug`
- 检查目标：确认目录 build64/src/ManualDllInjectorGui.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjectorGui.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0352
- 文件/目录范围：`build64/src/ManualDllInjectorGui.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/ManualDllInjectorGui.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjectorGui.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0353
- 文件/目录范围：`build64/src/ManualDllInjectorGui.dir/Release`
- 检查目标：确认目录 build64/src/ManualDllInjectorGui.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjectorGui.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0354
- 文件/目录范围：`build64/src/ManualDllInjectorGui.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/ManualDllInjectorGui.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/ManualDllInjectorGui.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0355
- 文件/目录范围：`build64/src/Release`
- 检查目标：确认目录 build64/src/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0356
- 文件/目录范围：`build64/src/V8CompileSamplerTests.dir`
- 检查目标：确认目录 build64/src/V8CompileSamplerTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8CompileSamplerTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0357
- 文件/目录范围：`build64/src/V8CompileSamplerTests.dir/Debug`
- 检查目标：确认目录 build64/src/V8CompileSamplerTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8CompileSamplerTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0358
- 文件/目录范围：`build64/src/V8CompileSamplerTests.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/V8CompileSamplerTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8CompileSamplerTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0359
- 文件/目录范围：`build64/src/V8CompileSamplerTests.dir/Release`
- 检查目标：确认目录 build64/src/V8CompileSamplerTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8CompileSamplerTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0360
- 文件/目录范围：`build64/src/V8CompileSamplerTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/V8CompileSamplerTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8CompileSamplerTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0361
- 文件/目录范围：`build64/src/V8FastNeutralizeTests.dir`
- 检查目标：确认目录 build64/src/V8FastNeutralizeTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8FastNeutralizeTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0362
- 文件/目录范围：`build64/src/V8FastNeutralizeTests.dir/Debug`
- 检查目标：确认目录 build64/src/V8FastNeutralizeTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8FastNeutralizeTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0363
- 文件/目录范围：`build64/src/V8FastNeutralizeTests.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/V8FastNeutralizeTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8FastNeutralizeTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0364
- 文件/目录范围：`build64/src/V8FastNeutralizeTests.dir/Release`
- 检查目标：确认目录 build64/src/V8FastNeutralizeTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8FastNeutralizeTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0365
- 文件/目录范围：`build64/src/V8FastNeutralizeTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/V8FastNeutralizeTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8FastNeutralizeTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0366
- 文件/目录范围：`build64/src/V8RawSourceProbeTests.dir`
- 检查目标：确认目录 build64/src/V8RawSourceProbeTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8RawSourceProbeTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0367
- 文件/目录范围：`build64/src/V8RawSourceProbeTests.dir/Debug`
- 检查目标：确认目录 build64/src/V8RawSourceProbeTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8RawSourceProbeTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0368
- 文件/目录范围：`build64/src/V8RawSourceProbeTests.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/V8RawSourceProbeTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8RawSourceProbeTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0369
- 文件/目录范围：`build64/src/V8RawSourceProbeTests.dir/Release`
- 检查目标：确认目录 build64/src/V8RawSourceProbeTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8RawSourceProbeTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0370
- 文件/目录范围：`build64/src/V8RawSourceProbeTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/V8RawSourceProbeTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8RawSourceProbeTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0371
- 文件/目录范围：`build64/src/V8SourceNeutralizerTests.dir`
- 检查目标：确认目录 build64/src/V8SourceNeutralizerTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8SourceNeutralizerTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0372
- 文件/目录范围：`build64/src/V8SourceNeutralizerTests.dir/Debug`
- 检查目标：确认目录 build64/src/V8SourceNeutralizerTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8SourceNeutralizerTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0373
- 文件/目录范围：`build64/src/V8SourceNeutralizerTests.dir/MinSizeRel`
- 检查目标：确认目录 build64/src/V8SourceNeutralizerTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8SourceNeutralizerTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0374
- 文件/目录范围：`build64/src/V8SourceNeutralizerTests.dir/Release`
- 检查目标：确认目录 build64/src/V8SourceNeutralizerTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8SourceNeutralizerTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0375
- 文件/目录范围：`build64/src/V8SourceNeutralizerTests.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/src/V8SourceNeutralizerTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/src/V8SourceNeutralizerTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0376
- 文件/目录范围：`build64/x64`
- 检查目标：确认目录 build64/x64 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/x64' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0377
- 文件/目录范围：`build64/x64/Release`
- 检查目标：确认目录 build64/x64/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/x64/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0378
- 文件/目录范围：`build64/x64/Release/ZERO_CHECK`
- 检查目标：确认目录 build64/x64/Release/ZERO_CHECK 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/x64/Release/ZERO_CHECK' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0379
- 文件/目录范围：`build64/x64/Release/ZERO_CHECK/ZERO_CHECK.tlog`
- 检查目标：确认目录 build64/x64/Release/ZERO_CHECK/ZERO_CHECK.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/x64/Release/ZERO_CHECK/ZERO_CHECK.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0380
- 文件/目录范围：`build64/ZERO_CHECK.dir`
- 检查目标：确认目录 build64/ZERO_CHECK.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ZERO_CHECK.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0381
- 文件/目录范围：`build64/ZERO_CHECK.dir/Debug`
- 检查目标：确认目录 build64/ZERO_CHECK.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ZERO_CHECK.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0382
- 文件/目录范围：`build64/ZERO_CHECK.dir/MinSizeRel`
- 检查目标：确认目录 build64/ZERO_CHECK.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ZERO_CHECK.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0383
- 文件/目录范围：`build64/ZERO_CHECK.dir/Release`
- 检查目标：确认目录 build64/ZERO_CHECK.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ZERO_CHECK.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0384
- 文件/目录范围：`build64/ZERO_CHECK.dir/RelWithDebInfo`
- 检查目标：确认目录 build64/ZERO_CHECK.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'build64/ZERO_CHECK.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0385
- 文件/目录范围：`cef`
- 检查目标：确认目录 cef 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0386
- 文件/目录范围：`cef/include`
- 检查目标：确认目录 cef/include 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0387
- 文件/目录范围：`cef/include/base`
- 检查目标：确认目录 cef/include/base 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/base' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0388
- 文件/目录范围：`cef/include/base/internal`
- 检查目标：确认目录 cef/include/base/internal 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/base/internal' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0389
- 文件/目录范围：`cef/include/capi`
- 检查目标：确认目录 cef/include/capi 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/capi' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0390
- 文件/目录范围：`cef/include/capi/test`
- 检查目标：确认目录 cef/include/capi/test 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/capi/test' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0391
- 文件/目录范围：`cef/include/capi/views`
- 检查目标：确认目录 cef/include/capi/views 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/capi/views' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0392
- 文件/目录范围：`cef/include/internal`
- 检查目标：确认目录 cef/include/internal 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/internal' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0393
- 文件/目录范围：`cef/include/net`
- 检查目标：确认目录 cef/include/net 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/net' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0394
- 文件/目录范围：`cef/include/net/base`
- 检查目标：确认目录 cef/include/net/base 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/net/base' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0395
- 文件/目录范围：`cef/include/test`
- 检查目标：确认目录 cef/include/test 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/test' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0396
- 文件/目录范围：`cef/include/views`
- 检查目标：确认目录 cef/include/views 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/views' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0397
- 文件/目录范围：`cef/include/wrapper`
- 检查目标：确认目录 cef/include/wrapper 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/include/wrapper' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0398
- 文件/目录范围：`cef/test`
- 检查目标：确认目录 cef/test 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/test' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0399
- 文件/目录范围：`cef/views`
- 检查目标：确认目录 cef/views 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'cef/views' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0400
- 文件/目录范围：`certs`
- 检查目标：确认目录 certs 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：运行数据、证书或发布包目录可能包含密钥、数据库或大文件，需隔离源码边界
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'certs' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0401
- 文件/目录范围：`chromium`
- 检查目标：确认目录 chromium 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'chromium' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0402
- 文件/目录范围：`chromium/net`
- 检查目标：确认目录 chromium/net 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'chromium/net' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0403
- 文件/目录范围：`chromium/net/base`
- 检查目标：确认目录 chromium/net/base 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'chromium/net/base' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0404
- 文件/目录范围：`client`
- 检查目标：确认目录 client 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：目录职责若未写入模块归属图，容易在并行重构中出现重复模块或错误分层
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0405
- 文件/目录范围：`client/build`
- 检查目标：确认目录 client/build 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0406
- 文件/目录范围：`client/build/ALL_BUILD.dir`
- 检查目标：确认目录 client/build/ALL_BUILD.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/ALL_BUILD.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0407
- 文件/目录范围：`client/build/ALL_BUILD.dir/Debug`
- 检查目标：确认目录 client/build/ALL_BUILD.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/ALL_BUILD.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0408
- 文件/目录范围：`client/build/ALL_BUILD.dir/MinSizeRel`
- 检查目标：确认目录 client/build/ALL_BUILD.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/ALL_BUILD.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0409
- 文件/目录范围：`client/build/ALL_BUILD.dir/Release`
- 检查目标：确认目录 client/build/ALL_BUILD.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/ALL_BUILD.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0410
- 文件/目录范围：`client/build/ALL_BUILD.dir/RelWithDebInfo`
- 检查目标：确认目录 client/build/ALL_BUILD.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/ALL_BUILD.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0411
- 文件/目录范围：`client/build/CMakeFiles`
- 检查目标：确认目录 client/build/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0412
- 文件/目录范围：`client/build/CMakeFiles/4.3.0`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0413
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdC`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdC 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdC' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0414
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdC/Debug`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdC/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdC/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0415
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0416
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdC/tmp`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdC/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdC/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0417
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdCXX`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdCXX 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdCXX' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0418
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0419
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0420
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/CompilerIdCXX/tmp`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/CompilerIdCXX/tmp 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/CompilerIdCXX/tmp' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0421
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/Debug`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0422
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/VCTargetsPath`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/VCTargetsPath 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/VCTargetsPath' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0423
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0424
- 文件/目录范围：`client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog`
- 检查目标：确认目录 client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0425
- 文件/目录范围：`client/build/CMakeFiles/62a0082e4b1a52a65dd4d65b5f5c23d8`
- 检查目标：确认目录 client/build/CMakeFiles/62a0082e4b1a52a65dd4d65b5f5c23d8 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/62a0082e4b1a52a65dd4d65b5f5c23d8' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0426
- 文件/目录范围：`client/build/CMakeFiles/f26b591db126c5731c31aa1b7e2a7a4b`
- 检查目标：确认目录 client/build/CMakeFiles/f26b591db126c5731c31aa1b7e2a7a4b 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/f26b591db126c5731c31aa1b7e2a7a4b' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0427
- 文件/目录范围：`client/build/CMakeFiles/pkgRedirects`
- 检查目标：确认目录 client/build/CMakeFiles/pkgRedirects 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/CMakeFiles/pkgRedirects' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0428
- 文件/目录范围：`client/build/Release`
- 检查目标：确认目录 client/build/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0429
- 文件/目录范围：`client/build/src`
- 检查目标：确认目录 client/build/src 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0430
- 文件/目录范围：`client/build/src/CefHook_autogen`
- 检查目标：确认目录 client/build/src/CefHook_autogen 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook_autogen' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0431
- 文件/目录范围：`client/build/src/CefHook_autogen/include_Release`
- 检查目标：确认目录 client/build/src/CefHook_autogen/include_Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook_autogen/include_Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0432
- 文件/目录范围：`client/build/src/CefHook.dir`
- 检查目标：确认目录 client/build/src/CefHook.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0433
- 文件/目录范围：`client/build/src/CefHook.dir/Debug`
- 检查目标：确认目录 client/build/src/CefHook.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0434
- 文件/目录范围：`client/build/src/CefHook.dir/MinSizeRel`
- 检查目标：确认目录 client/build/src/CefHook.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0435
- 文件/目录范围：`client/build/src/CefHook.dir/Release`
- 检查目标：确认目录 client/build/src/CefHook.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0436
- 文件/目录范围：`client/build/src/CefHook.dir/Release/CefHook.tlog`
- 检查目标：确认目录 client/build/src/CefHook.dir/Release/CefHook.tlog 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir/Release/CefHook.tlog' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0437
- 文件/目录范围：`client/build/src/CefHook.dir/RelWithDebInfo`
- 检查目标：确认目录 client/build/src/CefHook.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CefHook.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0438
- 文件/目录范围：`client/build/src/ClientTests.dir`
- 检查目标：确认目录 client/build/src/ClientTests.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/ClientTests.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0439
- 文件/目录范围：`client/build/src/ClientTests.dir/Debug`
- 检查目标：确认目录 client/build/src/ClientTests.dir/Debug 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/ClientTests.dir/Debug' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0440
- 文件/目录范围：`client/build/src/ClientTests.dir/MinSizeRel`
- 检查目标：确认目录 client/build/src/ClientTests.dir/MinSizeRel 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/ClientTests.dir/MinSizeRel' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0441
- 文件/目录范围：`client/build/src/ClientTests.dir/Release`
- 检查目标：确认目录 client/build/src/ClientTests.dir/Release 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/ClientTests.dir/Release' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0442
- 文件/目录范围：`client/build/src/ClientTests.dir/RelWithDebInfo`
- 检查目标：确认目录 client/build/src/ClientTests.dir/RelWithDebInfo 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/ClientTests.dir/RelWithDebInfo' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0443
- 文件/目录范围：`client/build/src/CMakeFiles`
- 检查目标：确认目录 client/build/src/CMakeFiles 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0444
- 文件/目录范围：`client/build/src/CMakeFiles/CefHook_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/CefHook_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/CefHook_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0445
- 文件/目录范围：`client/build/src/CMakeFiles/ClientTests_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/ClientTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/ClientTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0446
- 文件/目录范围：`client/build/src/CMakeFiles/Injector_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/Injector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/Injector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0447
- 文件/目录范围：`client/build/src/CMakeFiles/ManualDllInjector_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/ManualDllInjector_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/ManualDllInjector_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0448
- 文件/目录范围：`client/build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0449
- 文件/目录范围：`client/build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0450
- 文件/目录范围：`client/build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir`
- 检查目标：确认目录 client/build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir 的架构归属、源码/生成/运行数据属性、入口或依赖边界
- 潜在问题：生成产物目录可能被源码、脚本或发布流程误引用，且可能污染版本控制
- 修改策略：在 PROJECT_AUDIT_MAP/MODULE_OWNERSHIP_MAP 中标记归属；发现跨模块或生成产物问题时写入 CROSS_MODULE_REPORT/PARALLEL_HANDOFF；不直接移动业务目录
- 验证方式：Get-ChildItem 'client/build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir' 与 rg 对该目录引用路径、import/include/build 脚本引用进行交叉验证
- 完成状态：已完成

## S01-0451
- 文件/目录范围：`.github/workflows/release.yml`
- 检查目标：检查文档/API/ADR 是否覆盖实际入口、配置、模块边界和迁移约束
- 潜在问题：文档可能编码异常、过期或遗漏真实路由/环境变量
- 修改策略：在审计文档中标记差距并提出修订项
- 验证方式：rg 代码入口与文档条目交叉验证
- 完成状态：已完成

## S01-0452
- 文件/目录范围：`.gitignore`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0453
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content/authorization-control-plane.html`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0454
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content/authorization-ui-scope.html`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0455
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content/control-plane-gap.html`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0456
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content/full-rewrite-decomposition.html`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0457
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/content/waiting-for-spec-review.html`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0458
- 文件/目录范围：`.superpowers/brainstorm/admin-architecture-20260702-2235/state/server-info`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0459
- 文件/目录范围：`.vscode/settings.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0460
- 文件/目录范围：`AutoExam.js`
- 检查目标：检查 TypeScript/JavaScript 入口、DOM 挂载、API 调用和共享模块边界
- 潜在问题：前端模块可能混合 admin/agent 责任或依赖未文档化路由
- 修改策略：记录前端入口和边界，不直接重写 UI
- 验证方式：npm run typecheck
- 完成状态：已完成

## S01-0461
- 文件/目录范围：`build-status-review/ALL_BUILD.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0462
- 文件/目录范围：`build-status-review/ALL_BUILD.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0463
- 文件/目录范围：`build-status-review/CefHook.sln`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0464
- 文件/目录范围：`build-status-review/cmake_install.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0465
- 文件/目录范围：`build-status-review/CMakeCache.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0466
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeCCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0467
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeCXXCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0468
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeDetermineCompilerABI_C.bin`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0469
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeDetermineCompilerABI_CXX.bin`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0470
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeRCCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0471
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CMakeSystem.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0472
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/CMakeCCompilerId.c`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0473
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/CompilerIdC.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0474
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/CompilerIdC.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0475
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CMakeCCompilerId.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0476
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0477
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0478
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0479
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0480
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0481
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CompilerIdC.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0482
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0483
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0484
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0485
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0486
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdC/Debug/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0487
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/CMakeCXXCompilerId.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0488
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/CompilerIdCXX.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0489
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/CompilerIdCXX.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0490
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CMakeCXXCompilerId.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0491
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0492
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0493
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0494
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0495
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0496
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CompilerIdCXX.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0497
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0498
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0499
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0500
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0501
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/CompilerIdCXX/Debug/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0502
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0503
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0504
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0505
- 文件/目录范围：`build-status-review/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog/VCTargetsPath.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0506
- 文件/目录范围：`build-status-review/CMakeFiles/468707a8d1ff2952380d1f29a6746929/qrc_ui_resources.cpp.rule`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0507
- 文件/目录范围：`build-status-review/CMakeFiles/50bdb25440a67914ffc6b48028979208/generate.stamp.rule`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0508
- 文件/目录范围：`build-status-review/CMakeFiles/cmake.check_cache`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0509
- 文件/目录范围：`build-status-review/CMakeFiles/CMakeConfigureLog.yaml`
- 检查目标：检查文档/API/ADR 是否覆盖实际入口、配置、模块边界和迁移约束
- 潜在问题：文档可能编码异常、过期或遗漏真实路由/环境变量
- 修改策略：在审计文档中标记差距并提出修订项
- 验证方式：rg 代码入口与文档条目交叉验证
- 完成状态：已完成

## S01-0510
- 文件/目录范围：`build-status-review/CMakeFiles/generate.stamp`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0511
- 文件/目录范围：`build-status-review/CMakeFiles/generate.stamp.depend`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0512
- 文件/目录范围：`build-status-review/CMakeFiles/generate.stamp.list`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0513
- 文件/目录范围：`build-status-review/CMakeFiles/InstallScripts.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0514
- 文件/目录范围：`build-status-review/CMakeFiles/TargetDirectories.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0515
- 文件/目录范围：`build-status-review/src/CefHook.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0516
- 文件/目录范围：`build-status-review/src/CefHook.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0517
- 文件/目录范围：`build-status-review/src/cmake_install.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0518
- 文件/目录范围：`build-status-review/src/CMakeFiles/CefHook_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0519
- 文件/目录范围：`build-status-review/src/CMakeFiles/generate.stamp`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0520
- 文件/目录范围：`build-status-review/src/CMakeFiles/generate.stamp.depend`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0521
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0522
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0523
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Info.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0524
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Lock.lock`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0525
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Used_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0526
- 文件/目录范围：`build-status-review/src/CMakeFiles/Injector_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0527
- 文件/目录范围：`build-status-review/src/CMakeFiles/ManualDllInjector_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0528
- 文件/目录范围：`build-status-review/src/CMakeFiles/ManualDllInjectorGui_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0529
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/5SSUPNRDSB/moc_workers.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0530
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/5SSUPNRDSB/moc_workers.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0531
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_animated_button.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0532
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_animated_button.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0533
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_inline_status.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0534
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_inline_status.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0535
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_main_window.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0536
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_main_window.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0537
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_title_bar.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0538
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_title_bar.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0539
- 文件/目录范围：`build-status-review/src/Injector_autogen/include_Release/UYX5XTB5RZ/qrc_ui_resources_CMAKE_.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0540
- 文件/目录范围：`build-status-review/src/Injector_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0541
- 文件/目录范围：`build-status-review/src/Injector_autogen/UYX5XTB5RZ/qrc_ui_resources.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0542
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/api_client.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0543
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/CX_LingQiao.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0544
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/CX_LingQiao.iobj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0545
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/injector_qt.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0546
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0547
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0548
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0549
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0550
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0551
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0552
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0553
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/Injector.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0554
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0555
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0556
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0557
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0558
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/rc.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0559
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/rc.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0560
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/Injector.tlog/rc.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0561
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0562
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/qrc_ui_resources.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0563
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/update_controller.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0564
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0565
- 文件/目录范围：`build-status-review/src/Injector.dir/Release/version.res`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0566
- 文件/目录范围：`build-status-review/src/Injector.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0567
- 文件/目录范围：`build-status-review/src/Injector.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0568
- 文件/目录范围：`build-status-review/src/ManualDllInjector.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0569
- 文件/目录范围：`build-status-review/src/ManualDllInjector.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0570
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0571
- 文件/目录范围：`build-status-review/src/ManualDllInjectorGui.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0572
- 文件/目录范围：`build-status-review/src/Release/CX_LingQiao.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0573
- 文件/目录范围：`build-status-review/src/Release/D3Dcompiler_47.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0574
- 文件/目录范围：`build-status-review/src/Release/iconengines/qsvgicon.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0575
- 文件/目录范围：`build-status-review/src/Release/imageformats/qgif.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0576
- 文件/目录范围：`build-status-review/src/Release/imageformats/qicns.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0577
- 文件/目录范围：`build-status-review/src/Release/imageformats/qico.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0578
- 文件/目录范围：`build-status-review/src/Release/imageformats/qjpeg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0579
- 文件/目录范围：`build-status-review/src/Release/imageformats/qsvg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0580
- 文件/目录范围：`build-status-review/src/Release/imageformats/qtga.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0581
- 文件/目录范围：`build-status-review/src/Release/imageformats/qtiff.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0582
- 文件/目录范围：`build-status-review/src/Release/imageformats/qwbmp.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0583
- 文件/目录范围：`build-status-review/src/Release/imageformats/qwebp.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0584
- 文件/目录范围：`build-status-review/src/Release/libEGL.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0585
- 文件/目录范围：`build-status-review/src/Release/libGLESv2.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0586
- 文件/目录范围：`build-status-review/src/Release/opengl32sw.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0587
- 文件/目录范围：`build-status-review/src/Release/platforms/qwindows.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0588
- 文件/目录范围：`build-status-review/src/Release/Qt5Core.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0589
- 文件/目录范围：`build-status-review/src/Release/Qt5Gui.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0590
- 文件/目录范围：`build-status-review/src/Release/Qt5Svg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0591
- 文件/目录范围：`build-status-review/src/Release/Qt5Widgets.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0592
- 文件/目录范围：`build-status-review/src/Release/styles/qwindowsvistastyle.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0593
- 文件/目录范围：`build-status-review/src/version.rc`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0594
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0595
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0596
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0597
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0598
- 文件/目录范围：`build-status-review/Win32/Release/ZERO_CHECK/ZERO_CHECK.tlog/ZERO_CHECK.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0599
- 文件/目录范围：`build-status-review/ZERO_CHECK.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0600
- 文件/目录范围：`build-status-review/ZERO_CHECK.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0601
- 文件/目录范围：`build.bat`
- 检查目标：检查脚本/安装器入口、参数、环境变量、产物路径和签名链路
- 潜在问题：脚本可能硬编码本机路径、架构、证书或旧产物路径
- 修改策略：写入部署入口清单和 handoff，不直接重写发布流程
- 验证方式：静态执行 dry-run 或读取参数与产物路径
- 完成状态：已完成

## S01-0602
- 文件/目录范围：`build/.cmake/api/v1/query/client-vscode/query.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0603
- 文件/目录范围：`build/.cmake/api/v1/reply/cache-v2-08b1dff8d564fd1e57f9.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0604
- 文件/目录范围：`build/.cmake/api/v1/reply/cmakeFiles-v1-3bb66acdf33dfdbac6b8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0605
- 文件/目录范围：`build/.cmake/api/v1/reply/codemodel-v2-b76d9fa6ee58f4599527.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0606
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-.-Debug-f040eec181a89de70dbe.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0607
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-.-MinSizeRel-f040eec181a89de70dbe.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0608
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-.-Release-f040eec181a89de70dbe.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0609
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-.-RelWithDebInfo-f040eec181a89de70dbe.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0610
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-src-Debug-4002d1304ad276179d0e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0611
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-src-MinSizeRel-4002d1304ad276179d0e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0612
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-src-Release-4002d1304ad276179d0e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0613
- 文件/目录范围：`build/.cmake/api/v1/reply/directory-src-RelWithDebInfo-4002d1304ad276179d0e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0614
- 文件/目录范围：`build/.cmake/api/v1/reply/index-2026-07-03T15-49-06-0390.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0615
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ALL_BUILD-Debug-730b5d238d6a960f4fc7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0616
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ALL_BUILD-MinSizeRel-730b5d238d6a960f4fc7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0617
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ALL_BUILD-Release-730b5d238d6a960f4fc7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0618
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ALL_BUILD-RelWithDebInfo-730b5d238d6a960f4fc7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0619
- 文件/目录范围：`build/.cmake/api/v1/reply/target-CefHook-Debug-95c0aa722889ae2d30fb.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0620
- 文件/目录范围：`build/.cmake/api/v1/reply/target-CefHook-MinSizeRel-ce7108b8c962fedd5059.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0621
- 文件/目录范围：`build/.cmake/api/v1/reply/target-CefHook-Release-474a6a54553e06bedccd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0622
- 文件/目录范围：`build/.cmake/api/v1/reply/target-CefHook-RelWithDebInfo-f1042078e1fef394729e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0623
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ClientTests-Debug-083b7c86b22bf121f435.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0624
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ClientTests-MinSizeRel-30ca06839f9566b535ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0625
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ClientTests-Release-1da3db419eb5cd81188f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0626
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ClientTests-RelWithDebInfo-aa33049edbb5dce49929.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0627
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Injector-Debug-9d21441650c0d456658d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0628
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Injector-MinSizeRel-518d1ba85756db3c089e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0629
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Injector-Release-cf34f95347c3930a0ede.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0630
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Injector-RelWithDebInfo-f65c9bc4258d90cda421.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0631
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjector-Debug-2098adedca5fc6630d2d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0632
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjector-MinSizeRel-b488a3bb5da1ca7e33c7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0633
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjector-Release-56015e631ff81da2e0ce.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0634
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjector-RelWithDebInfo-a62f72cf766668fdb3ac.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0635
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjectorGui-Debug-c8f600e5f9f49dfcfc7a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0636
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjectorGui-MinSizeRel-fccc8c5b09e5822d617d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0637
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjectorGui-Release-4cb67fd5760edb411d53.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0638
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ManualDllInjectorGui-RelWithDebInfo-818940eb96aaf19d45f3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0639
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Concurrent-Debug-5aa5208eccb3ee6278ee.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0640
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Concurrent-MinSizeRel-5aa5208eccb3ee6278ee.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0641
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Concurrent-Release-5aa5208eccb3ee6278ee.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0642
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Concurrent-RelWithDebInfo-5aa5208eccb3ee6278ee.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0643
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__ConcurrentPrivate-Debug-943ded704f2cc60f88a6.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0644
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__ConcurrentPrivate-MinSizeRel-943ded704f2cc60f88a6.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0645
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__ConcurrentPrivate-Release-943ded704f2cc60f88a6.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0646
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__ConcurrentPrivate-RelWithDebInfo-943ded704f2cc60f88a6.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0647
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Core-Debug-4cf1d2b4dd068b96ec48.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0648
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Core-MinSizeRel-4cf1d2b4dd068b96ec48.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0649
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Core-Release-4cf1d2b4dd068b96ec48.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0650
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Core-RelWithDebInfo-4cf1d2b4dd068b96ec48.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0651
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__CorePrivate-Debug-85f732623506767a83bd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0652
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__CorePrivate-MinSizeRel-85f732623506767a83bd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0653
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__CorePrivate-Release-85f732623506767a83bd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0654
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__CorePrivate-RelWithDebInfo-85f732623506767a83bd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0655
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Gui-Debug-f20bf593362eac2488a9.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0656
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Gui-MinSizeRel-f20bf593362eac2488a9.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0657
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Gui-Release-f20bf593362eac2488a9.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0658
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Gui-RelWithDebInfo-f20bf593362eac2488a9.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0659
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__GuiPrivate-Debug-d700ae0cf4ea63e5b80f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0660
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__GuiPrivate-MinSizeRel-d700ae0cf4ea63e5b80f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0661
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__GuiPrivate-Release-d700ae0cf4ea63e5b80f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0662
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__GuiPrivate-RelWithDebInfo-d700ae0cf4ea63e5b80f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0663
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__moc-Debug-37ab49dcf236e21e4b33.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0664
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__moc-MinSizeRel-37ab49dcf236e21e4b33.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0665
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__moc-Release-37ab49dcf236e21e4b33.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0666
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__moc-RelWithDebInfo-37ab49dcf236e21e4b33.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0667
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Network-Debug-8c384fed14e6558a5b07.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0668
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Network-MinSizeRel-8c384fed14e6558a5b07.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0669
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Network-Release-8c384fed14e6558a5b07.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0670
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Network-RelWithDebInfo-8c384fed14e6558a5b07.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0671
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__NetworkPrivate-Debug-75514064d3029e56720e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0672
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__NetworkPrivate-MinSizeRel-75514064d3029e56720e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0673
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__NetworkPrivate-Release-75514064d3029e56720e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0674
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__NetworkPrivate-RelWithDebInfo-75514064d3029e56720e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0675
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__qmake-Debug-05b7eaf9ba6b0f23ec20.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0676
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__qmake-MinSizeRel-05b7eaf9ba6b0f23ec20.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0677
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__qmake-Release-05b7eaf9ba6b0f23ec20.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0678
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__qmake-RelWithDebInfo-05b7eaf9ba6b0f23ec20.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0679
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__rcc-Debug-194931ffe16f5eaa602a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0680
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__rcc-MinSizeRel-194931ffe16f5eaa602a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0681
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__rcc-Release-194931ffe16f5eaa602a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0682
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__rcc-RelWithDebInfo-194931ffe16f5eaa602a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0683
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__uic-Debug-52a048fd5e9a10074151.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0684
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__uic-MinSizeRel-52a048fd5e9a10074151.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0685
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__uic-Release-52a048fd5e9a10074151.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0686
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__uic-RelWithDebInfo-52a048fd5e9a10074151.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0687
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Widgets-Debug-44f130884438ed799d3c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0688
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Widgets-MinSizeRel-44f130884438ed799d3c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0689
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Widgets-Release-44f130884438ed799d3c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0690
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__Widgets-RelWithDebInfo-44f130884438ed799d3c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0691
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WidgetsPrivate-Debug-2a4422e374a72919509f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0692
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WidgetsPrivate-MinSizeRel-2a4422e374a72919509f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0693
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WidgetsPrivate-Release-2a4422e374a72919509f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0694
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WidgetsPrivate-RelWithDebInfo-2a4422e374a72919509f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0695
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtras-Debug-c1c3caacaa84f8c8f096.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0696
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtras-MinSizeRel-c1c3caacaa84f8c8f096.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0697
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtras-Release-c1c3caacaa84f8c8f096.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0698
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtras-RelWithDebInfo-c1c3caacaa84f8c8f096.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0699
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtrasPrivate-Debug-8caa9abfe369de216f02.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0700
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtrasPrivate-MinSizeRel-8caa9abfe369de216f02.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0701
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtrasPrivate-Release-8caa9abfe369de216f02.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0702
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinExtrasPrivate-RelWithDebInfo-8caa9abfe369de216f02.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0703
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinMain-Debug-5205c710973b5c7ac3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0704
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinMain-MinSizeRel-5205c710973b5c7ac3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0705
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinMain-Release-5205c710973b5c7ac3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0706
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt__WinMain-RelWithDebInfo-5205c710973b5c7ac3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0707
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Concurrent-Debug-2f88a87267964c7f67b6.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0708
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Concurrent-MinSizeRel-946543dae601baecc49e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0709
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Concurrent-Release-946543dae601baecc49e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0710
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Concurrent-RelWithDebInfo-946543dae601baecc49e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0711
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__ConcurrentPrivate-Debug-bd6d430419a74e9e29ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0712
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__ConcurrentPrivate-MinSizeRel-bd6d430419a74e9e29ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0713
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__ConcurrentPrivate-Release-bd6d430419a74e9e29ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0714
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__ConcurrentPrivate-RelWithDebInfo-bd6d430419a74e9e29ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0715
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Core-Debug-8fab53d55a586bbec54e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0716
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Core-MinSizeRel-8b358a12d31f1fee1871.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0717
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Core-Release-8b358a12d31f1fee1871.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0718
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Core-RelWithDebInfo-8b358a12d31f1fee1871.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0719
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__CorePrivate-Debug-bd05fd7d3bf3e1d59cb8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0720
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__CorePrivate-MinSizeRel-bd05fd7d3bf3e1d59cb8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0721
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__CorePrivate-Release-bd05fd7d3bf3e1d59cb8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0722
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__CorePrivate-RelWithDebInfo-bd05fd7d3bf3e1d59cb8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0723
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_EGL-Debug-15c9076d13cd788f81c0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0724
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_EGL-MinSizeRel-473df64f066e3ffd607e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0725
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_EGL-Release-473df64f066e3ffd607e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0726
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_EGL-RelWithDebInfo-473df64f066e3ffd607e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0727
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_GLESv2-Debug-42900784f7fb83b3677e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0728
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_GLESv2-MinSizeRel-433aeef772c64bd9c5e3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0729
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_GLESv2-Release-433aeef772c64bd9c5e3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0730
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui_GLESv2-RelWithDebInfo-433aeef772c64bd9c5e3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0731
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui-Debug-e5f5cd77487f5dccd836.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0732
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui-MinSizeRel-a99337e37f427e4b8bc0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0733
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui-Release-a99337e37f427e4b8bc0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0734
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Gui-RelWithDebInfo-a99337e37f427e4b8bc0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0735
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__GuiPrivate-Debug-0ce491095c053469a722.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0736
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__GuiPrivate-MinSizeRel-0ce491095c053469a722.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0737
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__GuiPrivate-Release-0ce491095c053469a722.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0738
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__GuiPrivate-RelWithDebInfo-0ce491095c053469a722.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0739
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__moc-Debug-93f32d8e8390151639c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0740
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__moc-MinSizeRel-93f32d8e8390151639c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0741
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__moc-Release-93f32d8e8390151639c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0742
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__moc-RelWithDebInfo-93f32d8e8390151639c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0743
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Network-Debug-451ce183afeb1b9c984c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0744
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Network-MinSizeRel-b615dd92e6ef0a60b5ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0745
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Network-Release-b615dd92e6ef0a60b5ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0746
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Network-RelWithDebInfo-b615dd92e6ef0a60b5ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0747
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__NetworkPrivate-Debug-69be40a4263f17fce162.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0748
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__NetworkPrivate-MinSizeRel-69be40a4263f17fce162.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0749
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__NetworkPrivate-Release-69be40a4263f17fce162.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0750
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__NetworkPrivate-RelWithDebInfo-69be40a4263f17fce162.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0751
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGenericEnginePlugin-Debug-1cb1c51c811e2e7b1105.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0752
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGenericEnginePlugin-MinSizeRel-4c26f62eb22a49a66072.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0753
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGenericEnginePlugin-Release-4c26f62eb22a49a66072.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0754
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGenericEnginePlugin-RelWithDebInfo-4c26f62eb22a49a66072.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0755
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGifPlugin-Debug-3778ad762d672b63ab76.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0756
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGifPlugin-MinSizeRel-d728a38dd3646c620476.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0757
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGifPlugin-Release-d728a38dd3646c620476.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0758
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QGifPlugin-RelWithDebInfo-d728a38dd3646c620476.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0759
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICNSPlugin-Debug-dda6b4a41923ad7142ab.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0760
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICNSPlugin-MinSizeRel-3bed47ffe13428b05ca4.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0761
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICNSPlugin-Release-3bed47ffe13428b05ca4.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0762
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICNSPlugin-RelWithDebInfo-3bed47ffe13428b05ca4.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0763
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICOPlugin-Debug-07bfe842738143a218c5.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0764
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICOPlugin-MinSizeRel-0c6bd0b78d8802118962.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0765
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICOPlugin-Release-0c6bd0b78d8802118962.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0766
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QICOPlugin-RelWithDebInfo-0c6bd0b78d8802118962.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0767
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QJpegPlugin-Debug-8454f664326b780f6140.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0768
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QJpegPlugin-MinSizeRel-8a04a0f7ade8aa101fdb.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0769
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QJpegPlugin-Release-8a04a0f7ade8aa101fdb.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0770
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QJpegPlugin-RelWithDebInfo-8a04a0f7ade8aa101fdb.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0771
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__qmake-Debug-e4454f0eb7d177121d50.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0772
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__qmake-MinSizeRel-e4454f0eb7d177121d50.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0773
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__qmake-Release-e4454f0eb7d177121d50.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0774
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__qmake-RelWithDebInfo-e4454f0eb7d177121d50.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0775
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QMinimalIntegrationPlugin-Debug-9c69cbc446471f0d0a17.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0776
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QMinimalIntegrationPlugin-MinSizeRel-1b20bb90cf49d62f3bd7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0777
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QMinimalIntegrationPlugin-Release-1b20bb90cf49d62f3bd7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0778
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QMinimalIntegrationPlugin-RelWithDebInfo-1b20bb90cf49d62f3bd7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0779
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QOffscreenIntegrationPlugin-Debug-64f6bb9058965fd3a114.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0780
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QOffscreenIntegrationPlugin-MinSizeRel-3054eb99fd1729df95ea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0781
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QOffscreenIntegrationPlugin-Release-3054eb99fd1729df95ea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0782
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QOffscreenIntegrationPlugin-RelWithDebInfo-3054eb99fd1729df95ea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0783
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgIconPlugin-Debug-0afde6beb3feddde0312.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0784
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgIconPlugin-MinSizeRel-5e2d740978596a70a759.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0785
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgIconPlugin-Release-5e2d740978596a70a759.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0786
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgIconPlugin-RelWithDebInfo-5e2d740978596a70a759.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0787
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgPlugin-Debug-7ce1fa008875c2804628.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0788
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgPlugin-MinSizeRel-4874e13be3338c2b0181.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0789
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgPlugin-Release-4874e13be3338c2b0181.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0790
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QSvgPlugin-RelWithDebInfo-4874e13be3338c2b0181.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0791
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTgaPlugin-Debug-1c6d89e4df868d664acd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0792
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTgaPlugin-MinSizeRel-9ab68a7b8f4b26ab9227.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0793
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTgaPlugin-Release-9ab68a7b8f4b26ab9227.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0794
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTgaPlugin-RelWithDebInfo-9ab68a7b8f4b26ab9227.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0795
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTiffPlugin-Debug-391c8fdadcf3f36f90fe.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0796
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTiffPlugin-MinSizeRel-80ec6167565d561a5fea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0797
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTiffPlugin-Release-80ec6167565d561a5fea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0798
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTiffPlugin-RelWithDebInfo-80ec6167565d561a5fea.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0799
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTuioTouchPlugin-Debug-40ee898d4d9b0126b44f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0800
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTuioTouchPlugin-MinSizeRel-e95d0e1ae5a153e52ae2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0801
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTuioTouchPlugin-Release-e95d0e1ae5a153e52ae2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0802
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QTuioTouchPlugin-RelWithDebInfo-e95d0e1ae5a153e52ae2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0803
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWbmpPlugin-Debug-b764e8417eea67ee36be.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0804
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWbmpPlugin-MinSizeRel-2933c6e84d65e1c0e7b0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0805
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWbmpPlugin-Release-2933c6e84d65e1c0e7b0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0806
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWbmpPlugin-RelWithDebInfo-2933c6e84d65e1c0e7b0.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0807
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWebpPlugin-Debug-0d150da3323a9c787832.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0808
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWebpPlugin-MinSizeRel-7c91d21010f224968463.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0809
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWebpPlugin-Release-7c91d21010f224968463.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0810
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWebpPlugin-RelWithDebInfo-7c91d21010f224968463.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0811
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsDirect2DIntegrationPlugin-Debug-0a1b5767e1ba4e6dc7a5.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0812
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsDirect2DIntegrationPlugin-MinSizeRel-ce4f88638dd8d0060cac.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0813
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsDirect2DIntegrationPlugin-Release-ce4f88638dd8d0060cac.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0814
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsDirect2DIntegrationPlugin-RelWithDebInfo-ce4f88638dd8d0060cac.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0815
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsIntegrationPlugin-Debug-c64c9ccb8e8547ea4167.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0816
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsIntegrationPlugin-MinSizeRel-a73d1d65d1c35f2fe40b.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0817
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsIntegrationPlugin-Release-a73d1d65d1c35f2fe40b.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0818
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsIntegrationPlugin-RelWithDebInfo-a73d1d65d1c35f2fe40b.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0819
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsVistaStylePlugin-Debug-e46b9b4ec2715adf575e.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0820
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsVistaStylePlugin-MinSizeRel-12b8a212b44986b1dbe2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0821
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsVistaStylePlugin-Release-12b8a212b44986b1dbe2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0822
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QWindowsVistaStylePlugin-RelWithDebInfo-12b8a212b44986b1dbe2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0823
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QXdgDesktopPortalThemePlugin-Debug-9b12b08abdc10687832b.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0824
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QXdgDesktopPortalThemePlugin-MinSizeRel-d8f250f88fb81a79b3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0825
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QXdgDesktopPortalThemePlugin-Release-d8f250f88fb81a79b3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0826
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__QXdgDesktopPortalThemePlugin-RelWithDebInfo-d8f250f88fb81a79b3cd.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0827
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__rcc-Debug-a728224f994601e31206.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0828
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__rcc-MinSizeRel-a728224f994601e31206.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0829
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__rcc-Release-a728224f994601e31206.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0830
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__rcc-RelWithDebInfo-a728224f994601e31206.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0831
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__uic-Debug-e4c445cb42599fc945c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0832
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__uic-MinSizeRel-e4c445cb42599fc945c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0833
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__uic-Release-e4c445cb42599fc945c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0834
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__uic-RelWithDebInfo-e4c445cb42599fc945c3.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0835
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Widgets-Debug-ce669863a58543799e27.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0836
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Widgets-MinSizeRel-4178bfb693e90045ab4c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0837
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Widgets-Release-4178bfb693e90045ab4c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0838
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__Widgets-RelWithDebInfo-4178bfb693e90045ab4c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0839
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WidgetsPrivate-Debug-9c7f065273427075d92a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0840
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WidgetsPrivate-MinSizeRel-9c7f065273427075d92a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0841
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WidgetsPrivate-Release-9c7f065273427075d92a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0842
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WidgetsPrivate-RelWithDebInfo-9c7f065273427075d92a.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0843
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtras-Debug-3cddfaca3684c5c1a556.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0844
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtras-MinSizeRel-07111b3a2b8d0f55f05d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0845
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtras-Release-07111b3a2b8d0f55f05d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0846
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtras-RelWithDebInfo-07111b3a2b8d0f55f05d.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0847
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtrasPrivate-Debug-b71556b47c7f2180dfc1.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0848
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtrasPrivate-MinSizeRel-b71556b47c7f2180dfc1.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0849
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtrasPrivate-Release-b71556b47c7f2180dfc1.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0850
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinExtrasPrivate-RelWithDebInfo-b71556b47c7f2180dfc1.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0851
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinMain-Debug-3d3cb5add1a48031e873.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0852
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinMain-MinSizeRel-5acb5d2ee0845ca8e451.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0853
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinMain-Release-5acb5d2ee0845ca8e451.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0854
- 文件/目录范围：`build/.cmake/api/v1/reply/target-Qt5__WinMain-RelWithDebInfo-5acb5d2ee0845ca8e451.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0855
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8CompileSamplerTests-Debug-1ed37bf84f95bcad1c3f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0856
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8CompileSamplerTests-MinSizeRel-347c6c3499b0bf1a4947.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0857
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8CompileSamplerTests-Release-813c87c1c2395b7d9a8b.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0858
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8CompileSamplerTests-RelWithDebInfo-4b633d0d2f73e92fdeb7.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0859
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8FastNeutralizeTests-Debug-7d63f77ea1b710b41e51.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0860
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8FastNeutralizeTests-MinSizeRel-d49749096b2ea2249164.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0861
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8FastNeutralizeTests-Release-548906eb2739c0bf6125.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0862
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8FastNeutralizeTests-RelWithDebInfo-a239971e85518069eaa5.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0863
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8RawSourceProbeTests-Debug-28375eae27962e0e2393.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0864
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8RawSourceProbeTests-MinSizeRel-8f2d9b3b8577cc5eee31.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0865
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8RawSourceProbeTests-Release-96a77cb3645690bd3c79.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0866
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8RawSourceProbeTests-RelWithDebInfo-53e088af8b3b9d7e93d8.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0867
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8SourceNeutralizerTests-Debug-eeedbc37d07e56f75e24.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0868
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8SourceNeutralizerTests-MinSizeRel-dd69aa9164857a28b84f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0869
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8SourceNeutralizerTests-Release-96c52de14816fac3f76f.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0870
- 文件/目录范围：`build/.cmake/api/v1/reply/target-V8SourceNeutralizerTests-RelWithDebInfo-713c51a89cba3690331c.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0871
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ZERO_CHECK-Debug-986d3eb3933098e6badc.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0872
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ZERO_CHECK-MinSizeRel-986d3eb3933098e6badc.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0873
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ZERO_CHECK-Release-986d3eb3933098e6badc.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0874
- 文件/目录范围：`build/.cmake/api/v1/reply/target-ZERO_CHECK-RelWithDebInfo-986d3eb3933098e6badc.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0875
- 文件/目录范围：`build/.cmake/api/v1/reply/toolchains-v1-95edfdd826f2c3647de2.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0876
- 文件/目录范围：`build/ALL_BUILD.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0877
- 文件/目录范围：`build/ALL_BUILD.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0878
- 文件/目录范围：`build/CefHook.sln`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0879
- 文件/目录范围：`build/cmake_install.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0880
- 文件/目录范围：`build/CMakeCache.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0881
- 文件/目录范围：`build/CMakeFiles/1c7bb12b6d416d4773566c17420a44f4/generate.stamp.rule`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0882
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeCCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0883
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeCXXCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0884
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeDetermineCompilerABI_C.bin`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0885
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeDetermineCompilerABI_CXX.bin`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0886
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeRCCompiler.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0887
- 文件/目录范围：`build/CMakeFiles/4.3.0/CMakeSystem.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-0888
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/CMakeCCompilerId.c`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0889
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/CompilerIdC.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0890
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/CompilerIdC.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0891
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CMakeCCompilerId.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0892
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0893
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0894
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0895
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0896
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0897
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/CompilerIdC.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0898
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0899
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0900
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0901
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/CompilerIdC.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0902
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdC/Debug/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0903
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/CMakeCXXCompilerId.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0904
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/CompilerIdCXX.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0905
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/CompilerIdCXX.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0906
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CMakeCXXCompilerId.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0907
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0908
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0909
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0910
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0911
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0912
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/CompilerIdCXX.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0913
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0914
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0915
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0916
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/CompilerIdCXX.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0917
- 文件/目录范围：`build/CMakeFiles/4.3.0/CompilerIdCXX/Debug/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0918
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0919
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0920
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0921
- 文件/目录范围：`build/CMakeFiles/4.3.0/VCTargetsPath/Debug/VCTargetsPath.tlog/VCTargetsPath.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0922
- 文件/目录范围：`build/CMakeFiles/cmake.check_cache`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0923
- 文件/目录范围：`build/CMakeFiles/CMakeConfigureLog.yaml`
- 检查目标：检查文档/API/ADR 是否覆盖实际入口、配置、模块边界和迁移约束
- 潜在问题：文档可能编码异常、过期或遗漏真实路由/环境变量
- 修改策略：在审计文档中标记差距并提出修订项
- 验证方式：rg 代码入口与文档条目交叉验证
- 完成状态：已完成

## S01-0924
- 文件/目录范围：`build/CMakeFiles/e7e6f86041512cc3686f61202aeab8e9/qrc_ui_resources.cpp.rule`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0925
- 文件/目录范围：`build/CMakeFiles/generate.stamp`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0926
- 文件/目录范围：`build/CMakeFiles/generate.stamp.depend`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0927
- 文件/目录范围：`build/CMakeFiles/generate.stamp.list`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0928
- 文件/目录范围：`build/CMakeFiles/InstallScripts.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-0929
- 文件/目录范围：`build/CMakeFiles/TargetDirectories.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0930
- 文件/目录范围：`build/src/CefHook_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0931
- 文件/目录范围：`build/src/CefHook.dir/Release/auto_inject.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0932
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_console_fallback.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0933
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_dispatcher.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0934
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_json.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0935
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_process_message.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0936
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_runtime.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0937
- 文件/目录范围：`build/src/CefHook.dir/Release/bridge_v8.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0938
- 文件/目录范围：`build/src/CefHook.dir/Release/buffer.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0939
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.dll.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0940
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.iobj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0941
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CefHook.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0942
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0943
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0944
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0945
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0946
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0947
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0948
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0949
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0950
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0951
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0952
- 文件/目录范围：`build/src/CefHook.dir/Release/CefHook.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0953
- 文件/目录范围：`build/src/CefHook.dir/Release/diagnostics_capability.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0954
- 文件/目录范围：`build/src/CefHook.dir/Release/dllmain.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0955
- 文件/目录范围：`build/src/CefHook.dir/Release/exam_capability.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0956
- 文件/目录范围：`build/src/CefHook.dir/Release/hde32.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0957
- 文件/目录范围：`build/src/CefHook.dir/Release/hook_engine.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0958
- 文件/目录范围：`build/src/CefHook.dir/Release/hook.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0959
- 文件/目录范围：`build/src/CefHook.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0960
- 文件/目录范围：`build/src/CefHook.dir/Release/page_capability.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0961
- 文件/目录范围：`build/src/CefHook.dir/Release/pattern_scan.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0962
- 文件/目录范围：`build/src/CefHook.dir/Release/pch.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0963
- 文件/目录范围：`build/src/CefHook.dir/Release/process_identity.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0964
- 文件/目录范围：`build/src/CefHook.dir/Release/runtime_capability.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0965
- 文件/目录范围：`build/src/CefHook.dir/Release/script_capability.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0966
- 文件/目录范围：`build/src/CefHook.dir/Release/thread.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0967
- 文件/目录范围：`build/src/CefHook.dir/Release/trampoline.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0968
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_compile_sampler.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0969
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_fast_neutralize.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0970
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_hooks.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0971
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_intercept.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0972
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_raw_source_probe.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0973
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_signatures.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0974
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_source_neutralizer.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0975
- 文件/目录范围：`build/src/CefHook.dir/Release/v8_source_probe.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0976
- 文件/目录范围：`build/src/CefHook.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0977
- 文件/目录范围：`build/src/CefHook.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0978
- 文件/目录范围：`build/src/CefHook.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0979
- 文件/目录范围：`build/src/ClientTests_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-0980
- 文件/目录范围：`build/src/ClientTests.dir/Release/api_client.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0981
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0982
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0983
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0984
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0985
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0986
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/ClientTests.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0987
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0988
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0989
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0990
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0991
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0992
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0993
- 文件/目录范围：`build/src/ClientTests.dir/Release/ClientTests.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0994
- 文件/目录范围：`build/src/ClientTests.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0995
- 文件/目录范围：`build/src/ClientTests.dir/Release/test_client.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0996
- 文件/目录范围：`build/src/ClientTests.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-0997
- 文件/目录范围：`build/src/ClientTests.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0998
- 文件/目录范围：`build/src/ClientTests.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-0999
- 文件/目录范围：`build/src/cmake_install.cmake`
- 检查目标：检查 CMake 构建入口、目标文件列表、依赖路径和安全编译选项
- 潜在问题：手工维护源文件列表可能漏文件或引用生成产物
- 修改策略：记录构建风险并交接客户端构建会话
- 验证方式：cmake -S client -B build 配置验证
- 完成状态：已完成

## S01-1000
- 文件/目录范围：`build/src/CMakeFiles/CefHook_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1001
- 文件/目录范围：`build/src/CMakeFiles/CefHook_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1002
- 文件/目录范围：`build/src/CMakeFiles/CefHook_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1003
- 文件/目录范围：`build/src/CMakeFiles/ClientTests_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1004
- 文件/目录范围：`build/src/CMakeFiles/ClientTests_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1005
- 文件/目录范围：`build/src/CMakeFiles/ClientTests_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1006
- 文件/目录范围：`build/src/CMakeFiles/generate.stamp`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1007
- 文件/目录范围：`build/src/CMakeFiles/generate.stamp.depend`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1008
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1009
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1010
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Info.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1011
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Lock.lock`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1012
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/AutoRcc_ui_resources_UYX5XTB5RZ_Used_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1013
- 文件/目录范围：`build/src/CMakeFiles/Injector_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1014
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjector_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1015
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjector_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1016
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjector_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1017
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1018
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1019
- 文件/目录范围：`build/src/CMakeFiles/ManualDllInjectorGui_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1020
- 文件/目录范围：`build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1021
- 文件/目录范围：`build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1022
- 文件/目录范围：`build/src/CMakeFiles/V8CompileSamplerTests_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1023
- 文件/目录范围：`build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1024
- 文件/目录范围：`build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1025
- 文件/目录范围：`build/src/CMakeFiles/V8FastNeutralizeTests_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1026
- 文件/目录范围：`build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1027
- 文件/目录范围：`build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1028
- 文件/目录范围：`build/src/CMakeFiles/V8RawSourceProbeTests_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1029
- 文件/目录范围：`build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir/AutogenInfo.json`
- 检查目标：检查 JSON/锁文件/配置的数据来源、是否为包管理或运行数据
- 潜在问题：配置与运行数据混放会导致迁移或发布误用
- 修改策略：分类为源码配置或 runtime data，并写入风险
- 验证方式：读取文件头/路径并与 package/build 脚本对照
- 完成状态：已完成

## S01-1030
- 文件/目录范围：`build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir/AutogenUsed_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1031
- 文件/目录范围：`build/src/CMakeFiles/V8SourceNeutralizerTests_autogen.dir/ParseCache_Release.txt`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1032
- 文件/目录范围：`build/src/Injector_autogen/include_Release/5SSUPNRDSB/moc_workers.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1033
- 文件/目录范围：`build/src/Injector_autogen/include_Release/5SSUPNRDSB/moc_workers.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1034
- 文件/目录范围：`build/src/Injector_autogen/include_Release/7CL2LH2DRO/moc_session_controller.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1035
- 文件/目录范围：`build/src/Injector_autogen/include_Release/7CL2LH2DRO/moc_session_controller.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1036
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_animated_button.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1037
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_animated_button.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1038
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_inline_status.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1039
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_inline_status.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1040
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_main_window.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1041
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_main_window.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1042
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_title_bar.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1043
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/moc_title_bar.cpp.d`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1044
- 文件/目录范围：`build/src/Injector_autogen/include_Release/UYX5XTB5RZ/qrc_ui_resources_CMAKE_.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1045
- 文件/目录范围：`build/src/Injector_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1046
- 文件/目录范围：`build/src/Injector_autogen/UYX5XTB5RZ/qrc_ui_resources.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1047
- 文件/目录范围：`build/src/Injector.dir/Release/api_client.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1048
- 文件/目录范围：`build/src/Injector.dir/Release/CX_LingQiao.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1049
- 文件/目录范围：`build/src/Injector.dir/Release/CX_LingQiao.iobj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1050
- 文件/目录范围：`build/src/Injector.dir/Release/injector_qt.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1051
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1052
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1053
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1054
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1055
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1056
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1057
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1058
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/Injector.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1059
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1060
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1061
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1062
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1063
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/rc.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1064
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/rc.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1065
- 文件/目录范围：`build/src/Injector.dir/Release/Injector.tlog/rc.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1066
- 文件/目录范围：`build/src/Injector.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1067
- 文件/目录范围：`build/src/Injector.dir/Release/qrc_ui_resources.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1068
- 文件/目录范围：`build/src/Injector.dir/Release/session_controller.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1069
- 文件/目录范围：`build/src/Injector.dir/Release/update_controller.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1070
- 文件/目录范围：`build/src/Injector.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-1071
- 文件/目录范围：`build/src/Injector.dir/Release/version.res`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1072
- 文件/目录范围：`build/src/Injector.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1073
- 文件/目录范围：`build/src/Injector.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1074
- 文件/目录范围：`build/src/ManualDllInjector_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1075
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/manual_dll_injector.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1076
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/manual_injector_core.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1077
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1078
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1079
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1080
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1081
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1082
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1083
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1084
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1085
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1086
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1087
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1088
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDl.CFC374BF.tlog/ManualDllInjector.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1089
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/ManualDllInjector.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1090
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1091
- 文件/目录范围：`build/src/ManualDllInjector.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-1092
- 文件/目录范围：`build/src/ManualDllInjector.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1093
- 文件/目录范围：`build/src/ManualDllInjector.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1094
- 文件/目录范围：`build/src/ManualDllInjectorGui_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1095
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/manual_dll_injector_gui.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1096
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/manual_injector_core.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1097
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1098
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1099
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1100
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1101
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1102
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1103
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1104
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1105
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1106
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1107
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1108
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDl.737E356E.tlog/ManualDllInjectorGui.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1109
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/ManualDllInjectorGui.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1110
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1111
- 文件/目录范围：`build/src/ManualDllInjectorGui.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-1112
- 文件/目录范围：`build/src/ManualDllInjectorGui.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1113
- 文件/目录范围：`build/src/ManualDllInjectorGui.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1114
- 文件/目录范围：`build/src/Release/CefHook.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1115
- 文件/目录范围：`build/src/Release/CefHook.exp`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1116
- 文件/目录范围：`build/src/Release/CefHook.lib`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1117
- 文件/目录范围：`build/src/Release/ClientTests.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1118
- 文件/目录范围：`build/src/Release/CX_LingQiao.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1119
- 文件/目录范围：`build/src/Release/D3Dcompiler_47.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1120
- 文件/目录范围：`build/src/Release/iconengines/qsvgicon.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1121
- 文件/目录范围：`build/src/Release/imageformats/qgif.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1122
- 文件/目录范围：`build/src/Release/imageformats/qicns.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1123
- 文件/目录范围：`build/src/Release/imageformats/qico.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1124
- 文件/目录范围：`build/src/Release/imageformats/qjpeg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1125
- 文件/目录范围：`build/src/Release/imageformats/qsvg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1126
- 文件/目录范围：`build/src/Release/imageformats/qtga.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1127
- 文件/目录范围：`build/src/Release/imageformats/qtiff.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1128
- 文件/目录范围：`build/src/Release/imageformats/qwbmp.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1129
- 文件/目录范围：`build/src/Release/imageformats/qwebp.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1130
- 文件/目录范围：`build/src/Release/libEGL.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1131
- 文件/目录范围：`build/src/Release/libGLESv2.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1132
- 文件/目录范围：`build/src/Release/ManualDllInjector.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1133
- 文件/目录范围：`build/src/Release/ManualDllInjectorGui.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1134
- 文件/目录范围：`build/src/Release/opengl32sw.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1135
- 文件/目录范围：`build/src/Release/platforms/qwindows.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1136
- 文件/目录范围：`build/src/Release/Qt5Core.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1137
- 文件/目录范围：`build/src/Release/Qt5Gui.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1138
- 文件/目录范围：`build/src/Release/Qt5Svg.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1139
- 文件/目录范围：`build/src/Release/Qt5Widgets.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1140
- 文件/目录范围：`build/src/Release/styles/qwindowsvistastyle.dll`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1141
- 文件/目录范围：`build/src/Release/V8CompileSamplerTests.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1142
- 文件/目录范围：`build/src/Release/V8FastNeutralizeTests.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1143
- 文件/目录范围：`build/src/Release/V8RawSourceProbeTests.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1144
- 文件/目录范围：`build/src/Release/V8SourceNeutralizerTests.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1145
- 文件/目录范围：`build/src/Release/vc_redist.x86.exe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1146
- 文件/目录范围：`build/src/V8CompileSamplerTests_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1147
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1148
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/test_v8_compile_sampler.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1149
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/v8_compile_sampler.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1150
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1151
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1152
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1153
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1154
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1155
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1156
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1157
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1158
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1159
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1160
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1161
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8Compil.DE24B7FB.tlog/V8CompileSamplerTests.lastbuildstate`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1162
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/V8CompileSamplerTests.exe.recipe`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1163
- 文件/目录范围：`build/src/V8CompileSamplerTests.dir/Release/vcpkg.applocal.log`
- 检查目标：检查运行数据、密钥、证书或日志是否进入源码目录和 git 状态
- 潜在问题：敏感材料或环境数据可能泄露并污染迁移验证
- 修改策略：列入高风险；建议忽略和运行目录隔离，不直接删除
- 验证方式：git status --short 与安全文档对照
- 完成状态：已完成

## S01-1164
- 文件/目录范围：`build/src/V8CompileSamplerTests.vcxproj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1165
- 文件/目录范围：`build/src/V8CompileSamplerTests.vcxproj.filters`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1166
- 文件/目录范围：`build/src/V8FastNeutralizeTests_autogen/mocs_compilation_Release.cpp`
- 检查目标：检查 C/C++ 文件的构建目标归属、include 边界和安全编译选项覆盖
- 潜在问题：文件可能未注册到 CMake 或跨模块 include 破坏分层
- 修改策略：交接客户端会话在 CMake/目标级修复
- 验证方式：cmake configure/build 对应目标
- 完成状态：已完成

## S01-1167
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/mocs_compilation_Release.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1168
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/test_v8_fast_neutralize.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1169
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/v8_fast_neutralize.obj`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1170
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CL.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1171
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/Cl.items.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1172
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CL.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1173
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CL.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1174
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CustomBuild.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1175
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CustomBuild.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1176
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/CustomBuild.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1177
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/link.command.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1178
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/link.read.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1179
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/link.secondary.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1180
- 文件/目录范围：`build/src/V8FastNeutralizeTests.dir/Release/V8FastNe.5BCDF020.tlog/link.write.1.tlog`
- 检查目标：检查文件在构建链路、运行入口、部署或文档中的具体归属
- 潜在问题：未知或混合用途文件可能导致重复模块、废弃模块或错误发布
- 修改策略：记录用途；必要时交接对应会话确认
- 验证方式：rg 文件名引用与目录归属对照
- 完成状态：已完成

## S01-1181
- 文件/目录范围：`跨模块调用链: client/src/services/api_client.cpp -> server/internal/auth/auth.go -> server/handler.go`
- 检查目标：验证客户端 HMAC 签名和服务端验签语义一致
- 潜在问题：编码、时间戳、nonce、路径/body 签名选择不一致会导致迁移后客户端不可用
- 修改策略：建立 Go/C++/Java golden vectors
- 验证方式：C++ 回归脚本 + Go auth tests
- 完成状态：已完成

## S01-1182
- 文件/目录范围：`跨模块调用链: client/src/controllers/session_controller.cpp -> server/internal/features/sessions`
- 检查目标：验证激活、心跳、退出、恢复的字段和错误语义
- 潜在问题：Java 迁移若改变响应字段或状态码会破坏客户端状态机
- 修改策略：将 sessions 作为第一批迁移域并保持兼容端点
- 验证方式：sessions Go tests + 客户端会话回归
- 完成状态：已完成

## S01-1183
- 文件/目录范围：`跨模块调用链: client/src/controllers/update_controller.cpp -> server/internal/features/releases`
- 检查目标：验证 release manifest、签名、公钥、包下载路径
- 潜在问题：签名算法或路径变更会导致更新失败
- 修改策略：保留 /api/v1/update/check 与 package 路由兼容
- 验证方式：release handler tests + 客户端更新编译
- 完成状态：已完成

## S01-1184
- 文件/目录范围：`跨模块调用链: client/src/hook/script_loader.h -> server/internal/features/scripts`
- 检查目标：验证脚本下载、激活和事实源路径
- 潜在问题：旧 /api/v1/script 与新 admin scripts API 语义可能分裂
- 修改策略：定义脚本 API 合约并交接 scripts 会话
- 验证方式：script_module_test + client runtime binding regression
- 完成状态：已完成

## S01-1185
- 文件/目录范围：`跨模块调用链: payloads key exchange -> client DLL/runtime`
- 检查目标：验证 AES/HMAC/PBKDF2/分片下载与 DLL 加密链路
- 潜在问题：跨语言加密实现任何差异都会导致 payload 或 DLL 不可用
- 修改策略：迁移后期处理并要求黄金向量
- 验证方式：payload Go tests + 手工 crypto vector
- 完成状态：已完成

## S01-1186
- 文件/目录范围：`部署链路: server/web npm build -> server/web-dist -> Go embed -> Java static resources`
- 检查目标：验证前端静态资源输出和服务路径
- 潜在问题：Go embed 迁移到 Java resources/static 时路径前缀可能改变
- 修改策略：制定静态资源迁移阶段
- 验证方式：npm run build + go build/go test
- 完成状态：已完成

## S01-1187
- 文件/目录范围：`部署链路: client CMake -> installer WiX -> scripts/release.ps1`
- 检查目标：验证客户端产物名、版本、签名和安装包输入
- 潜在问题：多个 build 目录和版本源可能导致发布旧二进制
- 修改策略：写入发布链路风险并交接 release 会话
- 验证方式：CMake configure/build + release dry-run
- 完成状态：已完成

## S01-1188
- 文件/目录范围：`数据链路: data/lingqiao.db -> server/db.go -> repository_sqlite.go`
- 检查目标：验证 SQLite schema、迁移和 Java JDBC 映射
- 潜在问题：schema 未版本化会使 Java 迁移难以回滚
- 修改策略：引入迁移工具计划，不直接改 schema
- 验证方式：go test db and repositories
- 完成状态：已完成

## S01-1189
- 文件/目录范围：`鉴权链路: admin login -> adminAuth -> RequirePermission -> feature handlers`
- 检查目标：验证权限名、路由保护和 cookie/session 策略
- 潜在问题：权限缺失或错误复用会扩大管理面暴露面
- 修改策略：写入安全风险和 Java Spring Security 映射
- 验证方式：rg RequirePermission/adminAuth + handler tests
- 完成状态：已完成

## S01-1190
- 文件/目录范围：`鉴权链路: agent login -> agent server -> agents services`
- 检查目标：验证代理端独立端口、CORS 和账号权限边界
- 潜在问题：agent 与 admin 共享资源但权限模型不同，迁移易串线
- 修改策略：独立 security filter chain 设计
- 验证方式：agent handler tests + route audit
- 完成状态：已完成

## S01-1191
- 文件/目录范围：`运行入口: server main CLI --generate-card`
- 检查目标：验证 CLI 卡密生成模式仍可在 Java 迁移中替代
- 潜在问题：运维脚本可能依赖 go run . --generate-card
- 修改策略：迁移为 Spring Shell/CLI 子命令或独立 admin API
- 验证方式：go run . --generate-card dry-run in test env
- 完成状态：已完成

## S01-1192
- 文件/目录范围：`任务调度入口: APIHandler.SessionCleanupTask`
- 检查目标：验证 session 过期清理周期和上下文取消
- 潜在问题：Java 迁移 Scheduler 线程池和事务边界需保持语义
- 修改策略：迁移为 @Scheduled 并保留测试
- 验证方式：sessions cleanup tests
- 完成状态：已完成

## S01-1193
- 文件/目录范围：`环境变量矩阵: docs/operations/config.md vs os.Getenv/envOr`
- 检查目标：对齐所有环境变量文档和代码读取项
- 潜在问题：遗漏 UPLOAD_KEY/SIGNING_CERT/CERT_HOST 等变量会导致部署失败
- 修改策略：生成配置矩阵并交接文档会话
- 验证方式：rg os.Getenv/envOr + docs 对照
- 完成状态：已完成

## S01-1194
- 文件/目录范围：`目录命名边界: server/internal/features/admin vs server/admin*.go`
- 检查目标：确认 admin 重复模块是否为迁移中遗留
- 潜在问题：重复 handlers 可能出现两个事实源
- 修改策略：写入重复模块风险，不直接删除
- 验证方式：rg admin handlers and route owners
- 完成状态：已完成

## S01-1195
- 文件/目录范围：`目录命名边界: AutoExam.js vs packages/autoexam/src/autoexam.js`
- 检查目标：确认 AutoExam 脚本事实源
- 潜在问题：根文件和 package 文件可能内容漂移
- 修改策略：交接脚本/考试会话做单一事实源收敛
- 验证方式：hash/rg 对照
- 完成状态：已完成

## S01-1196
- 文件/目录范围：`废弃模块候选: build-status-review`
- 检查目标：判断是否为临时构建审查产物
- 潜在问题：构建产物目录可能被误提交或发布脚本误用
- 修改策略：列入清理 handoff
- 验证方式：rg build-status-review
- 完成状态：已完成

## S01-1197
- 文件/目录范围：`废弃模块候选: server/..distserverlingqiao-server.exe`
- 检查目标：判断异常命名二进制是否为误生成文件
- 潜在问题：异常文件名可能影响发布/安全扫描
- 修改策略：列入清理 handoff，不删除
- 验证方式：git status and file existence
- 完成状态：已完成

## S01-1198
- 文件/目录范围：`语义范围/installer/迁移检查-1198`
- 检查目标：针对 installer 的 Java 迁移兼容性、入口归属、配置依赖和验证命令进行专项复核
- 潜在问题：installer 相关逻辑若存在隐式跨模块依赖，迁移或并行重构时可能出现行为漂移
- 修改策略：将发现写入对应全局报告；需要业务代码调整时转交并行会话，本会话只处理全局配置/文档/构建入口
- 验证方式：rg 'installer' server client docs scripts 并执行对应 Go/web/CMake 验证
- 完成状态：已完成

## S01-1199
- 文件/目录范围：`语义范围/docs/迁移检查-1199`
- 检查目标：针对 docs 的 Java 迁移兼容性、入口归属、配置依赖和验证命令进行专项复核
- 潜在问题：docs 相关逻辑若存在隐式跨模块依赖，迁移或并行重构时可能出现行为漂移
- 修改策略：将发现写入对应全局报告；需要业务代码调整时转交并行会话，本会话只处理全局配置/文档/构建入口
- 验证方式：rg 'docs' server client docs scripts 并执行对应 Go/web/CMake 验证
- 完成状态：已完成

## S01-1200
- 文件/目录范围：`语义范围/cards/迁移检查-1200`
- 检查目标：针对 cards 的 Java 迁移兼容性、入口归属、配置依赖和验证命令进行专项复核
- 潜在问题：cards 相关逻辑若存在隐式跨模块依赖，迁移或并行重构时可能出现行为漂移
- 修改策略：将发现写入对应全局报告；需要业务代码调整时转交并行会话，本会话只处理全局配置/文档/构建入口
- 验证方式：rg 'cards' server client docs scripts 并执行对应 Go/web/CMake 验证
- 完成状态：已完成

