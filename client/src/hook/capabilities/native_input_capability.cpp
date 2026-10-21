/* ============================================================================
 * native_input_capability.cpp — poll-based native input + AI result mailbox
 *
 * execute_java_script from the browser process often never reaches Chaoxing
 * frames. Page→DLL console bridge works, so the page polls for pending
 * middle-click triggers and for finished AI responses.
 * ========================================================================= */

#include "pch.h"

#include <map>
#include <string>

#include "bridge_dispatcher.h"
#include "bridge_protocol.h"
#include "bridge_json.h"

namespace {

/* A trigger armed longer than this is treated as stale and discarded, so a
 * middle-click on a page with no exam DOM cannot fire an AI request later on a
 * different page. Generous enough to survive a slow exam page load. */
#define LQ_TRIGGER_TTL_MS 20000

struct TriggerMailbox {
    volatile LONG pending;
    volatile LONG armTick;   /* GetTickCount when the trigger was armed */
    char source[32];
};

TriggerMailbox g_trigger = {0, 0, {0}};

struct AiMailbox {
    CRITICAL_SECTION cs;
    bool csReady;
    std::map<std::string, std::string> results; /* id -> raw JSON result or error */

    void Ensure() {
        if (!csReady) {
            InitializeCriticalSection(&cs);
            csReady = true;
        }
    }
};

AiMailbox g_aiBox;

} // namespace

void BridgeArmNativeTrigger(const char* source) {
    if (!source) source = "poll-MBUTTON";
    strncpy_s(g_trigger.source, source, _TRUNCATE);
    InterlockedExchange(&g_trigger.armTick, (LONG)GetTickCount());
    LONG prev = InterlockedExchange(&g_trigger.pending, 1);
    char dbg[160];
    sprintf_s(dbg, "[BRIDGE] arm g_trigger@%p pending %ld->1 src=%s\n",
              (void*)&g_trigger, prev, g_trigger.source);
    OutputDebugStringA(dbg);
}

unsigned BridgeNativeTriggerPendingMs() {
    if (InterlockedCompareExchange(&g_trigger.pending, 0, 0) != 1) return 0;
    DWORD armed = (DWORD)InterlockedCompareExchange(&g_trigger.armTick, 0, 0);
    DWORD now = GetTickCount();
    if (armed == 0 || now < armed) return 0;
    return (unsigned)(now - armed);
}

void BridgeStoreAiResult(const std::string& id, const std::string& resultOrErrorJson) {
    if (id.empty()) return;
    g_aiBox.Ensure();
    EnterCriticalSection(&g_aiBox.cs);
    g_aiBox.results[id] = resultOrErrorJson;
    if (g_aiBox.results.size() > 32) {
        g_aiBox.results.erase(g_aiBox.results.begin());
    }
    LeaveCriticalSection(&g_aiBox.cs);
    char dbg[96];
    sprintf_s(dbg, "[BRIDGE] AI result stored id=%s bytes=%zu\n", id.c_str(), resultOrErrorJson.size());
    OutputDebugStringA(dbg);
}

static std::string HandleNativePollTrigger(const BridgeRequest& req) {
    /* Multiple frames poll the same mailbox. Only a frame that reports
     * ready=1 (has exam DOM) may consume the pending trigger; otherwise a
     * shell/look frame eats it and the real exam iframe never sees it. */
    std::string ready = BridgeJsonGetString(req.params, "ready");
    LONG pending = InterlockedCompareExchange(&g_trigger.pending, 0, 0);
    {
        char dbg[160];
        sprintf_s(dbg, "[BRIDGE] poll g_trigger@%p pending=%ld ready=%s\n",
                  (void*)&g_trigger, pending, ready.empty() ? "?" : ready.c_str());
        OutputDebugStringA(dbg);
    }
    if (pending == 1 && ready == "1") {
        /* Read the age BEFORE clearing the flag — afterwards it reports 0. */
        unsigned pendingMs = BridgeNativeTriggerPendingMs();
        LONG was = InterlockedExchange(&g_trigger.pending, 0);
        if (was == 1) {
            /* A long-armed trigger is stale: the user middle-clicked on a page
             * with no exam DOM and only later reached this one. Delivering it
             * would send an AI request the user never asked for. */
            if (pendingMs > LQ_TRIGGER_TTL_MS) {
                char dbg[192];
                sprintf_s(dbg, "[BRIDGE] native.pollTrigger dropped: armed %u ms ago (> %u ms TTL) "
                               "— click predates this page\n", pendingMs, (unsigned)LQ_TRIGGER_TTL_MS);
                OutputDebugStringA(dbg);
                return "{\"pending\":false}";
            }
            char out[80];
            sprintf_s(out, "{\"pending\":true,\"source\":\"%s\"}", g_trigger.source);
            OutputDebugStringA("[BRIDGE] native.pollTrigger delivered\n");
            return out;
        }
    }
    return "{\"pending\":false}";
}

static std::string HandleAiPollResult(const BridgeRequest& req) {
    std::string id = BridgeJsonGetString(req.params, "id");
    if (id.empty()) {
        return std::string("__ERROR__:") + LQ_ERR_INVALID_REQUEST + "|id required";
    }
    g_aiBox.Ensure();
    EnterCriticalSection(&g_aiBox.cs);
    auto it = g_aiBox.results.find(id);
    if (it == g_aiBox.results.end()) {
        LeaveCriticalSection(&g_aiBox.cs);
        return "{\"pending\":true}";
    }
    std::string payload = it->second;
    g_aiBox.results.erase(it);
    LeaveCriticalSection(&g_aiBox.cs);
    /* payload is either {"content":"...","finish_reason":"..."} or
     * {"error":{"code":...,"message":...}} */
    return payload;
}

void RegisterNativeInputCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_NATIVE_POLL_TRIGGER, HandleNativePollTrigger);
    BridgeRegisterMethod(LQ_METHOD_AI_POLL_RESULT, HandleAiPollResult);
}
