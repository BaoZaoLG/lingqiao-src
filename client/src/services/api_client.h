#pragma once

#include <QString>
#include <QByteArray>
#include <QJsonObject>
#include <optional>

enum class ApiErrorCategory {
    None,
    Network,
    Http,
    Business,
    Parse
};

struct ApiError {
    ApiErrorCategory category = ApiErrorCategory::None;
    QString code;
    int status = 0;
    QString detail;
    QString rawMessage;

    bool isEmpty() const { return category == ApiErrorCategory::None; }
};

struct UpdateHint {
    QString latestVersion;
    QString downloadUrl;
    bool forceUpdate = false;
    QString sha256;
};

struct ActivationResponse {
    QString sessionToken;
    qint64 cardExpiresAt = 0;
    std::optional<UpdateHint> updateHint;
};

struct HeartbeatResponse {
    qint64 cardExpiresAt = 0;
    std::optional<UpdateHint> updateHint;
};

struct DeactivateResponse {
    bool success = false;
};

template<typename T>
struct ApiResultTyped {
    ApiError error;
    std::optional<T> data;
    bool ok() const { return error.isEmpty() && data.has_value(); }
};

struct ApiResult {
    int statusCode = 0;
    QByteArray body;
    QString error;

    bool ok() const { return statusCode >= 200 && statusCode < 300 && error.isEmpty(); }
};

class ApiClient {
public:
    ApiClient(const wchar_t* host, int port);

    // Typed specific endpoints
    ApiResultTyped<ActivationResponse> activate(const QString& cardCode, const QString& machineId, const QString& fingerprint, const QString& clientVersion) const;
    ApiResultTyped<HeartbeatResponse> heartbeat(const QString& sessionToken, const QString& machineId, const QString& clientVersion) const;
    ApiResultTyped<DeactivateResponse> deactivate(const QString& sessionToken, const QString& machineId) const;

    // Generic legacy endpoints
    ApiResult postJson(const wchar_t* path, const QJsonObject& body) const;
    ApiResult getSigned(const wchar_t* path,
                        const QString& sessionToken,
                        const QString& machineId,
                        const QString& cardCode) const;

    ApiError parseError(const ApiResult& rawResp, const QJsonObject& defaultBodyIfParsed = QJsonObject()) const;

private:
    const wchar_t* m_host;
    int m_port;
};
