#include "network_service.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPointer>
#include <QtConcurrent>

#include "config.h"
#include "http_client.h"
#include "dll_extractor.h"

NetworkService::NetworkService(QObject* parent) : QObject(parent) {}

void NetworkService::downloadDllAsync(const QString& token, const QString& machine, const QString& card) {
    QPointer<NetworkService> safeThis(this);
    QtConcurrent::run([safeThis, token, machine, card]() {
        QString err;
        for (int attempt = 1; attempt <= 3; ++attempt) {
            err = DownloadDll(SERVER_HOST, SERVER_PORT,
                (const wchar_t*)token.utf16(),
                (const wchar_t*)machine.utf16(),
                (const wchar_t*)card.utf16());
            if (err.isEmpty()) break;
            if (err.contains("HTTP 401") || err.contains("HTTP 403")) break;
            Sleep(800 * attempt);
        }
        QMetaObject::invokeMethod(safeThis, [safeThis, err]() {
            if (!safeThis) return;
            emit safeThis->dllFinished(err);
        }, Qt::QueuedConnection);
    });
}

void NetworkService::fetchModelsAsync(const QUrl& url, const QString& apiKey) {
    if (m_fetchingModels) return;
    m_fetchingModels = true;
    QPointer<NetworkService> safeThis(this);
    QtConcurrent::run([safeThis, url, apiKey]() {
        HttpResponse response = HttpGetBearerUrl(url, apiKey);
        QMetaObject::invokeMethod(safeThis, [safeThis, response]() {
            if (!safeThis) return;
            safeThis->m_fetchingModels = false;
            const int status = response.statusCode;
            const QByteArray body = response.body;
            const QString errorText = response.error;

            QJsonParseError parseError{};
            QJsonDocument doc = QJsonDocument::fromJson(body, &parseError);
            if (status < 200 || status >= 300 || parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                emit safeThis->modelsFinished(QStringList(),
                    QStringLiteral("HTTP %1 %2").arg(status).arg(errorText));
                return;
            }
            QJsonArray data = doc.object().value(QStringLiteral("data")).toArray();
            QStringList models;
            for (const QJsonValue& value : data) {
                QString id = value.isObject() ? value.toObject().value(QStringLiteral("id")).toString() : QString();
                if (!id.isEmpty() && !models.contains(id)) models.append(id);
            }
            emit safeThis->modelsFinished(models, QString());
        }, Qt::QueuedConnection);
    });
}

void NetworkService::fetchAnnouncementAsync() {
    QThread* thread = new QThread();
    QObject* worker = new QObject();
    worker->moveToThread(thread);
    auto host = SERVER_HOST;
    auto port = SERVER_PORT;
    auto path = g_pathAnn;
    QPointer<NetworkService> safeThis(this);

    connect(thread, &QThread::started, worker, [safeThis, thread, worker, host, port, path]() {
        HttpResponse resp = WinHttpGet(host, port, path);
        QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
            if (!safeThis) return;
            QString content, minVer, latest, downloadUrl, sha256, err;
            bool force = false;
            if (resp.statusCode == 200 && !resp.body.isEmpty()) {
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                    emit safeThis->announcementFinished({}, {}, {}, {}, {}, false,
                        QStringLiteral("malformed"));
                    return;
                }
                QJsonValue av = doc.object().value(QStringLiteral("announcement"));
                if (av.isObject()) {
                    QJsonObject anno = av.toObject();
                    content = anno.value(QStringLiteral("content")).toString();
                    minVer = anno.value(QStringLiteral("min_version")).toString();
                    if (minVer.startsWith('v') || minVer.startsWith('V')) minVer = minVer.mid(1);
                    latest = anno.value(QStringLiteral("latest_version")).toString();
                    if (latest.startsWith('v') || latest.startsWith('V')) latest = latest.mid(1);
                    downloadUrl = anno.value(QStringLiteral("download_url")).toString();
                    sha256 = anno.value(QStringLiteral("sha256")).toString();
                    force = anno.value(QStringLiteral("force_update")).toBool(false);
                }
            } else if (resp.statusCode != 0) {
                err = QStringLiteral("HTTP %1").arg(resp.statusCode);
            } else if (!resp.error.isEmpty()) {
                err = resp.error;
            }
            emit safeThis->announcementFinished(content, minVer, latest, downloadUrl, sha256, force, err);
        }, Qt::QueuedConnection);
        thread->quit();
    });
    connect(thread, &QThread::finished, worker, &QObject::deleteLater);
    connect(thread, &QThread::finished, thread, &QObject::deleteLater);
    thread->start();
}
