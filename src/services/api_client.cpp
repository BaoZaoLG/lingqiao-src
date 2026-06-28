#include "api_client.h"

#include "network/http_client.h"
#include <QJsonDocument>

ApiClient::ApiClient(const wchar_t* host, int port)
    : m_host(host), m_port(port) {}

ApiResult ApiClient::postJson(const wchar_t* path, const QJsonObject& body) const {
    HttpResponse resp = HttpPostJson(m_host, m_port, path, QJsonDocument(body).toJson(QJsonDocument::Compact));
    return ApiResult{resp.statusCode, resp.body, resp.error};
}

ApiResult ApiClient::getSigned(const wchar_t* path,
                               const QString& sessionToken,
                               const QString& machineId,
                               const QString& cardCode) const {
    HttpResponse resp = WinHttpGetSigned(
        m_host,
        m_port,
        path,
        reinterpret_cast<const wchar_t*>(sessionToken.utf16()),
        reinterpret_cast<const wchar_t*>(machineId.utf16()),
        reinterpret_cast<const wchar_t*>(cardCode.utf16()));
    return ApiResult{resp.statusCode, resp.body, resp.error};
}
