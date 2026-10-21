#pragma once

bool AttachV8SignatureHooks(void);
bool AttachV8CompileHookOnly(void);
void DetachV8SignatureHooks(void);
bool AreV8HooksResolved(void);
