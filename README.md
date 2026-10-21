# 灵桥平台 (Lingqiao Platform)

灵桥平台包含 Windows C++ 客户端、CEF/V8 Hook Runtime、Go 生产后端、Vite 管理/代理前端、AutoExam 脚本包、安装发布工具和一个默认关闭的 Java 迁移侧车。当前生产入口仍是 `server/main.go`；`server-java/` 只用于渐进兼容验证，不替换 Go 路由或生产端口。

## 仓库结构

| 路径 | 归属 |
| --- | --- |
| `client/` | Qt/C++ 客户端、注入器、Hook DLL 和客户端测试 |
| `server/` | Go 模块化单体、SQLite/文件存储和嵌入式 Web 资源 |
| `server/web/` | Vite + TypeScript 管理端、代理端和共享前端代码 |
| `server-java/` | Spring Boot 侧车与 Go 兼容契约测试，默认不接管流量 |
| `packages/autoexam/` | AutoExam 单一源码、构建和回归入口 |
| `e2e_test/` | 跨进程 Go E2E 测试 |
| `installer/` | WiX 安装包和签名入口 |
| `scripts/` | 质量门禁、发布和回归脚本 |
| `docs/` | 架构、ADR、API、运维和安全文档 |
| `cef/`, `chromium/` | 客户端原生依赖 |

`build*`、`dist/`、`server/web-dist/`、`data/`、`server/data/`、`certs/`、日志和本地工具状态不属于源码边界。完整归属和保护规则见 `docs/architecture/repository-boundaries.md`。

## Go 后端

要求 Go 版本以 `server/go.mod` 为准，当前为 Go 1.26.3。前端产物由 Go `embed` 打包，因此运行或测试服务端前先构建 Web：

```powershell
cd server/web
npm ci
npm run check
```

```powershell
cd server
$env:GOCACHE="$PWD\.gocache"
go test ./...
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

管理端/客户端 API 默认监听 `48901`，代理端默认监听 `38472`。完整环境变量和运行时文件矩阵见 `docs/operations/config.md`。

## C++ 客户端

要求 Visual Studio 2022 C++ 工具链、CMake 和 Qt 5.15。根构建脚本默认生成 Win32 Release，可通过环境变量覆盖构建目录、Qt 路径、生成器和平台：

```powershell
$env:BUILD_DIR='build'
$env:QT_PREFIX_PATH='C:\Qt\5.15.2\msvc2019'
$env:CMAKE_PLATFORM='Win32'
.\build.bat
```

等价的手动入口：

```powershell
cmake -S client -B build -G "Visual Studio 17 2022" -A Win32 -DCMAKE_PREFIX_PATH="C:\Qt\5.15.2\msvc2019"
cmake --build build --config Release --target CefHook Injector
```

## Java 侧车

Java 迁移必须保持 sidecar-first、默认只读和可回滚：

```powershell
cd server-java
mvn test
```

Java 不得默认绑定 Go 的 `48901`/`38472` 端口，不得默认写入现有 SQLite、JSON、密钥或发布文件。迁移顺序和兼容门禁见 `docs/project-management/JAVA_BACKEND_ARCHITECTURE.md` 与 `docs/architecture/server-module-map.md`。

## AutoExam

`packages/autoexam/src/autoexam.js` 是脚本源码入口。根 `AutoExam.js` 属于兼容/发布副本，任何同步规则都必须通过构建或回归脚本验证，不能手工形成新的脚本事实源。

```powershell
powershell -ExecutionPolicy Bypass -File packages/autoexam/build.ps1
powershell -ExecutionPolicy Bypass -File scripts/autoexam_multimodal_provider_regression_test.ps1
```

## 统一验证

```powershell
powershell -ExecutionPolicy Bypass -File scripts/check.ps1
```

该脚本覆盖 Web 检查、Go 测试、E2E、Go 漏洞扫描和 CMake 配置。C++ Release 目标的实际编译仍需运行 `build.bat` 或对应 `cmake --build` 命令。
