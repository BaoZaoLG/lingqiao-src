#include "hook/bridge_json.h"
#include "hook/bridge_security.h"

#include <cstdio>
#include <string>

static int Expect(bool condition, const char* message) {
    if (!condition) {
        std::printf("FAIL %s\n", message);
        return 1;
    }
    return 0;
}

int main() {
    int failures = 0;

    failures += Expect(
        BridgeUrlsHaveSameOrigin("https://example.test/path?a=1",
                                 "https://EXAMPLE.test/other#fragment"),
        "same HTTPS origin is accepted");
    failures += Expect(
        !BridgeUrlsHaveSameOrigin("https://example.test", "http://example.test"),
        "scheme downgrade is rejected");
    failures += Expect(
        !BridgeUrlsHaveSameOrigin("https://example.test", "https://evil.test"),
        "cross-origin source is rejected");
    failures += Expect(
        !BridgeUrlsHaveSameOrigin("https://example.test",
                                  "https://user:pass@example.test/path"),
        "userinfo origin is rejected");

    std::string redactedUrl =
        BridgeRedactUrlForLog("https://user:pass@example.test/path?token=secret#x", 256);
    failures += Expect(redactedUrl == "https://example.test/path",
                       "URL credentials, query, and fragment are removed");

    std::string redactedLog =
        BridgeRedactSensitiveLogField("line1\r\nsession_token=secret-value next", 256);
    failures += Expect(redactedLog.find('\r') == std::string::npos &&
                       redactedLog.find('\n') == std::string::npos,
                       "log control characters are flattened");
    failures += Expect(redactedLog.find("secret-value") == std::string::npos,
                       "sensitive log value is redacted");

    BridgeRequest request;
    failures += Expect(
        BridgeParseRequest(
            R"({"v":2,"id":"rpc_1","type":"request","method":"runtime.ping","params":{}})",
            &request).empty(),
        "valid bridge request is accepted");
    failures += Expect(
        !BridgeParseRequest(
            R"({"v":2,"id":"rpc_1","type":"event","method":"runtime.ping","params":{}})",
            &request).empty(),
        "non-request envelope is rejected");
    failures += Expect(
        !BridgeParseRequest(
            R"({"v":2,"id":"rpc 1","type":"request","method":"runtime.ping","params":{}})",
            &request).empty(),
        "unsafe request identifier is rejected");
    failures += Expect(
        !BridgeParseRequest(
            R"({"v":2,"id":"rpc_1","type":"request","method":"runtime.ping","params":[]})",
            &request).empty(),
        "non-object params are rejected");

    if (failures) return 1;
    std::printf("PASS bridge security tests\n");
    return 0;
}
