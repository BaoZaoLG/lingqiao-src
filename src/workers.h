#pragma once
// ============================================================================
// Workers — background threads for server communication
// ============================================================================
#include <QObject>
#include <QJsonObject>
#include <QJsonDocument>
#include "http_client.h"
#include "config.h"
#include "strcrypt.h"

// ============================================================================
// Worker: Activation
// ============================================================================
class ActivateWorker : public QObject {
    Q_OBJECT
public:
    QString cardCode, machineID, fingerprint;
public slots:
    void process() {
        QJsonObject req;
        req["client_id"]      = QString::fromWCharArray(CLIENT_ID);
        req["card"]           = cardCode;
        req["machine_id"]     = machineID;
        req["fingerprint"]    = fingerprint;
        req["client_version"] = GetClientVersion();
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathAct, body);
        if (resp.statusCode == 200) {
            QJsonDocument doc = QJsonDocument::fromJson(resp.body);
            QJsonObject obj = doc.object();
            if (obj["status"].toString() == "ok") {
                QJsonValue uv = obj["update"];
                if (!uv.isNull() && uv.isObject()) {
                    QJsonObject uo = uv.toObject();
                    emit updateAvailable(
                        uo["latest_version"].toString(),
                        uo["download_url"].toString(),
                        uo["force_update"].toBool(false));
                }
                emit activationSuccess(obj["session_token"].toString(), (qint64)obj["card_expires_at"].toDouble());
            }
            else
                emit activationFailed(obj["message"].toString(QString::fromUtf8(_S("激活失败"))));
        } else if (resp.statusCode == 401)
            emit activationFailed(QString::fromUtf8(_S("签名验证失败")));
        else if (resp.statusCode == 403) {
            QJsonDocument doc = QJsonDocument::fromJson(resp.body);
            QString errMsg = doc.object()["message"].toString(QString::fromUtf8(_S("卡密无效或已过期")));
            if (errMsg.contains(QString::fromUtf8(_S("版本过低"))))
                emit versionRejected(errMsg, doc.object()["download_url"].toString());
            else
                emit activationFailed(errMsg);
        } else if (resp.statusCode == 0)
            emit activationFailed(resp.error.isEmpty() ? QString::fromUtf8(_S("无法连接服务器，请检查网络")) : resp.error);
        else if (resp.statusCode == 429)
            emit activationFailed(QString::fromUtf8(_S("请求过于频繁，请稍后再试")));
        else if (resp.statusCode >= 500)
            emit activationFailed(QString::fromUtf8(_S("服务器暂时不可用 (HTTP %1)，请稍后重试")).arg(resp.statusCode));
        else
            emit activationFailed(QString::fromUtf8(_S("服务器错误 (HTTP %1)")).arg(resp.statusCode));
    }
signals:
    void activationSuccess(const QString& sessionToken, qint64 cardExpiresAt);
    void activationFailed(const QString& error);
    void versionRejected(const QString& message, const QString& downloadURL);
    void updateAvailable(const QString& latestVersion, const QString& downloadURL, bool forceUpdate);
};

// ============================================================================
// Worker: Heartbeat
// ============================================================================
class HeartbeatWorker : public QObject {
    Q_OBJECT
public:
    QString sessionToken, machineID, clientVersion;
public slots:
    void process() {
        QJsonObject req;
        req["client_id"]      = QString::fromWCharArray(CLIENT_ID);
        req["session_token"]  = sessionToken;
        req["machine_id"]     = machineID;
        req["client_version"] = clientVersion;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathHb, body);
        if (resp.statusCode == 200) {
            QJsonDocument doc = QJsonDocument::fromJson(resp.body);
            QJsonObject obj = doc.object();
            if (obj["status"].toString() == "ok") {
                qint64 exp = (qint64)obj["card_expires_at"].toDouble();
                QJsonValue uv = obj["update"];
                if (!uv.isNull() && uv.isObject()) {
                    QJsonObject uo = uv.toObject();
                    emit updateAvailable(
                        uo["latest_version"].toString(),
                        uo["download_url"].toString(),
                        uo["force_update"].toBool(false));
                }
                emit heartbeatOk(exp);
            } else emit heartbeatFail();
        } else if (resp.statusCode == 401) {
            // Session expired or invalid — trigger re-auth
            emit heartbeatFail();
        } else if (resp.statusCode == 403) {
            QJsonDocument doc = QJsonDocument::fromJson(resp.body);
            QString errMsg = doc.object()["message"].toString();
            if (errMsg.contains(QString::fromUtf8(_S("版本过低"))))
                emit versionRejected(errMsg, doc.object()["download_url"].toString());
            else
                emit heartbeatFail();
        } else emit heartbeatFail();
    }
signals:
    void heartbeatOk(qint64 cardExpiresAt);
    void heartbeatFail();
    void versionRejected(const QString& message, const QString& downloadURL);
    void updateAvailable(const QString& latestVersion, const QString& downloadURL, bool forceUpdate);
};
