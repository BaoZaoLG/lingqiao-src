#include "pch.h"

#include <string>

#include "keyboard_devtools.h"
#include "hook_engine.h"
#include "cef_ref.h"

/* Resolved in thread.cpp */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
typedef void(CEF_EXPORT* cef_string_clear_t)(cef_string_utf16_t*);
extern cef_string_from_ptr_t func_cef_string_from_ptr;
extern cef_string_clear_t    func_cef_string_clear;
extern PVOID g_cef_on_pre_key_event;
extern PVOID g_cef_on_key_event;
extern PVOID g_cef_get_keyboard_handler;

static bool IsDevToolsShortcut(const cef_key_event_t* event) {
    if (!event) return false;
    if (event->type != KEYEVENT_RAWKEYDOWN && event->type != KEYEVENT_KEYDOWN)
        return false;

    const uint32 required = EVENTFLAG_CONTROL_DOWN |
                            EVENTFLAG_SHIFT_DOWN |
                            EVENTFLAG_ALT_DOWN;
    if ((event->modifiers & required) != required)
        return false;

    return event->windows_key_code == VK_CONTROL ||
           event->windows_key_code == VK_LCONTROL ||
           event->windows_key_code == VK_RCONTROL ||
           event->windows_key_code == VK_SHIFT ||
           event->windows_key_code == VK_LSHIFT ||
           event->windows_key_code == VK_RSHIFT ||
           event->windows_key_code == VK_MENU ||
           event->windows_key_code == VK_LMENU ||
           event->windows_key_code == VK_RMENU;
}

static bool IsAutoExamTriggerKey(const cef_key_event_t* event) {
    if (!event) return false;
    if (event->type != KEYEVENT_RAWKEYDOWN && event->type != KEYEVENT_KEYDOWN)
        return false;
    /* F2 — native exam shell often consumes this before JS keydown. */
    return event->windows_key_code == VK_F2;
}

/* Inject a tiny JS snippet into the focused (or main) frame. */
static void ExecJsOnBrowser(struct _cef_browser_t* browser, const wchar_t* js) {
    if (!browser || !js || !func_cef_string_from_ptr) return;
    _cef_frame_t* frame = browser->get_focused_frame
        ? browser->get_focused_frame(browser)
        : NULL;
    if (!frame && browser->get_main_frame) frame = browser->get_main_frame(browser);
    if (!frame) return;

    cef_string_t eval = {0}, url = {0};
    func_cef_string_from_ptr(js, wcslen(js), &eval);
    func_cef_string_from_ptr(L"", 0, &url);
    frame->execute_java_script(frame, &eval, &url, 0);
    if (func_cef_string_clear) {
        func_cef_string_clear(&eval);
        func_cef_string_clear(&url);
    }
    CefRelease(&frame->base);
}

static void TriggerAutoExamFromNative(struct _cef_browser_t* browser, const char* source) {
    char jsUtf8[192];
    sprintf_s(jsUtf8,
        "(function(){try{if(window.AutoExam&&window.AutoExam.trigger)window.AutoExam.trigger('%s');}catch(e){}})();",
        source);
    int n = MultiByteToWideChar(CP_UTF8, 0, jsUtf8, -1, NULL, 0);
    if (n <= 1) return;
    std::wstring js((size_t)n, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, jsUtf8, -1, &js[0], n);
    OutputDebugStringA("[HOOK] Native trigger -> AutoExam.trigger\n");
    ExecJsOnBrowser(browser, js.c_str());
}

static void OpenDevTools(_cef_browser_t* browser) {
    if (!browser) return;

    _cef_browser_host_t* host = browser->get_host(browser);
    if (!host) return;

    cef_window_info_t windowInfo = {0};
    windowInfo.style = WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN |
                       WS_CLIPSIBLINGS | WS_VISIBLE;
    windowInfo.x = CW_USEDEFAULT;
    windowInfo.y = CW_USEDEFAULT;
    windowInfo.width = CW_USEDEFAULT;
    windowInfo.height = CW_USEDEFAULT;
    windowInfo.parent_window = host->get_window_handle(host);

    cef_browser_settings_t settings = {0};
    settings.size = sizeof(settings);

    if (func_cef_string_from_ptr) {
        func_cef_string_from_ptr(TEXT("LingQiao DevTools"),
                                 _tcslen(TEXT("LingQiao DevTools")),
                                 &windowInfo.window_name);
    }

    host->show_dev_tools(host, &windowInfo, NULL, &settings, NULL);

    if (func_cef_string_clear) {
        func_cef_string_clear(&windowInfo.window_name);
    }
    host->base.release(&host->base);

    OutputDebugStringA("[HOOK] DevTools opened via Ctrl+Shift+Alt\n");
}

int CEF_CALLBACK MyOnPreKeyEvent(
    struct _cef_keyboard_handler_t* self, struct _cef_browser_t* browser,
    const struct _cef_key_event_t* event, cef_event_handle_t os_event,
    int* is_keyboard_shortcut)
{
    if (IsDevToolsShortcut(event)) {
        if (is_keyboard_shortcut) *is_keyboard_shortcut = 1;
        OpenDevTools(browser);
        return 1;
    }

    if (IsAutoExamTriggerKey(event)) {
        if (is_keyboard_shortcut) *is_keyboard_shortcut = 1;
        TriggerAutoExamFromNative(browser, "native-F2");
        return 1;
    }

    if (g_cef_on_pre_key_event) {
        return ((on_pre_key_event_t)g_cef_on_pre_key_event)(
            self, browser, event, os_event, is_keyboard_shortcut);
    }
    return 0;
}

int CEF_CALLBACK MyOnKeyEvent(
    struct _cef_keyboard_handler_t* self, struct _cef_browser_t* browser,
    const struct _cef_key_event_t* event, cef_event_handle_t os_event)
{
    if (IsDevToolsShortcut(event)) {
        OpenDevTools(browser);
        return 1;
    }

    if (IsAutoExamTriggerKey(event)) {
        TriggerAutoExamFromNative(browser, "native-F2-key");
        return 1;
    }

    if (g_cef_on_key_event) {
        return ((on_key_event_t)g_cef_on_key_event)(
            self, browser, event, os_event);
    }
    return 0;
}

static void CEF_CALLBACK KeyboardFallbackAddRef(cef_base_ref_counted_t* self) {
    (void)self;
}

static int CEF_CALLBACK KeyboardFallbackRelease(cef_base_ref_counted_t* self) {
    (void)self;
    return 0;
}

static int CEF_CALLBACK KeyboardFallbackHasOneRef(cef_base_ref_counted_t* self) {
    (void)self;
    return 0;
}

static int CEF_CALLBACK KeyboardFallbackHasAtLeastOneRef(cef_base_ref_counted_t* self) {
    (void)self;
    return 1;
}

cef_keyboard_handler_t* GetFallbackKeyboardHandler() {
    static cef_keyboard_handler_t handler = {};
    static LONG initialized = 0;

    if (InterlockedCompareExchange(&initialized, 1, 0) == 0) {
        handler.base.size = sizeof(handler);
        handler.base.add_ref = KeyboardFallbackAddRef;
        handler.base.release = KeyboardFallbackRelease;
        handler.base.has_one_ref = KeyboardFallbackHasOneRef;
        handler.base.has_at_least_one_ref = KeyboardFallbackHasAtLeastOneRef;
        handler.on_pre_key_event = MyOnPreKeyEvent;
        handler.on_key_event = MyOnKeyEvent;
    }
    return &handler;
}

bool HookKeyboardCallbacks(struct _cef_keyboard_handler_t* kh) {
    if (!kh) return false;

    bool hooked = false;
    if (kh->on_pre_key_event && kh->on_pre_key_event != MyOnPreKeyEvent &&
        !g_cef_on_pre_key_event) {
        g_cef_on_pre_key_event = (PVOID)kh->on_pre_key_event;
        if (HookAttach(&g_cef_on_pre_key_event, (PVOID)MyOnPreKeyEvent) == ERROR_SUCCESS)
            hooked = true;
        else
            g_cef_on_pre_key_event = NULL;
    }
    if (kh->on_key_event && kh->on_key_event != MyOnKeyEvent &&
        !g_cef_on_key_event) {
        g_cef_on_key_event = (PVOID)kh->on_key_event;
        if (HookAttach(&g_cef_on_key_event, (PVOID)MyOnKeyEvent) == ERROR_SUCCESS)
            hooked = true;
        else
            g_cef_on_key_event = NULL;
    }
    return hooked;
}

struct _cef_keyboard_handler_t* CEF_CALLBACK MyGetKeyboardHandler(struct _cef_client_t* self)
{
    struct _cef_keyboard_handler_t* kh = NULL;
    if (g_cef_get_keyboard_handler) {
        __try {
            kh = ((get_keyboard_handler_t)g_cef_get_keyboard_handler)(self);
        }
        __except (EXCEPTION_EXECUTE_HANDLER) {
            kh = NULL;
        }
    }

    if (kh) {
        HookKeyboardCallbacks(kh);
        return kh;
    }

    return GetFallbackKeyboardHandler();
}
