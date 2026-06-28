#pragma once

#include <QString>
#include <QByteArray>
#include <QJsonObject>

struct ApiResult {
    int statusCode = 0;
    QByteArray body;
    QString error;

    bool ok() const { return statusCode >= 200 && statusCode < 300 && error.isEmpty(); }
};

class ApiClient {
public:
    ApiClient(const wchar_t* host, int port);

    ApiResult postJson(const wchar_t* path, const QJsonObject& body) const;
    ApiResult getSigned(const wchar_t* path,
                        const QString& sessionToken,
                        const QString& machineId,
                        const QString& cardCode) const;

private:
    const wchar_t* m_host;
    int m_port;
};
