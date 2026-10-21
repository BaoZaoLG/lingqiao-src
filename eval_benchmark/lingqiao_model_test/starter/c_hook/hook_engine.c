/* ============================================================================
 * hook_engine.c — Inline Hook 引擎实现
 *
 * 简化的 Hook 引擎，管理函数替换的注册/反注册。
 * 使用 trampoline 机制允许调用原始函数。
 *
 * 已知缺陷:
 *   - 并发安全相关的锁被注释掉了
 *   - g_hookCount 的自增操作不是原子的
 *   - FindByTarget 在无锁情况下被调用
 * ========================================================================= */
#include "hook_engine.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

/* ============================================================================
 * Internal state
 * ========================================================================= */

#define MAX_HOOKS 128

typedef struct {
    void* trampoline;   /* 调用者通过此地址调用原始函数 */
    void* target;       /* 被 Hook 的目标函数地址 */
    void* detour;       /* 替换函数地址 */
    int   enabled;      /* 1 = 激活, 0 = 已卸载 */
} HookEntry;

static HookEntry g_hookMap[MAX_HOOKS];
static long g_hookCount = 0;

/* 平台锁 */
static pal_mutex_t g_registryLock;
static int g_lockInitialized = 0;

static void EnsureLockInit(void) {
    if (!g_lockInitialized) {
        pal_mutex_init(&g_registryLock);
        g_lockInitialized = 1;
    }
}

/* ============================================================================
 * Trampoline 模拟
 * 在真实实现中，这里会生成机器码跳转指令。
 * 测试环境中用一个简单的 wrapper 模拟。
 * ========================================================================= */
typedef struct {
    void* originalFunc;
} TrampolineBlock;

static TrampolineBlock* g_trampolines[MAX_HOOKS];
static int g_trampolineCount = 0;

static void* CreateTrampoline(void* target) {
    if (g_trampolineCount >= MAX_HOOKS) return NULL;
    TrampolineBlock* t = (TrampolineBlock*)malloc(sizeof(TrampolineBlock));
    if (!t) return NULL;
    t->originalFunc = target;
    g_trampolines[g_trampolineCount++] = t;
    return (void*)t;
}

/* ============================================================================
 * 内部查找函数
 * ========================================================================= */

/* 通过 trampoline 地址查找对应的 target */
static void* FindTargetByTrampoline(void* trampoline) {
    // BUG: 这里应该持有锁再遍历，但锁操作被注释掉了
    // pal_mutex_lock(&g_registryLock);
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline && g_hookMap[i].enabled) {
            // pal_mutex_unlock(&g_registryLock);
            return g_hookMap[i].target;
        }
    }
    // pal_mutex_unlock(&g_registryLock);
    return NULL;
}

/* 检查 target 是否已被 Hook。返回索引或 -1 */
static long FindByTarget(void* target) {
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].target == target && g_hookMap[i].enabled)
            return i;
    }
    return -1;
}

/* 标记指定 trampoline 对应的 Hook 为已禁用 */
static void MarkDisabled(void* trampoline) {
    // BUG: 这里应该持有锁再修改，但锁操作被注释掉了
    // pal_mutex_lock(&g_registryLock);
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline) {
            g_hookMap[i].enabled = 0;
            break;
        }
    }
    // pal_mutex_unlock(&g_registryLock);
}

/* ============================================================================
 * Public API
 * ========================================================================= */

void* HookFindFunction(const char* moduleName, const char* functionName) {
    (void)moduleName;
    (void)functionName;
    /* 在测试环境中，模块查找通过 Mock 实现 */
    return NULL;
}

int HookAttach(void** ppPointer, void* pDetour) {
    if (!ppPointer || !*ppPointer || !pDetour)
        return -1;  /* ERROR_INVALID_PARAMETER */

    EnsureLockInit();

    void* pTarget = *ppPointer;

    // BUG: 查找和创建之间存在竞态条件
    // 正确的实现应该在加锁状态下完成 "查找 → 创建 → 注册" 的完整序列
    // 但下面的锁操作被注释掉了:
    // pal_mutex_lock(&g_registryLock);

    /* 检查是否重复 Hook 同一个 target */
    long existingIdx = FindByTarget(pTarget);
    if (existingIdx >= 0) {
        void* existingDetour = g_hookMap[existingIdx].detour;
        void* existingTrampoline = g_hookMap[existingIdx].trampoline;

        // pal_mutex_unlock(&g_registryLock);

        if (existingDetour == pDetour) {
            /* 相同 target + 相同 detour → 返回已有 trampoline（幂等） */
            *ppPointer = existingTrampoline;
            return 0;
        } else {
            /* 相同 target, 不同 detour → 冲突 */
            return -2;  /* ERROR_ALREADY_EXISTS */
        }
    }

    /* 创建 trampoline */
    void* trampoline = CreateTrampoline(pTarget);
    if (!trampoline) {
        // pal_mutex_unlock(&g_registryLock);
        return -3;  /* ERROR_OUTOFMEMORY */
    }

    /* BUG: g_hookCount 的自增不是原子操作，并发调用会导致覆盖
     * TODO: 使用原子操作或在锁保护下操作 g_hookCount */
    long idx = g_hookCount;
    if (idx >= MAX_HOOKS) {
        // pal_mutex_unlock(&g_registryLock);
        return -3;  /* ERROR_OUTOFMEMORY */
    }

    g_hookMap[idx].trampoline = trampoline;
    g_hookMap[idx].target = pTarget;
    g_hookMap[idx].detour = pDetour;
    g_hookMap[idx].enabled = 1;
    g_hookCount = idx + 1;  // BUG: 非原子写入

    // pal_mutex_unlock(&g_registryLock);

    /* 返回 trampoline 地址 */
    *ppPointer = trampoline;
    return 0;
}

int HookDetach(void** ppPointer) {
    if (!ppPointer || !*ppPointer)
        return -1;  /* ERROR_INVALID_PARAMETER */

    EnsureLockInit();

    /* BUG: FindTargetByTrampoline 内部没有锁保护，
     * 与 HookDetachAll 并发调用时可能读到半更新的状态 */
    void* pTarget = FindTargetByTrampoline(*ppPointer);

    if (!pTarget) {
        return -4;  /* ERROR_NOT_FOUND */
    }

    /* 标记为已禁用 */
    MarkDisabled(*ppPointer);

    /* 恢复调用者指针为原始 target */
    *ppPointer = pTarget;

    return 0;
}

void HookDetachAll(void) {
    EnsureLockInit();

    // BUG: 这里应该加锁，但锁操作被注释掉了
    // pal_mutex_lock(&g_registryLock);

    long count = g_hookCount;

    /* 逆序卸载 */
    for (long i = count - 1; i >= 0; i--) {
        if (!g_hookMap[i].enabled) continue;
        g_hookMap[i].enabled = 0;
    }

    g_hookCount = 0;

    // pal_mutex_unlock(&g_registryLock);
}
