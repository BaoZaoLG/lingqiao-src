#include "api_client.h"

#include "network/http_client.h"
#include "config.h"
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonArray>
#include <QJsonParseError>

ApiClient::ApiClient(const wchar_t* host, int port)
    : m_host(host), m_port(port) {}

ApiError ApiClient::parseError(const ApiResult& rawResp, const QJsonObject& defaultBodyIfParsed) const {
    ApiError err;
    if (rawResp.statusCode == 0 || !rawResp.error.isEmpty()) {
        err.category = ApiErrorCategory::Network;
        err.status = 0;
        err.detail = rawResp.error.isEmpty() ? QString::fromUtf8("无法连接服务器，请检查网络") : rawResp.error;
        return err;
    }
    
    QJsonObject obj = defaultBodyIfParsed;
    if (obj.isEmpty() && !rawResp.body.isEmpty()) {
        QJsonParseError jsonParseError{};
        QJsonDocument doc = QJsonDocument::fromJson(rawResp.body, &jsonParseError);
        if (jsonParseError.error == QJsonParseError::NoError && doc.isObject()) {
            obj = doc.object();
        } else if (rawResp.statusCode >= 200 && rawResp.statusCode < 300) {
            err.category = ApiErrorCategory::Parse;
            err.status = rawResp.statusCode;
            err.detail = QString::fromUtf8("服务器响应格式错误");
            return err;
        }
    }

    err.status = rawResp.statusCode;
    if (rawResp.statusCode >= 200 && rawResp.statusCode < 300) {
        if (obj["status"].toString() != "ok") {
            err.category = ApiErrorCategory::Business;
            err.detail = obj.contains("detail") ? obj["detail"].toString() : obj["message"].toString(QString::fromUtf8("操作失败"));
        }
    } else {
        if (rawResp.statusCode >= 500) {
            err.category = ApiErrorCategory::Http;
            err.detail = QString::fromUtf8("服务器暂时不可用 (HTTP %1)，请稍后重试").arg(rawResp.statusCode);
        } else if (rawResp.statusCode == 429) {
            err.category = ApiErrorCategory::Http;
            err.detail = QString::fromUtf8("请求过于频繁，请稍后再试");
        } else if (rawResp.statusCode == 401) {
            err.category = ApiErrorCategory::Business;
            err.detail = QString::fromUtf8("签名验证失败或会话已失效");
        } else if (rawResp.statusCode == 403) {
            err.category = ApiErrorCategory::Business;
            err.detail = obj.contains("detail") ? obj["detail"].toString() : obj["message"].toString(QString::fromUtf8("权限拒绝"));
        } else {
            err.category = ApiErrorCategory::Http;
            err.detail = QString::fromUtf8("请求失败 (HTTP %1)").arg(rawResp.statusCode);
        }

        // Check if there is a more specific error detail in JSON
        if (!obj.isEmpty()) {
            if (obj.contains("code")) err.code = obj["code"].toString();
            if (obj.contains("detail")) err.detail = obj["detail"].toString();
            else if (obj.contains("message")) err.detail = obj["message"].toString();
        }
    }

    return err;
}

ApiResultTyped<ActivationResponse> ApiClient::activate(const QString& cardCode, const QString& machineId, const QString& fingerprint, const QString& clientVersion) const {
    QJsonObject req;
    req["client_id"]      = QString::fromWCharArray(CLIENT_ID);
    req["card"]           = cardCode;
    req["machine_id"]     = machineId;
    req["fingerprint"]    = fingerprint;
    req["client_version"] = clientVersion;

    ApiResult rawResp = postJson(g_pathAct, req);
    QJsonObject obj;
    QJsonParseError jsonParseError{};
    QJsonDocument doc = QJsonDocument::fromJson(rawResp.body, &jsonParseError);
    if (jsonParseError.error == QJsonParseError::NoError && doc.isObject()) {
        obj = doc.object();
    }

    ApiError err = this->parseError(rawResp, obj);
    
    // Special handling for version rejection containing download links
    if (err.category == ApiErrorCategory::Business && rawResp.statusCode == 403 && !obj.isEmpty()) {
        if (err.detail.contains(QString::fromUtf8("版本过低"))) {
            // Keep the detail but we may need to pass update info. The controller handles this by parsing the error detail,
            // or we could add update info to the error. We will let the controller inspect the raw message if needed, 
            // but the original code emitted versionRejected with the download URL.
            // Let's attach rawMessage to carry the JSON for the controller to inspect if needed.
            err.rawMessage = QString::fromUtf8(QJsonDocument(obj).toJson(QJsonDocument::Compact));
        }
    }

    if (!err.isEmpty()) {
        return {err, std::nullopt};
    }

    ActivationResponse data;
    data.sessionToken = obj["session_token"].toString();
    data.cardExpiresAt = (qint64)obj["card_expires_at"].toDouble();

    QJsonValue uv = obj["update"];
    if (!uv.isNull() && uv.isObject()) {
        QJsonObject uo = uv.toObject();
        UpdateHint hint;
        hint.latestVersion = uo["latest_version"].toString();
        hint.downloadUrl = uo["download_url"].toString();
        hint.forceUpdate = uo["force_update"].toBool(false);
        hint.sha256 = uo["sha256"].toString();
        data.updateHint = hint;
    }

    return {err, data};
}

ApiResultTyped<HeartbeatResponse> ApiClient::heartbeat(const QString& sessionToken, const QString& machineId, const QString& clientVersion) const {
    QJsonObject req;
    req["client_id"]      = QString::fromWCharArray(CLIENT_ID);
    req["session_token"]  = sessionToken;
    req["machine_id"]     = machineId;
    req["client_version"] = clientVersion;

    ApiResult rawResp = postJson(g_pathHb, req);
    QJsonObject obj;
    QJsonParseError jsonParseError{};
    QJsonDocument doc = QJsonDocument::fromJson(rawResp.body, &jsonParseError);
    if (jsonParseError.error == QJsonParseError::NoError && doc.isObject()) {
        obj = doc.object();
    }

    ApiError err = this->parseError(rawResp, obj);
    
    // Special handling for version rejection
    if (err.category == ApiErrorCategory::Business && rawResp.statusCode == 403 && !obj.isEmpty()) {
        if (err.detail.contains(QString::fromUtf8("版本过低"))) {
            err.rawMessage = QString::fromUtf8(QJsonDocument(obj).toJson(QJsonDocument::Compact));
        }
    }

    if (!err.isEmpty()) {
        return {err, std::nullopt};
    }

    HeartbeatResponse data;
    data.cardExpiresAt = (qint64)obj["card_expires_at"].toDouble();

    QJsonValue uv = obj["update"];
    if (!uv.isNull() && uv.isObject()) {
        QJsonObject uo = uv.toObject();
        UpdateHint hint;
        hint.latestVersion = uo["latest_version"].toString();
        hint.downloadUrl = uo["download_url"].toString();
        hint.forceUpdate = uo["force_update"].toBool(false);
        hint.sha256 = uo["sha256"].toString();
        data.updateHint = hint;
    }

    return {err, data};
}

ApiResultTyped<DeactivateResponse> ApiClient::deactivate(const QString& sessionToken, const QString& machineId) const {
    QJsonObject req;
    req["client_id"]      = QString::fromWCharArray(CLIENT_ID);
    req["session_token"]  = sessionToken;
    req["machine_id"]     = machineId;

    ApiResult rawResp = postJson(g_pathDeact, req);
    ApiError err = this->parseError(rawResp);
    
    if (!err.isEmpty()) {
        return {err, std::nullopt};
    }
    
    DeactivateResponse data;
    data.success = true;
    return {err, data};
}

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
