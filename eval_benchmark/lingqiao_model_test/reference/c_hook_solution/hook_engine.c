/* ============================================================================
 * hook_engine.c — 参考解答: Inline Hook 引擎（完全修复版）
 *
 * 修复内容:
 *   - 所有 map 操作在锁保护下执行
 *   - g_hookCount 通过锁保护
 *   - FindTargetByTrampoline 持有共享锁
 *   - HookDetachAll 持有独占锁
 * ========================================================================= */
#include "hook_engine.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#define MAX_HOOKS 128

typedef struct {
    void* trampoline;
    void* target;
    void* detour;
    int   enabled;
} HookEntry;

static HookEntry g_hookMap[MAX_HOOKS];
static long g_hookCount = 0;
static pal_mutex_t g_registryLock;
static int g_lockInitialized = 0;

static void EnsureLockInit(void) {
    if (!g_lockInitialized) {
        pal_mutex_init(&g_registryLock);
        g_lockInitialized = 1;
    }
}

/* Trampoline 模拟 */
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

/* 修复: FindTargetByTrampoline 在锁保护下遍历 */
static void* FindTargetByTrampoline(void* trampoline) {
    pal_mutex_lock(&g_registryLock);
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline && g_hookMap[i].enabled) {
            pal_mutex_unlock(&g_registryLock);
            return g_hookMap[i].target;
        }
    }
    pal_mutex_unlock(&g_registryLock);
    return NULL;
}

static long FindByTarget(void* target) {
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].target == target && g_hookMap[i].enabled)
            return i;
    }
    return -1;
}

static void MarkDisabled(void* trampoline) {
    pal_mutex_lock(&g_registryLock);
    long count = g_hookCount;
    for (long i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline) {
            g_hookMap[i].enabled = 0;
            break;
        }
    }
    pal_mutex_unlock(&g_registryLock);
}

void* HookFindFunction(const char* moduleName, const char* functionName) {
    (void)moduleName;
    (void)functionName;
    return NULL;
}

/* 修复: HookAttach 全程在锁保护下完成 "查找 -> 创建 -> 注册" */
int HookAttach(void** ppPointer, void* pDetour) {
    if (!ppPointer || !*ppPointer || !pDetour)
        return -1;

    EnsureLockInit();

    void* pTarget = *ppPointer;

    /* 修复: 加锁保护整个操作序列 */
    pal_mutex_lock(&g_registryLock);

    long existingIdx = FindByTarget(pTarget);
    if (existingIdx >= 0) {
        void* existingDetour = g_hookMap[existingIdx].detour;
        void* existingTrampoline = g_hookMap[existingIdx].trampoline;

        pal_mutex_unlock(&g_registryLock);

        if (existingDetour == pDetour) {
            *ppPointer = existingTrampoline;
            return 0;
        } else {
            return -2;
        }
    }

    void* trampoline = CreateTrampoline(pTarget);
    if (!trampoline) {
        pal_mutex_unlock(&g_registryLock);
        return -3;
    }

    long idx = g_hookCount;
    if (idx >= MAX_HOOKS) {
        pal_mutex_unlock(&g_registryLock);
        return -3;
    }

    g_hookMap[idx].trampoline = trampoline;
    g_hookMap[idx].target = pTarget;
    g_hookMap[idx].detour = pDetour;
    g_hookMap[idx].enabled = 1;
    g_hookCount = idx + 1;

    pal_mutex_unlock(&g_registryLock);

    *ppPointer = trampoline;
    return 0;
}

/* 修复: HookDetach 使用独立的锁保护 FindTargetByTrampoline */
int HookDetach(void** ppPointer) {
    if (!ppPointer || !*ppPointer)
        return -1;

    EnsureLockInit();

    void* pTarget = FindTargetByTrampoline(*ppPointer);

    if (!pTarget) {
        return -4;
    }

    MarkDisabled(*ppPointer);
    *ppPointer = pTarget;
    return 0;
}

/* 修复: HookDetachAll 在锁保护下操作 */
void HookDetachAll(void) {
    EnsureLockInit();

    pal_mutex_lock(&g_registryLock);

    long count = g_hookCount;
    for (long i = count - 1; i >= 0; i--) {
        if (!g_hookMap[i].enabled) continue;
        g_hookMap[i].enabled = 0;
    }
    g_hookCount = 0;

    pal_mutex_unlock(&g_registryLock);
}
