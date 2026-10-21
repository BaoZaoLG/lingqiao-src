#pragma once
#include <string>

/* Arm a one-shot native trigger for the page to poll via native.pollTrigger. */
void BridgeArmNativeTrigger(const char* source);

/* Milliseconds since the pending trigger was armed, or 0 when nothing is
 * pending. A trigger only leaves the mailbox for a frame that reports
 * ready=1, so a trigger that stays armed means no frame saw the exam DOM —
 * that is the difference between "the click was lost" and "there was nothing
 * to answer on this page". */
unsigned BridgeNativeTriggerPendingMs();

/* Store AI chatComplete result/error for ai.pollResult. */
void BridgeStoreAiResult(const std::string& id, const std::string& resultOrErrorJson);

void RegisterNativeInputCapabilities();
