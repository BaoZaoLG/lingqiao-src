#include "bridge_security.h"

#include <algorithm>
#include <cctype>
#include <vector>

namespace {

std::string LowerAscii(std::string value) {
    std::transform(value.begin(), value.end(), value.begin(), [](unsigned char ch) {
        return static_cast<char>(std::tolower(ch));
    });
    return value;
}

bool IsValueDelimiter(char ch) {
    return std::isspace(static_cast<unsigned char>(ch)) != 0 ||
           ch == ',' || ch == ';' || ch == '&' || ch == '}' || ch == ']';
}

std::string ExtractOrigin(const std::string& value) {
    if (value.empty()) return "";
    for (unsigned char ch : value) {
        if (ch < 0x20 || ch == 0x7f || ch == '\\') return "";
    }

    const size_t schemeEnd = value.find("://");
    if (schemeEnd == std::string::npos || schemeEnd == 0) return "";

    const std::string scheme = LowerAscii(value.substr(0, schemeEnd));
    if (scheme != "http" && scheme != "https") return "";

    const size_t authorityStart = schemeEnd + 3;
    const size_t authorityEnd = value.find_first_of("/?#", authorityStart);
    std::string authority = value.substr(
        authorityStart,
        authorityEnd == std::string::npos ? std::string::npos : authorityEnd - authorityStart);
    if (authority.empty() || authority.find('@') != std::string::npos) return "";

    return scheme + "://" + LowerAscii(authority);
}

void RedactKeywordValues(std::string* value) {
    static const char* const keywords[] = {
        "authorization", "session_token", "sessiontoken", "api_key",
        "apikey", "password", "secret", "token"
    };

    std::string lower = LowerAscii(*value);
    for (const char* keyword : keywords) {
        size_t searchFrom = 0;
        const size_t keywordLen = std::char_traits<char>::length(keyword);
        while (true) {
            const size_t pos = lower.find(keyword, searchFrom);
            if (pos == std::string::npos) break;

            size_t valueStart = pos + keywordLen;
            while (valueStart < value->size() &&
                   std::isspace(static_cast<unsigned char>((*value)[valueStart]))) {
                ++valueStart;
            }
            if (valueStart < value->size() &&
                ((*value)[valueStart] == ':' || (*value)[valueStart] == '=')) {
                ++valueStart;
                while (valueStart < value->size() &&
                       std::isspace(static_cast<unsigned char>((*value)[valueStart]))) {
                    ++valueStart;
                }
                size_t valueEnd = valueStart;
                while (valueEnd < value->size() && !IsValueDelimiter((*value)[valueEnd])) {
                    ++valueEnd;
                }
                if (valueEnd > valueStart) {
                    value->replace(valueStart, valueEnd - valueStart, "[redacted]");
                    lower = LowerAscii(*value);
                    searchFrom = valueStart + 10;
                    continue;
                }
            }
            searchFrom = pos + keywordLen;
        }
    }
}

} // namespace

std::string BridgeSanitizeLogField(const std::string& value, size_t maxBytes) {
    if (maxBytes == 0) return "";

    std::string result;
    result.reserve(std::min(value.size(), maxBytes));
    for (unsigned char ch : value) {
        if (result.size() >= maxBytes) break;
        result.push_back((ch < 0x20 || ch == 0x7f) ? ' ' : static_cast<char>(ch));
    }
    if (value.size() > maxBytes && maxBytes >= 3) {
        result.resize(maxBytes - 3);
        result += "...";
    }
    return result;
}

std::string BridgeRedactSensitiveLogField(const std::string& value, size_t maxBytes) {
    std::string result = BridgeSanitizeLogField(value, maxBytes);
    RedactKeywordValues(&result);
    return result;
}

std::string BridgeRedactUrlForLog(const std::string& value, size_t maxBytes) {
    std::string result = BridgeSanitizeLogField(value, maxBytes);
    const size_t schemeEnd = result.find("://");
    if (schemeEnd != std::string::npos) {
        const size_t authorityStart = schemeEnd + 3;
        const size_t authorityEnd = result.find_first_of("/?#", authorityStart);
        const size_t at = result.find('@', authorityStart);
        if (at != std::string::npos &&
            (authorityEnd == std::string::npos || at < authorityEnd)) {
            result.erase(authorityStart, at - authorityStart + 1);
        }
    }
    const size_t sensitiveStart = result.find_first_of("?#");
    if (sensitiveStart != std::string::npos) {
        result.resize(sensitiveStart);
    }
    return result;
}

bool BridgeUrlsHaveSameOrigin(const std::string& left, const std::string& right) {
    const std::string leftOrigin = ExtractOrigin(left);
    return !leftOrigin.empty() && leftOrigin == ExtractOrigin(right);
}
