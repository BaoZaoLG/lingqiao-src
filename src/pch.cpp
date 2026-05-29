#include "pch.h"

#include <sstream>
#include <iomanip>

string_t IntToString(int value) {
#ifdef UNICODE
	return std::to_wstring(value);
#else
	return std::to_string(value);
#endif
}

string_t IntToHex(DWORD_PTR value) {
#ifdef UNICODE
	std::wstringstream ss;
	ss << std::hex << std::uppercase << value;
	return ss.str();
#else
	std::stringstream ss;
	ss << std::hex << std::uppercase << value;
	return ss.str();
#endif
}
