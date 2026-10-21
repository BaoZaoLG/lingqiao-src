#include "pch.h"

#include <mutex>
#include <string>
#include <thread>

#include "page_inject.h"
#include "script_loader.h"
#include "bridge_runtime.h"
#include "bridge_v8.h"
#include "secret_channel.h"
#include "bridge_bootstrap.js.h"
#include "cef_ref.h"
#include "include/capi/cef_frame_capi.h"

/* Same CEF string helpers as thread.cpp */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
typedef void(CEF_EXPORT* cef_string_clear_t)(cef_string_utf16_t*);
extern cef_string_from_ptr_t func_cef_string_from_ptr;
extern cef_string_clear_t    func_cef_string_clear;
extern bool g_isRendererProcess;
extern void BridgeSetScriptVersion(const std::string& version);

/* Cache server script so every frame does not re-download. */
static std::mutex g_scriptCacheMutex;
static std::string g_cachedScript;
static std::string g_cachedVersion;
static bool g_scriptCacheValid = false;

void PageScriptInjectReset() {
    /* Navigation may deliver a new script version; keep cache for the
     * process lifetime but allow refresh if fetch fails later. */
}

static void ExecJs(_cef_frame_t* frame, const TCHAR* jsCode) {
    if (!frame || !jsCode || !func_cef_string_from_ptr) return;
    cef_string_t eval = {0}, url = {0};
    func_cef_string_from_ptr(jsCode, _tcslen(jsCode), &eval);
    func_cef_string_from_ptr(TEXT(""), 0, &url);
    frame->execute_java_script(frame, &eval, &url, 0);
    if (func_cef_string_clear) {
        func_cef_string_clear(&eval);
        func_cef_string_clear(&url);
    }
}

static std::string JsEscape(const char* s) {
    std::string out;
    if (!s) return out;
    for (; *s; ++s) {
        switch (*s) {
        case '\\': out += "\\\\"; break;
        case '\'': out += "\\'"; break;
        case '"':  out += "\\\""; break;
        case '\n': out += "\\n"; break;
        case '\r': out += "\\r"; break;
        case '\t': out += "\\t"; break;
        default:
            if ((unsigned char)*s < 0x20) {
                char buf[8];
                sprintf_s(buf, "\\x%02x", (unsigned char)*s);
                out += buf;
            } else {
                out += *s;
            }
        }
    }
    return out;
}

static bool EnsureCachedScript() {
    {
        std::lock_guard<std::mutex> lock(g_scriptCacheMutex);
        if (g_scriptCacheValid && !g_cachedScript.empty()) return true;
    }
    const LqSecretBundle& secrets = LqSecretsCached();
    if (secrets.sessionToken.empty()) {
        OutputDebugStringA("[HOOK] AutoExam cache skip: no session token\n");
        return false;
    }
    std::string script;
    std::string version;
    if (!FetchVersionedScript(&script, &version) || script.empty()) {
        char dbg[160];
        sprintf_s(dbg,
            "[HOOK] AutoExam script unavailable (session_token=%d model=%d)\n",
            secrets.sessionToken.empty() ? 0 : 1,
            secrets.model.empty() ? 0 : 1);
        OutputDebugStringA(dbg);
        return false;
    }
    std::lock_guard<std::mutex> lock(g_scriptCacheMutex);
    g_cachedScript = script;
    g_cachedVersion = version;
    g_scriptCacheValid = true;
    BridgeSetScriptVersion(version);
    char dbg[128];
    sprintf_s(dbg, "[HOOK] AutoExam script cached version=%s size=%zu\n",
              version.c_str(), script.size());
    OutputDebugStringA(dbg);
    return true;
}

void InjectPageScripts(_cef_frame_t* frame) {
    if (!frame) return;

    /* Bridge bootstrap — every frame (JS bootstrap is idempotent). */
    {
        int bsLen = MultiByteToWideChar(CP_UTF8, 0, BRIDGE_BOOTSTRAP_JS, -1, NULL, 0);
        if (bsLen > 0) {
            WCHAR* bsBuf = new WCHAR[bsLen];
            MultiByteToWideChar(CP_UTF8, 0, BRIDGE_BOOTSTRAP_JS, -1, bsBuf, bsLen);
            ExecJs(frame, bsBuf);
            delete[] bsBuf;
        }
    }
    BridgeSetScriptInjected(true);

    if (g_isRendererProcess) {
        BridgeTryInstallNativeObject(frame);
    }

    if (!EnsureCachedScript()) {
        return;
    }

    const LqSecretBundle& secrets = LqSecretsCached();
    std::string remoteScript;
    {
        std::lock_guard<std::mutex> lock(g_scriptCacheMutex);
        remoteScript = g_cachedScript;
    }

    /* JS-side __AUTOEXAM_INJECTED__ prevents double init per frame — but a bare
     * guard can trap a frame permanently: it is per-window, and if an earlier
     * injection set it and the payload never finished (threw before assigning
     * window.AutoExam, or the document was swapped under it), every later
     * injection is skipped forever. That frame then has no AutoExam at all: it
     * never reports ready=1, never takes the native trigger, and nothing appears
     * in the log — indistinguishable from a dead AI path.
     *
     * window.AutoExam is assigned at the very end of the payload, so its absence
     * is a reliable "the script did not take". Re-run in that case, and report
     * which branch ran so [HOOK] "script injected" can never again be mistaken
     * for "script executed". */
    std::string js;
    js += "if (!window.__AUTOEXAM_INJECTED__ || !window.AutoExam) {\n";
    js += "window.__AUTOEXAM_RECOVERED__ = !!window.__AUTOEXAM_INJECTED__;\n";
    js += "window.__AUTOEXAM_INJECTED__ = true;\n";
    js += "window.__AUTOEXAM_AI_CONFIG = {\n";
    js += "  model: '" + JsEscape(secrets.model.c_str()) + "',\n";
    js += "  baseUrl: '" + JsEscape(secrets.baseUrl.c_str()) + "'\n";
    js += "};\n";
    js += remoteScript;
    js += "\n";
    js += "try{if(window.LingQiaoBridge&&window.LingQiaoBridge.ready){"
          "window.LingQiaoBridge.request('diagnostics.log',{level:'info',scope:'hook',"
          "message:'autoexam-ran recovered='+(window.__AUTOEXAM_RECOVERED__?1:0)+' top='+(window.top===window?1:0)+' href='+"
          "((window.location&&window.location.href)||'').substring(0,100)}).catch(function(){});}}catch(e){}\n";
    js += "} else {\n";
    js += "try{if(window.LingQiaoBridge&&window.LingQiaoBridge.ready){"
          "window.LingQiaoBridge.request('diagnostics.log',{level:'warn',scope:'hook',"
          "message:'autoexam-skip already-injected top='+(window.top===window?1:0)+' href='+"
          "((window.location&&window.location.href)||'').substring(0,100)}).catch(function(){});}}catch(e){}\n";
    js += "}\n";

    int wideLen = MultiByteToWideChar(CP_UTF8, 0, js.data(), (int)js.size(), NULL, 0);
    if (wideLen > 0) {
        WCHAR* wideBuf = new WCHAR[wideLen + 1];
        MultiByteToWideChar(CP_UTF8, 0, js.data(), (int)js.size(), wideBuf, wideLen);
        wideBuf[wideLen] = L'\0';
        ExecJs(frame, wideBuf);
        delete[] wideBuf;
    }
    OutputDebugString(TEXT("[HOOK] AutoExam script injected\n"));
}

namespace {

struct DelayedInjectCtx {
    _cef_frame_t* frame;
};

static void DelayedInjectOnUi(void* raw) {
    DelayedInjectCtx* ctx = static_cast<DelayedInjectCtx*>(raw);
    if (!ctx) return;
    _cef_frame_t* frame = ctx->frame;
    if (frame && (!frame->is_valid || frame->is_valid(frame))) {
        InjectPageScripts(frame);
    }
    if (frame) CefRelease(&frame->base);
    delete ctx;
}

static void ScheduleOneRetry(_cef_frame_t* frame, DWORD delayMs) {
    if (!frame) return;
    CefAddRef(&frame->base);
    std::thread([frame, delayMs]() {
        Sleep(delayMs);
        DelayedInjectCtx* ctx = new (std::nothrow) DelayedInjectCtx();
        if (!ctx) {
            CefRelease(&frame->base);
            return;
        }
        ctx->frame = frame;
        if (!BridgePostToUiThread(DelayedInjectOnUi, ctx)) {
            CefRelease(&frame->base);
            delete ctx;
        }
    }).detach();
}

} // namespace

void SchedulePageScriptRetries(_cef_frame_t* frame) {
    /* Two retries cover both immediate SPA replacement and the later exam
     * document commit without changing normal injection behavior. */
    ScheduleOneRetry(frame, 300);
    ScheduleOneRetry(frame, 1200);
}
