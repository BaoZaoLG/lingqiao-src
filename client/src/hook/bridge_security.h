#pragma once

#include <cstddef>
#include <string>

std::string BridgeSanitizeLogField(const std::string& value, size_t maxBytes);
std::string BridgeRedactSensitiveLogField(const std::string& value, size_t maxBytes);
std::string BridgeRedactUrlForLog(const std::string& value, size_t maxBytes);
bool BridgeUrlsHaveSameOrigin(const std::string& left, const std::string& right);
