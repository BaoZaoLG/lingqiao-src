#pragma once
/* ============================================================================
 * cef_ref.h — CEF C API reference counting utilities
 *
 * CEF C API objects (cef_frame_t, cef_browser_t, etc.) are ref-counted via
 * their base.add_ref / base.release callbacks. This header provides safe
 * wrappers for managing these references.
 *
 * Key functions:
 *   CefAddRef(base)                 — increment refcount (NULL-safe)
 *   CefRelease(base)                — decrement refcount (NULL-safe)
 *   CefExchangeRefPointer(slot, p)  — atomically swap pointer, managing refs
 *   CefReleaseSlot(slot)            — release and NULL a stored pointer
 * ========================================================================= */

#include "include/internal/cef_types.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ---- Increment refcount (NULL-safe) ---- */
static inline void CefAddRef(cef_base_ref_counted_t* base) {
    if (base && base->add_ref)
        base->add_ref(base);
}

/* ---- Decrement refcount (NULL-safe) ---- */
static inline void CefRelease(cef_base_ref_counted_t* base) {
    if (base && base->release)
        base->release(base);
}

/* ---- Atomically exchange a stored pointer, managing refcounts.
 *
 * 1. add_ref the new pointer (if non-NULL)
 * 2. Atomically swap the stored pointer
 * 3. release the old pointer (if non-NULL)
 *
 * This ensures the stored pointer always has a valid reference.
 * Usage:
 *   CefExchangeRefPointer((void**)&g_mainFrame, &newFrame->base);
 * ---- */
static inline void CefExchangeRefPointer(void* volatile* slot,
                                          cef_base_ref_counted_t* newBase)
{
    /* add_ref new before storing */
    if (newBase)
        newBase->add_ref(newBase);

    /* Atomic swap */
    void* old = InterlockedExchangePointer((volatile PVOID*)slot, (PVOID)newBase);

    /* release old after swap */
    if (old) {
        cef_base_ref_counted_t* oldBase = (cef_base_ref_counted_t*)old;
        if (oldBase->release)
            oldBase->release(oldBase);
    }
}

/* ---- Release and NULL a stored pointer ---- */
static inline void CefReleaseSlot(void* volatile* slot) {
    void* old = InterlockedExchangePointer((volatile PVOID*)slot, NULL);
    if (old) {
        cef_base_ref_counted_t* base = (cef_base_ref_counted_t*)old;
        if (base->release)
            base->release(base);
    }
}

#ifdef __cplusplus
}
#endif
