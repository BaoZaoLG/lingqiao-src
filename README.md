# 灵桥平台 (Lingqiao Platform)

灵桥平台是一个结合了 C++ Windows 客户端与 Go 后台管理服务器的综合系统。客户端主要通过挂钩 CEF (Chromium Embedded Framework) / V8 引擎提供高级诊断和安全性分析能力，Go 服务端则负责设备/卡片管理、在线更新、聊天和管理控制台。

## 目录结构 (Directory Layout)

- **`src/`**：客户端 C++ 源代码（基于 Qt 构建的 UI 界面、CEF Hook 引擎、注入器模块等）。
- **`injector-server/`**：Go 后端服务器源码，包含基于 Vite + TypeScript 构建的前端管理面板（`web/`）。
- **`cef/` & `chromium/`**：客户端依赖的 CEF 和 Chromium 原生头文件与库。
- **`installer/`**：客户端的 WiX Toolset 安装包打包脚本。
- **`scripts/`**：构建与辅助工具脚本（包含用于生成加密 C++ 头部的脚本）。
- **`AutoExam_silent.js`**：客户端注入的核心自动考试 JS 脚本。

---

## 快速上手 (Quick Start)

### 1. Go 后端开发 (Server Development)

#### 环境准备
在 `injector-server` 目录下配置缓存并安装前端依赖：
```powershell
cd injector-server
# 配置本地 Go 编译缓存（可选）
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\injector-server\.gocache'

# 编译管理前端
cd web
npm install
npm run build
cd ..
```

#### 运行与测试
```powershell
# 运行单元测试
go test ./...

# 运行服务器 (默认端口 48901)
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

### 2. C++ 客户端开发 (Client Development)

#### 编译环境
- **编译器**：MSBuild / Visual Studio 2022 (v143)
- **库依赖**：Qt 5.15+ / CMake 3.20+
- **构建命令**：
  ```powershell
  # 创建并进入构建目录
  mkdir build
  cd build
  # 使用 CMake 生成 VS 解决方案
  cmake .. -DCMAKE_BUILD_TYPE=Release
  # 执行编译
  cmake --build . --config Release
  ```
