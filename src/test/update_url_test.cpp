#include "update_url.h"

#include <iostream>

static int Expect(bool condition, const char* message) {
    if (condition) return 0;
    std::cerr << message << std::endl;
    return 1;
}

int main() {
    int failures = 0;

    auto httpsTarget = ResolveUpdateDownloadTarget(
        "https://cdn.example.com/releases/LingqiaoSetup.exe?token=abc",
        "download", L"api.example.com", 8443);
    failures += Expect(httpsTarget.error.isEmpty(), "https target should be valid");
    failures += Expect(httpsTarget.secure, "https target should be secure");
    failures += Expect(httpsTarget.host == L"cdn.example.com", "https target host should come from URL");
    failures += Expect(httpsTarget.port == INTERNET_DEFAULT_HTTPS_PORT, "https target should default to port 443");
    failures += Expect(httpsTarget.path == L"/releases/LingqiaoSetup.exe?token=abc", "https target should preserve path and query");

    auto httpTarget = ResolveUpdateDownloadTarget(
        "http://downloads.example.com/pkg/setup.exe",
        "download", L"api.example.com", 8443);
    failures += Expect(httpTarget.error.isEmpty(), "http target should be valid");
    failures += Expect(!httpTarget.secure, "http target should not be secure");
    failures += Expect(httpTarget.port == INTERNET_DEFAULT_HTTP_PORT, "http target should default to port 80");

    auto relativeTarget = ResolveUpdateDownloadTarget(
        "/api/v1/update/download?id=42",
        "download", L"api.example.com", 8443);
    failures += Expect(relativeTarget.error.isEmpty(), "relative target should be valid");
    failures += Expect(relativeTarget.secure, "relative target should inherit secure default");
    failures += Expect(relativeTarget.host == L"api.example.com", "relative target should use default host");
    failures += Expect(relativeTarget.port == 8443, "relative target should use default port");
    failures += Expect(relativeTarget.path == L"/api/v1/update/download?id=42", "relative target should preserve relative path");

    return failures == 0 ? 0 : 1;
}
