# 灵桥平台 (Lingqiao) 发布与构建指南

本文档规定了 Lingqiao 平台各模块的构建方式以及自动化打包发布的标准流程。整个系统由三部分构成：Web 前端、Go 服务端、C++ 客户端及 Hook DLL。

## 构建脚本体系

我们提供不同层级的构建脚本以满足不同场景需求：

1. **`build.bat` (开发期 C++ 快速构建)**
   - **位置**：根目录
   - **作用**：仅针对 C++ 客户端与 Hook DLL 进行 CMake 增量配置与构建。
   - **适用场景**：日常 C++ 调试与开发。不会打包服务端，也不会生成安装包。

2. **`scripts/release.ps1` (自动化全链路打包)**
   - **位置**：`scripts/release.ps1`
   - **作用**：一键执行 Web 编译、Go 编译、C++ 编译以及 WiX 安装包制作。
   - **适用场景**：需要生成用于对外分发的整体 Release 产物时。
   - **参数支持**：
     - `-SkipWeb`: 跳过 `npm run build`
     - `-SkipGo`: 跳过 Go 后端编译
     - `-SkipCpp`: 跳过 C++ 客户端编译
     - `-SkipWix`: 跳过 MSI/Bundle 安装包打包

3. **`installer/build_installer.ps1` (底层安装包脚本)**
   - **位置**：`installer/build_installer.ps1`
   - **作用**：读取已有的编译产物（主要是 `CX_LingQiao.exe` 等），执行 WiX Toolset 构建，生成 MSI 和 Bootstrapper Bundle。通常不需要手动调用，它会被 `release.ps1` 内部调用。

---

## 自动化发版流程 (Release Pipeline)

当准备好发布一个新版本时，请按以下步骤操作：

### 1. 更新版本号
在发布之前，必须确保代码库中的版本号一致：
1. 修改 `client/CMakeLists.txt` 中的 `set(APP_VERSION "x.x.x")`
2. 修改 Web 前端（可选）
3. 确保所有改动已提交并打上 Git Tag（如 `v2.x.x`）

### 2. 执行一键构建
在项目 **根目录** 打开 PowerShell，执行：
```powershell
.\scripts\release.ps1
```

> **注意：** 此脚本要求机器上已配置好 Node.js、Go、CMake、Visual Studio 2022 (MSVC)、以及 WiX Toolset v4/v5。

### 3. 获取发布产物
流水线执行成功后，所有可供发布的产物会输出到 `dist/` 目录下：
- 服务端独立程序：`dist/server/lingqiao-server.exe`
- 客户端安装包：`dist/installer/LingqiaoSetup-x.x.x.exe`
- 客户端 MSI 文件：`dist/installer/Lingqiao-x.x.x.msi`
- 发版元数据与校验和：`dist/installer/release-info.json`

### 4. 系统内建发布与更新 (灰度与全量)
拿到产物后，通过管理后台进行发布：
1. 以超级管理员身份登录。
2. 前往 **发布管理**，创建新的 Release 记录，填写发版说明与版本号。
3. 上传刚生成的 `LingqiaoSetup-x.x.x.exe`。
4. 可以先配置 `Rollout Percent` 为较小数值进行灰度发布，观察日志后再逐步调到 100%。

---

## 本地代码检查清单 (Preflight Checks)

在提交重大版本前，请务必执行以下验证确保代码健康度：

```powershell
# 1. 验证 Go 后端及数据库重构
cd server
go test ./...

# 2. 验证前端类型与质量
cd web
npm run check
```
