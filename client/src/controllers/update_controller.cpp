#include "update_controller.h"

#include <QJsonDocument>
#include <QJsonObject>
#include <QtConcurrent>

UpdateController::UpdateController(ApiClient* api)
    : m_api(api) {}

void UpdateController::checkUpdate(const QString& currentVersion,
                                   const QString& sessionToken,
                                   const QString& machineId,
                                   const QString& cardCode,
                                   std::function<void(UpdateCheckResult)> onComplete) {
    QJsonObject req;
    req["client_version"] = currentVersion;
    req["machine_id"] = machineId;
    req["card_code"] = cardCode;
    req["session_token"] = sessionToken;
    req["channel"] = "stable"; // Default to stable for now

    QtConcurrent::run([this, req, onComplete]() {
        ApiResult res = m_api->postJson(L"/api/v1/update/check", req);
        UpdateCheckResult result;

        if (!res.ok()) {
            result.error = res.error.isEmpty() ? QString("HTTP %1").arg(res.statusCode) : res.error;
            onComplete(result);
            return;
        }

        QJsonDocument doc = QJsonDocument::fromJson(res.body);
        QJsonObject obj = doc.object();

        if (obj["update_available"].toBool()) {
            result.updateAvailable = true;
            QJsonObject manifest = obj["manifest"].toObject();
            result.version = manifest["version"].toString();
            result.releaseNotes = manifest["release_notes"].toString();
            result.downloadUrl = manifest["package_url"].toString();
            result.sha256 = manifest["package_sha256"].toString();
            result.forceUpdate = manifest["force_update"].toBool();
        }

        onComplete(result);
    });
}
