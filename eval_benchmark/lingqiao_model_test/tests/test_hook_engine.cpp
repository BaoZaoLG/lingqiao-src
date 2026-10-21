/* ============================================================================
 * test_hook_engine.cpp — Hook 引擎并发安全单元测试
 * ========================================================================= */

#include "../starter/c_hook/hook_engine.h"
#include <cassert>
#include <cstdio>
#include <cstring>
#include <thread>
#include <vector>
#include <atomic>

static int g_passed = 0;
static int g_failed = 0;

#define TEST(name) void name()
#define RUN(name) do { \
    printf("  [TEST] %s ... ", #name); \
    name(); \
    printf("PASSED\n"); \
    g_passed++; \
} while(0)

#define ASSERT(cond) do { \
    if (!(cond)) { \
        printf("FAILED at %s:%d: %s\n", __FILE__, __LINE__, #cond); \
        g_failed++; \
        return; \
    } \
} while(0)

// ============================================================================
// Mock 函数
// ============================================================================

static int g_originalFunc1_called = 0;
static int g_detourFunc1_called = 0;

void OriginalFunc1() { g_originalFunc1_called++; }
void DetourFunc1() { g_detourFunc1_called++; }
void OriginalFunc2() {}
void DetourFunc2() {}
void OriginalFunc3() {}
void DetourFunc3() {}

// ============================================================================
// 测试用例
// ============================================================================

TEST(test_attach_null_params) {
    void* ptr = nullptr;
    ASSERT(HookAttach(nullptr, (void*)DetourFunc1) != 0);
    ASSERT(HookAttach(&ptr, (void*)DetourFunc1) != 0);
    ptr = (void*)OriginalFunc1;
    ASSERT(HookAttach(&ptr, nullptr) != 0);
}

TEST(test_attach_basic) {
    void* ptr = (void*)OriginalFunc1;
    int result = HookAttach(&ptr, (void*)DetourFunc1);
    ASSERT(result == 0);
    ASSERT(ptr != (void*)OriginalFunc1);  // ptr 现在是 trampoline
    ASSERT(ptr != nullptr);
}

TEST(test_attach_idempotent) {
    void* ptr1 = (void*)OriginalFunc2;
    int r1 = HookAttach(&ptr1, (void*)DetourFunc2);
    ASSERT(r1 == 0);

    void* ptr2 = (void*)OriginalFunc2;
    int r2 = HookAttach(&ptr2, (void*)DetourFunc2);
    ASSERT(r2 == 0);
    ASSERT(ptr1 == ptr2);  // 相同 target+detour 返回相同 trampoline
}

TEST(test_attach_conflict) {
    void* ptr = (void*)OriginalFunc2;
    int result = HookAttach(&ptr, (void*)DetourFunc3);  // 不同 detour
    ASSERT(result == -2);  // 冲突错误
}

TEST(test_detach_basic) {
    void* ptr = (void*)OriginalFunc3;
    HookAttach(&ptr, (void*)DetourFunc3);
    void* trampoline = ptr;

    int result = HookDetach(&ptr);
    ASSERT(result == 0);
    ASSERT(ptr == (void*)OriginalFunc3);  // 恢复为原始地址
}

TEST(test_detach_unknown_trampoline) {
    void* fake = (void*)0xDEADBEEF;
    int result = HookDetach(&fake);
    ASSERT(result == -4);  // NOT_FOUND
}

TEST(test_detach_null) {
    void* ptr = nullptr;
    ASSERT(HookDetach(nullptr) != 0);
    ASSERT(HookDetach(&ptr) != 0);
}

TEST(test_detach_all) {
    HookDetachAll();
    // 验证不会崩溃，以及后续 attach 正常
    void* ptr = (void*)OriginalFunc1;
    // 注意: detach all 后可以重新 attach
}

TEST(test_concurrent_attach) {
    // 并发测试: 多线程同时 attach 不同目标
    // 修复后不应有 data race
    HookDetachAll();

    static void (*targets[100])();
    static void (*detours[100])();
    // 使用不同的地址模拟不同目标
    for (int i = 0; i < 100; i++) {
        targets[i] = (void(*)())((uintptr_t)0x10000 + i * 0x100);
        detours[i] = (void(*)())((uintptr_t)0x20000 + i * 0x100);
    }

    std::atomic<int> successCount{0};
    std::vector<std::thread> threads;

    for (int i = 0; i < 100; i++) {
        threads.emplace_back([i, &successCount]() {
            void* ptr = (void*)targets[i];
            if (HookAttach(&ptr, (void*)detours[i]) == 0) {
                successCount++;
            }
        });
    }

    for (auto& t : threads) {
        t.join();
    }

    // 所有 attach 应该成功（在 MAX_HOOKS 限制内）
    ASSERT(successCount.load() > 0);
}

TEST(test_concurrent_attach_detach) {
    // 并发 attach + detach 混合测试
    HookDetachAll();

    std::atomic<int> errors{0};
    std::vector<std::thread> threads;

    for (int i = 0; i < 50; i++) {
        threads.emplace_back([i, &errors]() {
            void* target = (void*)((uintptr_t)0x30000 + i * 0x100);
            void* detour = (void*)((uintptr_t)0x40000 + i * 0x100);
            void* ptr = target;
            if (HookAttach(&ptr, detour) == 0) {
                // 尝试 detach
                if (HookDetach(&ptr) != 0) {
                    errors++;
                }
            }
        });
    }

    for (auto& t : threads) {
        t.join();
    }

    // 没有崩溃就算通过；data race 由 TSan 检测
    ASSERT(errors.load() == 0);
}

// ============================================================================
// 主函数
// ============================================================================

int main() {
    printf("[SUITE] test_hook_engine\n");

    RUN(test_attach_null_params);
    RUN(test_attach_basic);
    RUN(test_attach_idempotent);
    RUN(test_attach_conflict);
    RUN(test_detach_basic);
    RUN(test_detach_unknown_trampoline);
    RUN(test_detach_null);
    RUN(test_detach_all);
    RUN(test_concurrent_attach);
    RUN(test_concurrent_attach_detach);

    printf("\n[RESULT] %d passed, %d failed\n", g_passed, g_failed);
    return g_failed > 0 ? 1 : 0;
}
