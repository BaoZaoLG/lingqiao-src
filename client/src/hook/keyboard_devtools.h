#pragma once
// Ctrl+Shift+Alt DevTools shortcut + CEF keyboard handler hooks.

#include "include/capi/cef_client_capi.h"
#include "include/capi/cef_keyboard_handler_capi.h"

typedef int(CEF_CALLBACK* on_pre_key_event_t)(
    struct _cef_keyboard_handler_t*, struct _cef_browser_t*,
    const struct _cef_key_event_t*, cef_event_handle_t, int*);

typedef int(CEF_CALLBACK* on_key_event_t)(
    struct _cef_keyboard_handler_t*, struct _cef_browser_t*,
    const struct _cef_key_event_t*, cef_event_handle_t);

typedef struct _cef_keyboard_handler_t*(CEF_CALLBACK* get_keyboard_handler_t)(
    struct _cef_client_t*);

int CEF_CALLBACK MyOnPreKeyEvent(
    struct _cef_keyboard_handler_t* self, struct _cef_browser_t* browser,
    const struct _cef_key_event_t* event, cef_event_handle_t os_event,
    int* is_keyboard_shortcut);

int CEF_CALLBACK MyOnKeyEvent(
    struct _cef_keyboard_handler_t* self, struct _cef_browser_t* browser,
    const struct _cef_key_event_t* event, cef_event_handle_t os_event);

cef_keyboard_handler_t* GetFallbackKeyboardHandler();
bool HookKeyboardCallbacks(struct _cef_keyboard_handler_t* kh);
struct _cef_keyboard_handler_t* CEF_CALLBACK MyGetKeyboardHandler(struct _cef_client_t* self);
