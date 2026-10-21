// ============================================================================
// config.cpp — Single-definition home for the globals declared extern in config.h.
// Without this file every TU that includes config.h would get its own
// zero-initialised copy (the old "static in header" bug), so paths like
// g_pathAct would be empty strings when read from api_client.cpp.
// ============================================================================
#include <QString>
#include <QByteArray>
#include <QJsonDocument>
#include "config.h"

wchar_t g_hostBuf[64]     = {0};
wchar_t g_clientIdBuf[32] = {0};
char    g_secretBuf[128]  = {0};
wchar_t g_pathAct[64]     = {0};
wchar_t g_pathHb[64]      = {0};
wchar_t g_pathDeact[64]   = {0};
wchar_t g_pathAnn[64]     = {0};
BYTE    g_derivedKey[32]  = {0};

const wchar_t* SERVER_HOST   = nullptr;
const int      SERVER_PORT   = 443;
const wchar_t* CLIENT_ID     = nullptr;
const char*    CLIENT_SECRET = nullptr;
const BYTE*    HMAC_KEY      = nullptr;
