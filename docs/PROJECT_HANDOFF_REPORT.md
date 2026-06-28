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
