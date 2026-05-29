#ifndef PCH_H
#define PCH_H

#include "framework.h"
#include <tchar.h>
#include <string>

#ifdef UNICODE
typedef std::wstring string_t;
#else
typedef std::string string_t;
#endif

extern string_t IntToString(int value);
extern string_t IntToHex(DWORD_PTR value);

#endif //PCH_H
