# CEF / V8 诊断研究路线图

本文档总结当前 CEF/V8 运行时诊断方向的研究进展。范围限定在授权环境下的本地调试、进程角色识别、V8 特征码验证、Hook 健康检查和稳定性测试。

本项目将诊断能力与内容采集、绕过限制等行为明确分离。核心目标是理解 CEF 多进程应用中 V8 的实际运行位置，并验证内部 V8 锚点能否被稳定定位和命中。

## 1. 背景

CEF 应用通常采用多进程模型。同一个可执行文件可能启动多个不同角色的子进程：

- `browser`：主进程，通常没有 `--type=` 参数。
- `renderer`：页面渲染进程，负责运行网页和 V8 JavaScript。
- `gpu-process`：GPU / ANGLE / 图形相关进程。
- `utility`：网络、服务等辅助进程。

对于 V8 诊断来说，关键目标是 Renderer 进程：

```text
--type=renderer
```

在 Browser、GPU 或 Utility 进程里安装 V8 hook，可能在二进制层面安装成功，但这些进程不一定会执行页面 JavaScript，因此不会稳定触发 V8 页面运行时相关函数。

## 2. 当前研究结论

当前已经验证以下链路：

```text
在 libcef.dll 中定位 V8 特征码
-> 解析唯一运行时地址
-> 将诊断 DLL 注入 Renderer 进程
-> 安装 V8 hook stub
-> 观察到 V8 hook hit
```

目前确认的高价值 V8 锚点包括：

```text
v8.script_compiler.compile_script.func
v8.script_run.xref
v8.function_template.set_call_handler.func
v8.function_template.new.func
v8.function_template.new.alt_func
```

其中最重要的编译路径锚点是：

```text
v8.script_compiler.compile_script.func
```

该锚点与 V8 脚本编译路径相关，适合作为后续非侵入式参数诊断的主要入口。

## 3. 诊断范围

建议支持的诊断能力：

- 根据命令行识别当前 CEF 进程角色。
- DLL 启动时输出进程身份日志。
- 扫描当前进程内 `libcef.dll` 的可执行节。
- 使用 V8 字节特征码定位内部函数。
- 输出每个签名的命中数量。
- 仅当目标签名唯一命中时才安装 hook。
- 统计每个 V8 锚点的 hit 次数。
- 限制每个 hook 的日志输出量，避免日志噪声。
- 初期只记录参数指针快照，不做深度解引用。

不建议纳入本研究范围：

- 采集私有页面内容。
- 提取受保护数据。
- 在编译前修改 JavaScript。
- 绕过应用、平台或考试类限制。
- 隐藏模块、提权、隐蔽注入等行为。

## 4. 进程角色识别

第一步诊断应当确认 DLL 被注入到了哪个 CEF 进程。

示例日志：

```text
[CefHook] process pid=1234 ppid=5678 role=renderer exe="C:\App\App.exe" cmd="C:\App\App.exe --type=renderer ..."
```

只有 `role=renderer` 的进程适合继续做 V8 页面运行时诊断。

### 角色推断模板

```cpp
std::wstring InferCefRoleFromCommandLine(const std::wstring& commandLine) {
    const std::wstring marker = L"--type=";
    size_t pos = commandLine.find(marker);
    if (pos == std::wstring::npos) {
        return L"browser";
    }

    pos += marker.size();
    size_t end = commandLine.find_first_of(L" \t\r\n\"", pos);
    if (end == std::wstring::npos) {
        end = commandLine.size();
    }

    if (end <= pos) {
        return L"other";
    }

    return commandLine.substr(pos, end - pos);
}
```

### 进程身份日志模板

```cpp
void LogCurrentProcessIdentity() {
    DWORD pid = GetCurrentProcessId();

    wchar_t exePath[MAX_PATH * 4]{};
    GetModuleFileNameW(nullptr, exePath, _countof(exePath));

    std::wstring commandLine = GetCommandLineW();
    std::wstring role = InferCefRoleFromCommandLine(commandLine);

    wchar_t log[4096]{};
    swprintf_s(
        log,
        L"[CefHook] process pid=%lu role=%s exe=\"%s\" cmd=\"%s\"\n",
        pid,
        role.c_str(),
        exePath,
        commandLine.c_str());

    OutputDebugStringW(log);
}
```

## 5. 特征码扫描策略

V8 内部函数会随着 CEF / Chromium 版本变化而变化，因此不应硬编码静态地址。更稳妥的方式是使用带通配符的字节特征码。

推荐扫描流程：

- 在当前进程中查找 `libcef.dll`。
- 解析 PE 头。
- 只扫描可执行节。
- 支持 IDA 风格通配符特征码。
- 分别报告零命中、唯一命中和多重命中。
- 只有目标签名唯一命中时才继续安装 hook。

示例诊断日志：

```text
[V8Sig] scanning libcef.dll base=0x50000000 signatures=5
[V8Sig] hit name=v8.script_compiler.compile_script.func count=1 rva=0x00DF2150 va=0x50DF2150
```

## 6. Hook 健康检查

初期 V8 hook 应保持“只附加、只诊断”。第一个里程碑不是解析参数，而是确认 hook 能被命中且能安全回跳。

推荐日志：

```text
[V8Hook] attached name=v8.script_compiler.compile_script.func target=0x50DF2150 trampoline=0x12340F80
[V8Hook] hit name=v8.script_compiler.compile_script.func count=1
```

### Hit 计数模板

```cpp
static LONG g_v8HookHits[5] = {};
static const LONG kMaxLoggedHitsPerHook = 20;

extern "C" void __cdecl LogV8HookHit(int index) {
    if (index < 0 || index >= 5) {
        return;
    }

    LONG count = InterlockedIncrement(&g_v8HookHits[index]);
    if (count > kMaxLoggedHitsPerHook) {
        return;
    }

    char log[256]{};
    sprintf_s(
        log,
        "[V8Hook] hit index=%d count=%ld",
        index,
        count);

    OutputDebugStringA(log);
    OutputDebugStringA("\n");
}
```

## 7. 编译路径参数诊断

当确认 `v8.script_compiler.compile_script.func` 在 Renderer 中能够 hit 之后，下一步安全研究方向是记录参数指针快照。

在没有完全理解 V8 内部对象布局、handle 所有权和参数语义之前，不建议对参数做深度解引用。

### 指针快照模板

```cpp
extern "C" void __cdecl LogV8CompileArgs(
    void* a1,
    void* a2,
    void* a3,
    void* a4,
    void* a5) {
    char log[512]{};
    sprintf_s(
        log,
        "[V8Compile] a1=0x%p a2=0x%p a3=0x%p a4=0x%p a5=0x%p",
        a1,
        a2,
        a3,
        a4,
        a5);

    OutputDebugStringA(log);
    OutputDebugStringA("\n");
}
```

这些参数快照应当回到 IDA 中对照反编译结果分析。重点是判断哪些值是 handle、context、source 容器或 metadata 对象。

## 8. 推荐工具改进

### Renderer 选择界面

注入器界面建议支持：

- 进程列表刷新。
- 文本筛选。
- 显示 PID、父 PID、进程名和路径。
- 显示 CEF role。
- 高亮 Renderer 进程。
- 可选显示 `renderer-client-id`。

界面应尽量避免用户在做 V8 Renderer 诊断时误选 Browser、GPU 或 Utility 进程。

### Hook 状态面板

建议展示字段：

```text
process pid
process role
libcef base
signature name
match count
target VA
trampoline VA
attach status
hit count
```

### 稳定性测试

推荐人工测试流程：

- 启动应用。
- 进入会创建 Renderer 的页面。
- 只注入 Renderer 进程。
- 确认 V8 hook attached 日志。
- 多次刷新页面。
- 确认 hit count 递增。
- 确认 Renderer 不崩溃。
- 新建 Renderer 后重复测试。

## 9. 后续路线图

### 阶段一：可靠诊断

- 增加进程身份日志。
- 在注入器 UI 中增加 Renderer 筛选。
- 增加 V8 签名扫描日志。
- 增加 attach 状态和 hit 计数。

### 阶段二：安全运行时观察

- 增加编译路径参数指针快照。
- 将参数快照与 IDA 参数使用点关联。
- 为日志逻辑增加崩溃保护。
- 限制输出频率和输出量。

### 阶段三：签名维护

- 使用独立签名目录管理 V8 特征码。
- 记录 CEF / libcef 版本信息。
- 在条件允许时增加签名命中数量回归测试。
- 区分函数入口签名和引用点签名。

### 阶段四：开发者体验

- 增加面向 Renderer 的注入视图。
- 增加状态摘要。
- 支持导出诊断日志。
- 增加常见问题提示：
  - 进程角色选择错误。
  - DLL 构建版本过旧。
  - 特征码多重命中。
  - hook attach 失败。

## 10. 关键经验

- CEF 进程角色比 exe 名称更重要。
- 页面运行时 V8 hook 必须装在 Renderer 进程。
- 签名扫描成功不等于该函数一定是热路径。
- hook attached 成功不等于函数一定会被调用。
- `FunctionTemplate` 路径 hit 可以证明 V8 API 活动，但编译路径需要单独验证。
- `v8.script_compiler.compile_script.func` 是当前最有价值的编译路径诊断锚点。

## 11. 当前状态

当前已验证状态：

```text
Renderer 进程识别：已验证
V8 签名唯一命中：已验证
V8 hook attached：已验证
V8 hook hit：已验证
compile_script hit：已观察到
内容提取：未实现
JavaScript 改写：未实现
```

这为授权环境下的 CEF/V8 运行时诊断、签名维护、稳定性测试和后续逆向研究提供了基础。
