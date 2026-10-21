# 灵桥平台核心机制 — LLM 代码能力评测测试集

> **适用场景**: 评估大语言模型 (LLM) 在系统级 C++ Hook 开发、Go 后端并发安全开发、以及跨进程 IPC 协议实现方面的代码能力。

---

## 评测概览

本测试集从灵桥考试防作弊平台（Lingqiao Platform）的核心子系统中提炼出 **3 个梯度难度的编程任务**。每个任务包含一份带有**故意缺陷**和 `TODO` 标记的初始代码（`starter/`），被测模型需要理解代码意图并完成修复/实现。

| 任务 | 难度 | 语言 | 核心考核点 |
|------|------|------|-----------|
| Task 1: C++ Hook 引擎与特征码扫描 | ⭐⭐⭐ | C++17 | 内存安全、并发 Hook、特征码搜索边界处理 |
| Task 2: Go 代理服务端并发与鉴权 | ⭐⭐⭐ | Go 1.21+ | WebSocket 并发安全、HMAC 签名鉴权、防重放攻击 |
| Task 3: 跨进程 IPC 桥接协议 | ⭐⭐⭐⭐ | C++17 + Go | JSON 协议解析、AES-GCM 加密、状态机、错误码设计 |

---

## 目录结构

```
eval_benchmark/lingqiao_model_test/
├── PROBLEM.md                     ← 本文件（题目规范）
├── starter/
│   ├── c_hook/                    ← Task 1 & Task 3 (C++ 部分) 初始代码
│   │   ├── CMakeLists.txt
│   │   ├── pattern_scan.h / .cpp
│   │   ├── hook_engine.h / .c
│   │   ├── v8_intercept.h / .cpp
│   │   └── bridge_protocol.h
│   └── go_server/                 ← Task 2 & Task 3 (Go 部分) 初始代码
│       ├── go.mod
│       ├── agent_hub.go
│       ├── auth_middleware.go
│       └── bridge_router.go
├── reference/                     ← 标准参考解答
│   ├── c_hook_solution/
│   └── go_server_solution/
└── tests/                         ← 自动化测试与评分
    ├── test_pattern_scan.cpp
    ├── test_hook_engine.cpp
    ├── test_bridge_protocol.cpp
    ├── server_test.go
    └── run_eval.sh
```

---

## Task 1: C++ Hook 引擎内存安全与特征码扫描修复

### 背景
灵桥客户端通过在目标进程中扫描特征码定位关键函数，然后安装 Inline Hook 拦截 V8 引擎的编译流程。此模块必须在多线程环境下安全运行。

### 文件: `starter/c_hook/`

### 缺陷描述

1. **`pattern_scan.cpp` — 边界越读 (Out-of-Bounds Read)**
   - `MatchPatternAt()` 在传入 `data` 为 NULL 时缺少空指针检查（检查已被注释掉）
   - `FindPatternInModule()` 在遍历节区时，没有校验 `VirtualSize` 是否为 0 导致潜在的溢出计算
   - `ParseIdaPattern()` 对超长模式字符串（> 1024 字节）没有限制，可能导致栈溢出

2. **`hook_engine.c` — 竞态条件 (Race Condition)**
   - `HookAttach()` 中查找已有 Hook 与创建新 Hook 之间缺少锁保护（锁的获取/释放被注释掉）
   - `HookDetach()` 在查找 trampoline 时未持有锁，可能与 `HookDetachAll()` 并发冲突
   - `g_hookCount` 使用普通 LONG 而非原子操作，并发自增存在丢失风险

3. **`v8_intercept.cpp` — 异常恢复缺陷 (Exception Safety)**
   - `V8InterceptCompileScript()` 在源码替换失败时没有恢复原始内存，导致 V8 编译损坏的数据
   - 缺少 SEH (Structured Exception Handling) 保护，非法内存访问直接导致进程崩溃

### 任务要求
- 修复所有标记了 `// BUG:` 和 `// TODO:` 的缺陷
- 确保代码通过 `tests/test_pattern_scan.cpp` 和 `tests/test_hook_engine.cpp` 中的所有测试
- 修复后代码在 ThreadSanitizer 下不报任何 Data Race

### 评分标准
| 维度 | 分值 | 说明 |
|------|------|------|
| 编译通过 | 20 分 | C++17 标准，无编译错误和警告 |
| 单元测试 | 50 分 | 每个测试用例 5 分，共 10 个 |
| 内存/并发安全 | 30 分 | ASan 和 TSan 零报警 |

---

## Task 2: Go 代理服务端并发与鉴权模块

### 背景
灵桥服务端通过 Agent 代理系统管理多个客户端的连接。每个代理需要经过 HMAC 签名鉴权后才能接入 WebSocket 长连接池。系统必须防止签名重放攻击。

### 文件: `starter/go_server/`

### 缺陷描述

1. **`agent_hub.go` — 并发不安全的连接池 (Data Race)**
   - `AgentHub.clients` map 在没有锁保护的情况下被多个 goroutine 并发读写
   - `Broadcast()` 在迭代 map 时，其他 goroutine 可能同时执行 `Register()`/`Unregister()`
   - 消息发送通道没有容量限制，高负载时会导致 goroutine 泄漏

2. **`auth_middleware.go` — 重放攻击漏洞 (Replay Attack)**
   - HMAC 签名验证逻辑正确，但缺少 Nonce 去重检查
   - 时间戳校验窗口为 0（即不校验），允许任意旧请求重放
   - `VerifyHMAC()` 使用 `==` 比较而非常量时间比较 (`hmac.Equal`)，存在时序攻击风险

3. **`bridge_router.go` — 路由分发器缺陷**
   - 命令分发 `switch` 缺少 `default` 分支，未知命令被静默丢弃
   - 响应 channel 没有超时机制，未响应的请求会永久阻塞
   - 缺少请求 ID 去重，重复请求会被多次执行

### 任务要求
- 修复所有标记了 `// BUG:` 和 `// TODO:` 的缺陷
- 确保代码通过 `tests/server_test.go` 中的所有测试
- `go vet` 和 `go test -race` 均无报错

### 评分标准
| 维度 | 分值 | 说明 |
|------|------|------|
| 编译通过 | 20 分 | `go build` 无错误 |
| 单元测试 | 50 分 | 每个测试用例 5 分，共 10 个 |
| 竞态检测 | 30 分 | `-race` 检测零报警 |

---

## Task 3: 跨进程 IPC 桥接协议实现

### 背景
灵桥平台的 C++ Hook 模块需要将拦截的数据通过加密 IPC 通道发送给 Go 后端进行决策。此任务要求实现完整的双向通信协议。

### 协议规范

```
┌─────────────────────────────────────────────────────┐
│  Bridge Protocol v2 (JSON over AES-256-GCM)         │
├─────────────────────────────────────────────────────┤
│  Message Format:                                     │
│    {                                                 │
│      "version": 2,                                   │
│      "type": "request" | "response" | "event",      │
│      "id": "<uuid>",                                 │
│      "method": "<namespace>.<action>",               │
│      "params": { ... },                              │
│      "error": { "code": "...", "message": "..." }   │
│    }                                                 │
│                                                      │
│  Wire Format:                                        │
│    [4 bytes: payload length (big-endian)]            │
│    [12 bytes: AES-GCM nonce]                         │
│    [N bytes: AES-GCM encrypted JSON]                 │
│    [16 bytes: GCM authentication tag]                │
└─────────────────────────────────────────────────────┘
```

### 任务要求

**C++ 端 (`starter/c_hook/v8_intercept.cpp` 中的 IPC 部分)**:
- 实现 `BridgeEncrypt()` 和 `BridgeDecrypt()` 函数
- 实现消息序列化：将拦截事件打包为符合协议的 JSON
- 实现请求-响应匹配的状态机（基于 message ID）

**Go 端 (`starter/go_server/bridge_router.go` 中的协议部分)**:
- 实现 `DecryptBridgeMessage()` 和 `EncryptBridgeMessage()` 函数
- 实现命令分发与响应路由
- 实现超时重试机制（3 次重试，指数退避）

### 评分标准
| 维度 | 分值 | 说明 |
|------|------|------|
| 编译通过 | 20 分 | 两端均编译成功 |
| 协议兼容 | 40 分 | C++ 加密的消息 Go 能正确解密并分发 |
| 错误处理 | 20 分 | 非法输入不崩溃，返回正确错误码 |
| 状态管理 | 20 分 | 并发请求状态机正确 |

---

## 评测运行方式

### 前置条件
- C++17 编译器 (GCC 9+ / Clang 10+ / MSVC 2019+)
- Go 1.21+
- CMake 3.16+

### 执行评测

```bash
cd eval_benchmark/lingqiao_model_test/
bash tests/run_eval.sh --target starter/     # 对初始代码评测（预期不通过）
bash tests/run_eval.sh --target reference/   # 对参考解答评测（预期满分）
```

### 输出格式

评测脚本输出 JSON 格式的结果报告：

```json
{
  "total_score": 100,
  "max_score": 300,
  "tasks": [
    {
      "name": "task1_c_hook",
      "score": 40,
      "max": 100,
      "compile": true,
      "tests_passed": 6,
      "tests_total": 10,
      "sanitizer_clean": false,
      "errors": ["TSan: data race in HookAttach at hook_engine.c:149"]
    },
    {
      "name": "task2_go_server",
      "score": 35,
      "max": 100,
      "compile": true,
      "tests_passed": 5,
      "tests_total": 10,
      "race_clean": false,
      "errors": ["race detected: concurrent map read/write in AgentHub.Broadcast"]
    },
    {
      "name": "task3_bridge_protocol",
      "score": 25,
      "max": 100,
      "compile": true,
      "protocol_compat": false,
      "errors": ["decryption failed: cipher text too short"]
    }
  ]
}
```

---

## 提交规范

被测模型需要输出修复后的完整源文件。评测系统会将输出文件覆盖到 `starter/` 对应位置后重新编译和测试。

### 允许的修改
- 修复 `// BUG:` 标记的代码
- 补全 `// TODO:` 标记的空白实现
- 添加必要的头文件引用或 Go import
- 添加辅助函数/类型（不得修改公有 API 签名）

### 不允许的修改
- 修改测试文件 (`tests/` 目录)
- 修改 `CMakeLists.txt` 或 `go.mod` 中的依赖
- 删除或重命名已有的公共函数/类型
- 引入外部第三方库

---

## 注意事项

1. 本测试集从真实生产项目中提炼，代码风格和架构决策基于实际工程需求
2. 所有 Windows API 调用均已通过跨平台抽象层 (PAL) 包装，测试可在 Linux 环境运行
3. 评测环境不包含真实的 CEF/V8 动态库，所有外部依赖通过 Mock 接口模拟
4. 内存安全检测使用 AddressSanitizer，并发检测使用 ThreadSanitizer
