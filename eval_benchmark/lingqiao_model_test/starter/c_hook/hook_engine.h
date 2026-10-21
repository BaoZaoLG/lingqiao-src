#pragma once
/* ============================================================================
 * hook_engine.h — Inline Hook 引擎 API
 *
 * 提供轻量级 Inline Function Hooking 能力。
 * 支持 x86/x64 目标，内置线程安全机制。
 * ========================================================================= */
#include "platform_compat.h"

#ifdef __cplusplus
extern "C" {
#endif

/// 通过模块名和导出函数名查找函数地址
/// @return 函数地址，未找到返回 NULL
void* HookFindFunction(const char* moduleName, const char* functionName);

/// 安装 Inline Hook
///
/// @param ppPointer [in/out]
///   IN:  指向原始函数地址的指针
///   OUT: 指向 trampoline 的指针（通过 *ppPointer 调用原始函数）
///
/// @param pDetour 替换函数的地址
///
/// @return 0 成功, 非零 错误码
///
/// 线程安全。可从任意线程调用。
int HookAttach(void** ppPointer, void* pDetour);

/// 卸载之前通过 HookAttach 安装的 Hook
///
/// @param ppPointer [in/out]
///   IN:  指向 trampoline 地址的指针（HookAttach 返回的）
///   OUT: 恢复为原始函数地址
///
/// @return 0 成功, -1 未找到, 其他错误码
int HookDetach(void** ppPointer);

/// 以逆序卸载所有已安装的 Hook
/// 在运行时关闭时调用以确保安全卸载。
void HookDetachAll(void);

#ifdef __cplusplus
}
#endif
