#pragma once
// Background network ops for the injector UI (DLL download, models, announcement).

#include <QObject>
#include <QString>
#include <QStringList>
#include <QUrl>

class NetworkService : public QObject {
    Q_OBJECT
public:
    explicit NetworkService(QObject* parent = nullptr);

    void downloadDllAsync(const QString& token, const QString& machine, const QString& card);
    void fetchModelsAsync(const QUrl& url, const QString& apiKey);
    void fetchAnnouncementAsync();

signals:
    void dllFinished(const QString& error); // empty error = success
    void modelsFinished(const QStringList& models, const QString& error);
    void announcementFinished(
        const QString& content,
        const QString& minVersion,
        const QString& latestVersion,
        const QString& downloadUrl,
        const QString& sha256,
        bool forceUpdate,
        const QString& error);

private:
    bool m_fetchingModels = false;
};
