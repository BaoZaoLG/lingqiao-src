# CX_LingQiao v3.0 全面重构设计文档

> 日期: 2026-05-29
> 版本: v2.1.13 → v3.0.0
> 策略: 方案 B — 全面重构
> 约束: 专业团队、小范围用户、C++/Qt 客户端

---

## 目录

1. [总体架构](#1-总体架构)
2. [服务端重构](#2-服务端重构)
3. [客户端重构](#3-客户端重构)
4. [AI 网关](#4-ai-网关)
5. [平台适配器系统](#5-平台适配器系统)
6. [插件系统](#6-插件系统)
7. [安全加固](#7-安全加固)
8. [工程基础设施](#8-工程基础设施)
9. [数据迁移策略](#9-数据迁移策略)
10. [里程碑计划](#10-里程碑计划)

---

## 1. 总体架构

### 1.1 现状

```
CX_LingQiao v2.1.13:
├── 客户端 (C++17 + Qt5 + MinHook, Windows-only, Win32 API)
│   ├── Injector EXE — 注入器 GUI
│   └── CefHook.dll — CEF hook + JS 注入
├── JS 脚本 (AutoExam_silent.js, 编译时嵌入 DLL, 直调 DeepSeek API)
└── 服务端 (Go 1.21, JSON 文件存储, 单进程双端口)
```

### 1.2 目标

```
CX_LingQiao v3.0:
├── 客户端 (C++17 + Qt6 + MinHook, 平台抽象层)
│   ├── Injector EXE — 模块化 CMake, Qt6 GUI
│   ├── CefHook.dll — 插件化宿主
│   └── plugins/ — 可选插件 (OCR, 截图等)
├── JS 引擎 (从服务端热加载, 适配器系统, 多平台支持)
│   ├── 核心层 (ai-client, answer-filler, cache)
│   ├── 适配器层 (chaoxing, zhihuishu, yuke, generic)
│   └── 适配器注册表
└── 服务端 (Go 1.22+, PostgreSQL, 模块化单体)
    ├── 核心服务 (card, session, auth)
    ├── AI 网关 (多模型路由, 缓存, 评估)
    ├── 管理后台 (SPA)
    └── 插件/脚本分发
```

### 1.3 核心原则

- **不追求企业级复杂度** — 无需 Kubernetes、多租户、高可用集群
- **追求工程质量和可扩展性** — 模块化、插件化、可测试
- **渐进式交付** — 每个子系统独立可交付、可回滚

---

## 2. 服务端重构

### 2.1 现状问题

- `storage.go` 用 JSON 文件持久化，无事务、无索引、无并发安全
- 业务逻辑耦合在 `card.go`、`handler.go`、`admin.go`、`agent.go`
- 认证、限流、审计混在业务代码中
- 自签名 TLS 硬编码

### 2.2 目标结构

```
server/
├── cmd/
│   └── server/
│       └── main.go                  # 启动入口，依赖注入组装
├── internal/
│   ├── domain/                      # 领域模型（纯 Go struct + 业务规则）
│   │   ├── card.go                  # Card 实体 + 规则
│   │   ├── session.go               # Session 实体 + 规则
│   │   ├── user.go                  # Admin/Agent 用户实体
│   │   └── machine.go               # Machine 实体
│   ├── repository/                  # 数据访问层（接口 + 实现）
│   │   ├── interfaces.go            # CardRepo, SessionRepo, UserRepo 等接口
│   │   ├── postgres/                # PostgreSQL 实现
│   │   │   ├── card_repo.go
│   │   │   ├── session_repo.go
│   │   │   └── migrations/          # SQL 迁移脚本
│   │   └── memory/                  # 内存实现（测试/开发用）
│   ├── service/                     # 业务服务层
│   │   ├── card_service.go          # 卡密生成、激活、续期
│   │   ├── session_service.go       # 会话管理、心跳
│   │   ├── auth_service.go          # 认证、鉴权
│   │   ├── ai_gateway.go            # AI 模型路由、调用、计费
│   │   └── ota_service.go           # OTA 更新管理
│   ├── handler/                     # HTTP 处理器
│   │   ├── card_handler.go
│   │   ├── session_handler.go
│   │   ├── admin_handler.go
│   │   ├── agent_handler.go
│   │   ├── ai_handler.go
│   │   └── ota_handler.go
│   ├── middleware/                   # 中间件
│   │   ├── auth.go                  # JWT 认证
│   │   ├── ratelimit.go             # 限流
│   │   ├── cors.go
│   │   └── logging.go               # 结构化日志
│   └── config/
│       └── config.go                # 环境变量 + 配置文件
├── pkg/                             # 公共工具包
│   ├── crypto/                      # 加密工具
│   ├── id/                          # ID 生成（UUID、短码）
│   └── response/                    # 统一响应格式
├── web/                             # 前端资源（独立 SPA）
│   └── admin/
├── migrations/                      # 数据库迁移
│   ├── 001_init.up.sql
│   └── 001_init.down.sql
├── docker-compose.yml
├── Makefile
└── go.mod
```

### 2.3 数据库设计

```sql
-- 核心表
CREATE TABLE cards (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         VARCHAR(14) UNIQUE NOT NULL,       -- Crockford Base32
    duration     INTERVAL NOT NULL,                 -- '30 days', '1 year'
    status       VARCHAR(20) DEFAULT 'unused',      -- unused/active/expired/disabled
    agent_id     UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ DEFAULT now(),
    activated_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    machine_fp   VARCHAR(64),
    metadata     JSONB DEFAULT '{}'
);

CREATE TABLE sessions (
    id           UUID PRIMARY KEY,
    card_id      UUID REFERENCES cards(id),
    machine_fp   VARCHAR(64) NOT NULL,
    ip_address   INET,
    heartbeat_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username     VARCHAR(50) UNIQUE NOT NULL,
    password     VARCHAR(128) NOT NULL,             -- bcrypt hash
    role         VARCHAR(20) NOT NULL,              -- admin/agent
    permissions  JSONB DEFAULT '[]',
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE machines (
    fingerprint  VARCHAR(64) PRIMARY KEY,
    ip_address   INET,
    first_seen   TIMESTAMPTZ DEFAULT now(),
    last_seen    TIMESTAMPTZ DEFAULT now(),
    blacklisted  BOOLEAN DEFAULT false,
    note         TEXT
);

CREATE TABLE audit_logs (
    id           BIGSERIAL PRIMARY KEY,
    actor_id     UUID,
    action       VARCHAR(50) NOT NULL,
    target_type  VARCHAR(30),
    target_id    VARCHAR(64),
    detail       JSONB,
    ip_address   INET,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE ai_usage_logs (
    id                BIGSERIAL PRIMARY KEY,
    card_id           UUID REFERENCES cards(id),
    provider          VARCHAR(30) NOT NULL,
    model             VARCHAR(50) NOT NULL,
    prompt_tokens     INT,
    completion_tokens INT,
    cost_cents        INT,
    latency_ms        INT,
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE answer_cache (
    question_hash  VARCHAR(64) PRIMARY KEY,
    platform       VARCHAR(30) NOT NULL,
    question_type  VARCHAR(20) NOT NULL,
    question_text  TEXT NOT NULL,
    answer         TEXT NOT NULL,
    model          VARCHAR(50) NOT NULL,
    confidence     REAL DEFAULT 1.0,
    hit_count      INT DEFAULT 0,
    created_at     TIMESTAMPTZ DEFAULT now(),
    last_hit_at    TIMESTAMPTZ
);

CREATE TABLE scripts (
    id          VARCHAR(30) PRIMARY KEY,            -- 'auto-exam'
    version     VARCHAR(20) NOT NULL,
    js_code     TEXT NOT NULL,
    signature   VARCHAR(128) NOT NULL,              -- HMAC 签名防篡改
    enabled     BOOLEAN DEFAULT true,
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE platform_adapters (
    id              VARCHAR(30) PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    version         VARCHAR(20) NOT NULL,
    js_code         TEXT NOT NULL,
    match_patterns  JSONB NOT NULL,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE payloads (
    id            VARCHAR(30) PRIMARY KEY,
    version       VARCHAR(20) NOT NULL,
    encrypted_dll BYTEA NOT NULL,
    encrypted_key BYTEA NOT NULL,
    sha256        VARCHAR(64) NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT now()
);

-- 索引
CREATE INDEX idx_cards_code ON cards(code);
CREATE INDEX idx_cards_status ON cards(status);
CREATE INDEX idx_cards_agent_id ON cards(agent_id);
CREATE INDEX idx_sessions_card_id ON sessions(card_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_ai_usage_logs_card_id ON ai_usage_logs(card_id);
CREATE INDEX idx_answer_cache_platform ON answer_cache(platform, question_type);
```

### 2.4 HTTP 框架

使用 Gin 或 Chi。轻量、成熟、中间件生态好。

### 2.5 认证设计

| 端 | 认证方式 |
|---|---|
| 管理后台 / 代理面板 | JWT token，无状态 |
| 客户端 API（激活、心跳、下载） | 保持现有 HMAC-SHA256 签名 + 时间戳 + 防重放 |

### 2.6 统一响应格式

```go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

### 2.7 配置管理

环境变量优先，支持 `.env` 文件和 YAML 配置：

```yaml
server:
  admin_port: 48901
  agent_port: 38472
  tls:
    cert: ./certs/server.crt
    key: ./certs/server.key
database:
  host: localhost
  port: 5432
  name: lingqiao
  user: postgres
  password: ${DB_PASSWORD}
ai:
  providers:
    - name: deepseek
      base_url: https://api.deepseek.com
      api_key: ${DEEPSEEK_API_KEY}
      models: [deepseek-v4-pro, deepseek-v4-flash]
    - name: openai
      base_url: https://api.openai.com/v1
      api_key: ${OPENAI_API_KEY}
      models: [gpt-4o, gpt-4o-mini]
```

---

## 3. 客户端重构

### 3.1 现状问题

- 39 个源文件堆在 `src/`，头文件隐式依赖
- Win32 API 散落各处，无平台抽象
- Qt5 已停维
- `build.bat` 硬编码路径
- 无单元测试

### 3.2 目标结构

```
src/
├── CMakeLists.txt
├── app/                              # 注入器可执行文件
│   ├── CMakeLists.txt
│   ├── main.cpp
│   └── ui/
│       ├── main_window.h/.cpp
│       ├── theme.h
│       ├── injection_panel.h/.cpp
│       ├── settings_panel.h/.cpp
│       └── tray_icon.h/.cpp
├── core/                             # 核心业务逻辑（无 UI 依赖）
│   ├── CMakeLists.txt
│   ├── config.h/.cpp
│   ├── session.h/.cpp
│   └── updater.h/.cpp
├── platform/                         # 平台抽象层
│   ├── CMakeLists.txt
│   ├── platform.h                    # 统一接口
│   ├── win32/
│   │   ├── win32_platform.h/.cpp
│   │   ├── win32_injector.h/.cpp
│   │   ├── win32_fingerprint.h/.cpp
│   │   └── win32_antidebug.h/.cpp
│   ├── macos/                        # 未来扩展
│   └── linux/                        # 未来扩展
├── network/                          # 网络通信层
│   ├── CMakeLists.txt
│   ├── http_client.h/.cpp            # 抽象接口
│   ├── winhttp_client.h/.cpp         # WinHTTP (Windows)
│   ├── curl_client.h/.cpp            # libcurl (跨平台)
│   └── api_types.h
├── crypto/                           # 加密模块
│   ├── CMakeLists.txt
│   ├── crypto.h/.cpp
│   ├── win32_crypto.h/.cpp           # BCrypt API
│   └── openssl_crypto.h/.cpp         # OpenSSL (跨平台)
├── hook/                             # Hook DLL 独立模块
│   ├── CMakeLists.txt
│   ├── dllmain.cpp
│   ├── cef_hook.h/.cpp
│   ├── hook_engine.h/.cpp
│   ├── vtable_prober.h/.cpp
│   └── payload.h/.cpp
├── fingerprint/                      # 机器指纹
│   ├── CMakeLists.txt
│   ├── fingerprint.h
│   ├── win32_fp.h/.cpp
│   └── aggregator.h/.cpp
└── common/                           # 公共工具
    ├── CMakeLists.txt
    ├── strcrypt.h
    ├── xor_cipher.h/.cpp
    ├── pe_validator.h/.cpp
    └── logger.h/.cpp
```

### 3.3 CMake 模块化

```cmake
# core — 依赖 network, crypto, fingerprint，不依赖 app 或 hook
add_library(core STATIC ${CORE_SOURCES})
target_link_libraries(core PUBLIC network crypto fingerprint common)

# app — 依赖 core + platform + Qt6
add_executable(injector WIN32 ${APP_SOURCES})
target_link_libraries(injector PRIVATE core platform Qt6::Widgets Qt6::Network)

# hook — 独立 DLL
add_library(CefHook SHARED ${HOOK_SOURCES})
target_link_libraries(CefHook PRIVATE common MinHook)
```

### 3.4 平台抽象层

```cpp
// platform/platform.h
class Platform {
public:
    virtual ~Platform() = default;
    virtual bool InjectDll(uint32_t pid, const std::wstring& dll_path) = 0;
    virtual std::string GetMachineFingerprint() = 0;
    virtual bool DetectDebugger() = 0;
    virtual std::vector<ProcessInfo> EnumerateProcesses() = 0;
    static std::unique_ptr<Platform> Create();
};
```

### 3.5 网络层抽象

```cpp
// network/http_client.h
class HttpClient {
public:
    struct Response {
        int status_code;
        std::string body;
        std::map<std::string, std::string> headers;
    };
    virtual Response Get(const std::string& url, const Headers& headers = {}) = 0;
    virtual Response Post(const std::string& url, const std::string& body, const Headers& headers = {}) = 0;
    virtual void SetCertPin(const std::string& sha256_fingerprint) = 0;
    static std::unique_ptr<HttpClient> Create();
};
```

### 3.6 Qt5 → Qt6 迁移要点

| 变更 | 处理 |
|---|---|
| `Qt5::WinExtras` 移除 | 替换为 Win32 API 直接调用 |
| `QTextCodec` 移除 | 使用 `QStringConverter` |
| High-DPI 默认启用 | 移除手动 `setAttribute` |
| CMake 目标名 | `Qt5::*` → `Qt6::*` |

### 3.7 构建系统

```json
// CMakePresets.json
{
  "version": 6,
  "configurePresets": [
    {
      "name": "windows-msvc",
      "generator": "Visual Studio 17 2022",
      "architecture": { "value": "Win32" },
      "cacheVariables": {
        "CMAKE_BUILD_TYPE": "Release",
        "QT_DIR": "C:/Qt/6.8/msvc2022"
      }
    }
  ]
}
```

---

## 4. AI 网关

### 4.1 现状问题

- AI 调用硬编码在 JS 中，直调 `api.deepseek.com`
- API Key 暴露在客户端
- 仅 200 条内存缓存
- 无模型切换和质量评估

### 4.2 请求流程

```
JS (注入页面) → 服务端 AI Gateway → 模型路由 → DeepSeek / GPT-4o / 通义千问
                                    ↓
                              答案缓存 (PostgreSQL)
                              质量评估
                              用量统计
```

### 4.3 模块结构

```
server/internal/service/
├── ai_gateway.go          # 网关入口
├── ai_router.go           # 模型路由策略
├── ai_provider.go         # 模型提供商接口
├── providers/
│   ├── deepseek.go
│   ├── openai.go
│   ├── qwen.go
│   └── ollama.go
├── ai_cache.go
├── ai_evaluator.go
└── ai_usage.go
```

### 4.4 模型提供商接口

```go
type ModelProvider interface {
    Name() string
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    AvailableModels() []string
    EstimateCost(tokens int) float64
}

type ChatRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    Temperature float64   `json:"temperature,omitempty"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
    Content          string `json:"content"`
    Model            string `json:"model"`
    PromptTokens     int    `json:"prompt_tokens"`
    CompletionTokens int    `json:"completion_tokens"`
    Latency          time.Duration
}
```

### 4.5 路由策略

```go
const (
    RouteByQuestionType = iota  // 选择题→flash, 论述→pro
    RouteByCost                 // 最低成本
    RouteByQuality              // 最高质量
    RouteByLatency              // 最低延迟
)
```

### 4.6 多模型交叉验证（高质量模式）

```go
func (e *Evaluator) CrossValidate(ctx context.Context, question *ExamQuestion) (*Answer, error) {
    // 2-3 个模型分别回答 → 多数投票 → 置信度评分
}
```

### 4.7 JS 端调用变更

```javascript
// 旧: 直调 DeepSeek（API Key 暴露）
fetch('https://api.deepseek.com/v1/chat/completions', { ... });

// 新: 调服务端网关
fetch('https://server:48901/api/ai/ask', {
    method: 'POST',
    body: JSON.stringify({
        question_text: questionText,
        question_type: 'single_choice',
        platform: 'chaoxing',
        options: ['A', 'B', 'C', 'D']
    })
});
```

---

## 5. 平台适配器系统

### 5.1 现状问题

- JS 硬编码超星/CX 的 DOM 选择器
- 新增平台需改源码重编译 DLL

### 5.2 目标架构

```
JS 引擎
├── 平台检测器 → 匹配 URL/DOM → 选择适配器
├── 适配器层
│   ├── chaoxing.js      (超星学习通)
│   ├── zhihuishu.js     (智慧树/知到)
│   ├── yuke.js          (雨课堂)
│   ├── tencent.js       (腾讯课堂)
│   └── generic.js       (通用 LLM 兜底)
├── 核心层
│   ├── question-parser.js
│   ├── answer-filler.js
│   ├── ai-client.js
│   ├── cache-client.js
│   └── hotkey.js
└── 适配器注册表 registry.js
```

### 5.3 适配器接口规范

```javascript
const PlatformAdapter = {
    id: 'chaoxing',
    name: '超星学习通',
    version: '1.0.0',
    matchPatterns: ['*/mooc2.chaoxing.com/*', '*/exam*chaoxing*'],

    detect() { /* 返回 true/false */ },

    parseQuestions() {
        // 返回 Question[]
        return [{
            id: 'q1',
            type: 'single_choice',
            text: '以下哪个是...',
            options: [{ key: 'A', text: '选项A' }],
            element: HTMLElement,
            metadata: {}
        }];
    },

    fillAnswer(question, answer) { /* 返回 boolean */ },
    submitAll() { /* 返回 boolean */ },

    // 可选钩子
    onPageLoad() {},
    onQuestionChange() {},
    onBeforeSubmit() {},
};
```

### 5.4 通用兜底适配器

当没有专用适配器匹配时，使用 LLM 智能解析页面：

```javascript
const GenericAdapter = {
    id: 'generic',
    detect() { return true; },

    async parseQuestions() {
        const html = document.body.innerHTML;
        const response = await aiClient.ask({
            type: 'parse_page',
            prompt: '分析以下 HTML，提取所有考试题目，返回 JSON 数组',
            content: this.truncate(html, 8000)
        });
        return JSON.parse(response.content);
    }
};
```

### 5.5 适配器热更新

```
服务端:
GET /api/adapters             → 适配器列表
GET /api/adapters/:id         → 单个适配器 JS 代码
GET /api/adapters/:id/version → 版本号

客户端: 启动时拉取 → 对比版本 → 下载更新 → 动态加载
```

---

## 6. 插件系统

### 6.1 目标

DLL 功能从单一 JS 注入扩展为可插拔架构，支持外部插件动态加载。

### 6.2 插件接口

```cpp
class IPlugin {
public:
    virtual ~IPlugin() = default;
    virtual const char* Id() const = 0;
    virtual const char* Name() const = 0;
    virtual const char* Version() const = 0;

    virtual bool OnLoad(PluginHost* host) = 0;
    virtual void OnUnload() = 0;

    virtual void OnBrowserCreated(void* browser) {}
    virtual void OnPageLoaded(const char* url) {}
    virtual void OnMessage(const char* msg) {}
};

class PluginHost {
public:
    virtual void InjectScript(const char* js_code) = 0;
    virtual void RegisterHook(HookType type, HookCallback cb) = 0;
    virtual void SendMessage(const char* target, const char* msg) = 0;
    virtual const char* GetConfig(const char* key) = 0;
    virtual void* GetCefBrowser() = 0;
};

#define DECLARE_PLUGIN(PluginClass)                            \
    extern "C" __declspec(dllexport) IPlugin* CreatePlugin() { \
        return new PluginClass();                              \
    }                                                          \
    extern "C" __declspec(dllexport) void DestroyPlugin(IPlugin* p) { \
        delete p;                                              \
    }
```

### 6.3 内置插件

| 插件 | 功能 |
|---|---|
| `js-injector` | 核心 JS 注入（现有功能重构），从服务端热加载 JS |
| `screen-capture` | 屏幕截图/录屏 |
| `ocr-engine` | 屏幕 OCR 识别 (PaddleOCR/Tesseract) |
| `clipboard-sync` | 剪贴板同步 |

### 6.4 JS 脚本热更新

```
编译时: DLL 不再嵌入 JS 代码
运行时: DLL 启动 → 从服务端下载最新 JS → 本地缓存
        → 每次页面加载使用缓存的 JS
        → 定期检查服务端新版本
```

---

## 7. 安全加固

### 7.1 反调试升级

现有 14 项保留，新增：

| 检测项 | 方法 |
|---|---|
| 虚拟机检测 | CPUID 检测 VM 厂商标识 |
| 沙箱检测 | 磁盘大小、进程列表特征 |
| 内核调试 | NtQuerySystemInformation(SystemKernelDebuggerInformation) |
| 时序异常 | RDTSC 指令计时检测单步调试 |
| 内存完整性 | 关键代码段 CRC 校验（防 patch） |

### 7.2 通信安全升级

```
现状: HMAC-SHA256 签名 + TLS 证书锁定

增强:
  ECDH 密钥协商 + AES-256-GCM 加密通信
  + HMAC-SHA256 签名 + 时间戳防重放
  + TLS 证书锁定 + 证书透明度校验
  + 请求序列号（防重排序攻击）
```

```cpp
class SecureChannel {
public:
    bool Handshake(const std::string& server_pubkey);
    std::vector<uint8_t> Encrypt(const std::vector<uint8_t>& plaintext);
    std::vector<uint8_t> Decrypt(const std::vector<uint8_t>& ciphertext);
private:
    std::array<uint8_t, 32> shared_secret_;
    uint64_t sequence_number_ = 0;
};
```

### 7.3 DLL 传输安全

```
服务端:
  1. DLL 用随机 AES-256 密钥加密
  2. 密钥用客户端 RSA 公钥加密（密钥封装）
  3. 一起打包传输

客户端:
  1. RSA 私钥解密出 AES 密钥
  2. AES 密钥解密 DLL
  3. 校验 PE 签名 + 完整性哈希
```

### 7.4 代码保护

| 层级 | 措施 |
|---|---|
| 编译时 | 字符串加密、控制流平坦化 |
| 链接时 | 导入表混淆、延迟加载 |
| 运行时 | 代码段 CRC 校验、反 dump |
| 发布时 | VMProtect/Themida 加壳（可选） |

### 7.5 配置加密

- 服务器地址和密钥不再硬编码
- 首次运行从服务端获取配置（一次性激活码）
- 本地存储 AES-256-GCM + 机器指纹绑定
- 配置文件防拷贝

---

## 8. 工程基础设施

### 8.1 CI/CD

GitHub Actions 流水线：

- **server.yml**: Go 测试 → golangci-lint → 构建 → PostgreSQL 集成测试
- **client.yml**: MSVC 构建 → Qt6 → CTest → 产物上传
- **release.yml**: tag 触发 → 构建 → 代码签名 → 打包 → GitHub Release

### 8.2 测试体系

```
测试金字塔:
├── E2E 测试 (少量)    — Playwright: 注入→答题→提交完整流程
├── 集成测试 (适量)    — 真实 DB: API 端点完整请求/响应
└── 单元测试 (大量)    — Mock 依赖: 每个函数/模块独立验证
```

服务端: Go testing + testify + 内存仓库 mock
客户端: Google Test + CTest

### 8.3 静态分析

- 服务端: golangci-lint (errcheck, govet, staticcheck, bodyclose, sqlclosecheck)
- 客户端: Clang-Tidy + MSVC /W4 /WX

### 8.4 版本管理

```
格式: vMAJOR.MINOR.PATCH
提交规范: conventional commits (feat/fix/refactor/docs/BREAKING CHANGE)
变更日志: 自动生成 CHANGELOG.md
```

### 8.5 文档体系

```
docs/
├── architecture.md
├── api/
│   ├── openapi.yaml
│   └── client-api.md
├── development/
│   ├── setup.md
│   ├── building.md
│   ├── testing.md
│   └── contributing.md
├── deployment/
│   ├── server-deploy.md
│   └── client-release.md
└── plugins/
    └── plugin-dev-guide.md
```

### 8.6 Docker Compose

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: lingqiao
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

  server:
    build: .
    depends_on: [postgres]
    ports: ["48901:48901", "38472:38472"]
```

---

## 9. 数据迁移策略

### 9.1 服务端迁移

现有 JSON 数据一次性导入 PostgreSQL：

```
JSON cards.json     → INSERT INTO cards
JSON sessions.json  → INSERT INTO sessions
JSON agents.json    → INSERT INTO users (role='agent')
JSON blacklist.json → INSERT INTO machines (blacklisted=true)
```

提供迁移脚本 `cmd/migrate-json/main.go`，迁移后验证数据完整性。

### 9.2 客户端迁移

- DLL 不再从服务端下载，改为内置在安装包中
- 已有的加密会话文件向后兼容，v3 客户端可读取 v2 会话
- 首次运行 v3 时自动迁移本地配置

---

## 10. 里程碑计划

```
M1 — 服务端基础重构 (2-3周)
├── PostgreSQL schema + 迁移脚本
├── Go 项目结构搭建 (domain/repository/service/handler)
├── card_service + session_service 核心逻辑
├── Gin/Chi 路由 + JWT 中间件
├── JSON 数据迁移工具
└── Docker Compose 环境

M2 — 客户端模块化重构 (2-3周)
├── CMake 模块化拆分
├── 平台抽象层 (Platform 接口 + Win32 实现)
├── 网络层抽象 (HttpClient 接口)
├── Qt5 → Qt6 迁移
├── 单元测试框架 (Google Test)
└── CMakePresets.json 替代 build.bat

M3 — AI 网关 (1-2周)
├── 多模型提供商接口
├── DeepSeek + OpenAI 适配器
├── 模型路由策略
├── 答案缓存 (PostgreSQL)
├── 用量统计
└── JS 端调用改为走服务端

M4 — 平台适配器系统 (2周)
├── 适配器接口规范
├── 超星/CX 适配器（从现有 JS 重构）
├── 通用兜底适配器 (LLM 智能解析)
├── 平台检测器
├── 答案填写引擎
└── 适配器热更新机制

M5 — 插件系统 (1-2周)
├── IPlugin 接口 + PluginHost
├── 插件管理器 (动态加载/卸载)
├── js-injector 插件重构（从 DLL 内嵌改为运行时加载）
├── JS 脚本热更新
└── 示例外部插件

M6 — 安全加固 (1-2周)
├── 反调试升级 (VM/沙箱/内核/时序/内存)
├── 通信安全升级 (ECDH + AES-256-GCM)
├── DLL 传输安全 (RSA 密钥封装)
├── 配置加密
└── 代码保护 (编译/链接/运行时)

M7 — 工程化收尾 (1-2周)
├── CI/CD 流水线
├── API 文档 (OpenAPI)
├── 开发者文档
├── 代码签名
└── 首个 v3.0.0-rc1 发布
```

---

## 设计决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 服务端架构 | 模块化单体 | 小范围用户，无需微服务复杂度 |
| 数据库 | PostgreSQL | JSON → 正式 DB，支持事务/索引/并发 |
| HTTP 框架 | Gin 或 Chi | 轻量成熟，中间件生态好 |
| 客户端技术 | C++/Qt6 | 团队熟悉，Qt6 支持跨平台 |
| AI 模型 | 多模型网关 | 解耦客户端和模型提供商 |
| 适配器系统 | JS 动态加载 | 新平台无需重编译 DLL |
| 插件系统 | DLL 动态加载 | 扩展性好，C++ 原生性能 |
| 认证 | JWT + HMAC | 管理端无状态，客户端保持安全模型 |
