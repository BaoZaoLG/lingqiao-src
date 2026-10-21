#pragma once
// One-shot page script injection (bridge bootstrap + AutoExam).

#include "include/capi/cef_frame_capi.h"

void PageScriptInjectReset();
void InjectPageScripts(_cef_frame_t* frame);
/* Retry after navigation settles; Chaoxing may replace the document shortly
 * after OnLoadEnd, making the first execute_java_script a no-op. */
void SchedulePageScriptRetries(_cef_frame_t* frame);
