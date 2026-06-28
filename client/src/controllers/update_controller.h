#pragma once

#include "services/api_client.h"
#include <QString>
#include <functional>

struct UpdateCheckResult {
    bool updateAvailable = false;
    QString version;
    QString releaseNotes;
    QString downloadUrl;
    QString sha256;
    bool forceUpdate = false;
    QString error;
};

class UpdateController {
public:
    UpdateController(ApiClient* api);

    void checkUpdate(const QString& currentVersion,
                     const QString& sessionToken,
                     const QString& machineId,
                     const QString& cardCode,
                     std::function<void(UpdateCheckResult)> onComplete);

private:
    ApiClient* m_api;
};
