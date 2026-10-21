#include <cstdio>
#include <cstring>

extern "C" int V8SourceContainsDebuggerStatementForTest(const char* source);

static int ExpectDebuggerDecision(const char* name, const char* source, int expected) {
    int actual = V8SourceContainsDebuggerStatementForTest(source);
    if (actual != expected) {
        std::printf("FAIL %s: expected %d got %d for source: %s\n",
                    name, expected, actual, source ? source : "<null>");
        return 1;
    }
    return 0;
}

int main() {
    int failures = 0;
    failures += ExpectDebuggerDecision(
        "anonymous debugger function",
        "(function anonymous() { debugger })",
        1);
    failures += ExpectDebuggerDecision(
        "function constructor debugger string",
        "setInterval(Function('debugger'), 100)",
        1);
    failures += ExpectDebuggerDecision(
        "identifier containing debugger is allowed",
        "var debuggerFlag = true; window.mydebugger = 1;",
        0);
    failures += ExpectDebuggerDecision(
        "plain script is allowed",
        "console.log('ready');",
        0);

    if (failures) return 1;
    std::printf("PASS v8 source neutralizer tests\n");
    return 0;
}
