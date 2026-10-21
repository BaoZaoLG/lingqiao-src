#include "session_controller.h"
#include <QSettings>
#include <QDateTime>
#include <QtConcurrent>
#include <QJsonDocument>
#include <QJsonObject>
#include "crypto.h"

static QString injectorOrg() { return QStringLiteral("LingQiao"); }
static QString injectorApp() { return QStringLiteral("Injector"); }

SessionController::SessionController(ApiClient* apiClient, QObject* parent)
    : QObject(parent), m_apiClient(apiClient) {
}

SessionController::~SessionController() {
    delete m_apiClient;
}

void SessionController::setState(SessionState state) {
    m_state = state;
}

QString SessionController::xorDecrypt(const QString& encBase64, const QString& key) {
    QByteArray dataBytes = QByteArray::fromBase64(encBase64.toUtf8());
    QByteArray keyBytes = key.toUtf8();
    if (keyBytes.isEmpty()) return QString();
    QByteArray result;
    for (int i = 0; i < dataBytes.size(); i++) {
        result.append(dataBytes[i] ^ keyBytes[i % keyBytes.size()] ^ (char)((i * 0x9D) & 0xFF));
    }
    return QString::fromUtf8(result);
}

QString SessionController::protectToken(const QString& token) {
    return DpapiProtect(token.toUtf8());
}

QString SessionController::unprotectToken(const QString& enc, const QString& fingerprint) {
    if (enc.isEmpty()) return QString();
    QByteArray plain = DpapiUnprotect(enc);
    if (!plain.isEmpty()) return QString::fromUtf8(plain);
    // Migration: older builds stored XOR+fingerprint ciphertext.
    return xorDecrypt(enc, fingerprint);
}

void SessionController::saveSession(const QString& fingerprint) {
    Q_UNUSED(fingerprint);
    if (m_sessionToken.isEmpty()) return;
    QSettings s(injectorOrg(), injectorApp());
    s.setValue("session_token", protectToken(m_sessionToken));
    s.setValue("session_token_v", 2);
    s.setValue("machine_id", m_machineID);
    s.setValue("card_expires_at", m_cardExpiresAt);
    s.setValue("card_code", m_lastCardCode);
}

void SessionController::clearSavedSession() {
    QSettings s(injectorOrg(), injectorApp());
    s.remove("session_token");
    s.remove("session_token_v");
    s.remove("machine_id");
    s.remove("card_expires_at");
    s.remove("card_code");
}

void SessionController::queuePendingDeactivation(const QString& token, const QString& mid, const QString& fingerprint) {
    if (token.isEmpty() || mid.isEmpty()) return;
    Q_UNUSED(fingerprint);
    QSettings s(injectorOrg(), injectorApp());
    QStringList tokens = s.value("pending_deactivate_tokens").toStringList();
    QStringList machines = s.value("pending_deactivate_machines").toStringList();
    QString encToken = protectToken(token);
    if (encToken.isEmpty() || tokens.contains(encToken)) return;
    tokens.append(encToken);
    machines.append(mid);
    s.setValue("pending_deactivate_tokens", tokens);
    s.setValue("pending_deactivate_machines", machines);
}

void SessionController::flushPendingDeactivations(const QString& fingerprint) {
    if (fingerprint.isEmpty()) return;
    QSettings s(injectorOrg(), injectorApp());
    QStringList tokens = s.value("pending_deactivate_tokens").toStringList();
    QStringList machines = s.value("pending_deactivate_machines").toStringList();
    if (tokens.isEmpty() || tokens.size() != machines.size()) {
        s.remove("pending_deactivate_tokens");
        s.remove("pending_deactivate_machines");
        return;
    }
    
    ApiClient clientCopy = *m_apiClient;
    QtConcurrent::run([tokens, machines, fingerprint, clientCopy]() {
        QStringList keepTokens;
        QStringList keepMachines;
        for (int i = 0; i < tokens.size(); ++i) {
            QString token = SessionController::unprotectToken(tokens[i], fingerprint);
            QString mid = machines[i];
            if (token.isEmpty() || mid.isEmpty()) continue;

            auto resp = clientCopy.deactivate(token, mid);
            // Keep if network error or 5xx server error
            if (!resp.ok() && (resp.error.category == ApiErrorCategory::Network ||
                               (resp.error.category == ApiErrorCategory::Http && resp.error.status >= 500))) {
                keepTokens.append(tokens[i]);
                keepMachines.append(mid);
            }
        }
        QSettings out(injectorOrg(), injectorApp());
        if (keepTokens.isEmpty()) {
            out.remove("pending_deactivate_tokens");
            out.remove("pending_deactivate_machines");
        } else {
            out.setValue("pending_deactivate_tokens", keepTokens);
            out.setValue("pending_deactivate_machines", keepMachines);
        }
    });
}

void SessionController::deactivateSessionOnServer(const QString& fingerprint) {
    deactivateSpecificSessionOnServer(m_sessionToken, m_machineID, fingerprint);
}

void SessionController::deactivateSpecificSessionOnServer(const QString& token, const QString& mid, const QString& fingerprint) {
    if (token.isEmpty() || mid.isEmpty()) return;
    
    ApiClient clientCopy = *m_apiClient;
    QPointer<SessionController> safeThis(this);
    
    QtConcurrent::run([safeThis, clientCopy, token, mid, fingerprint]() {
        auto resp = clientCopy.deactivate(token, mid);
        // Only keep if network error or 5xx
        if (!resp.ok() && (resp.error.category == ApiErrorCategory::Network || 
                           (resp.error.category == ApiErrorCategory::Http && resp.error.status >= 500))) {
            QMetaObject::invokeMethod(safeThis, [safeThis, token, mid, fingerprint]() {
                if (safeThis) safeThis->queuePendingDeactivation(token, mid, fingerprint);
            }, Qt::QueuedConnection);
        }
    });
}

void SessionController::resetToInactive(const QString& reason) {
    if (m_state == SessionState::Inactive) return;
    setState(SessionState::Inactive);
    m_sessionToken.clear();
    m_machineID.clear();
    m_cardExpiresAt = 0;
    m_heartbeatFailCount = 0;
    m_heartbeatInProgress = false;
    clearSavedSession();
    emit onSessionExpired(reason);
}

void SessionController::activate(const QString& cardCode, const QString& machineId, const QString& fingerprint, const QString& clientVersion) {
    if (m_state == SessionState::Activating) return;
    setState(SessionState::Activating);
    
    QPointer<SessionController> safeThis(this);
    ApiClient clientCopy = *m_apiClient;
    
    QtConcurrent::run([safeThis, clientCopy, cardCode, machineId, fingerprint, clientVersion]() {
        auto resp = clientCopy.activate(cardCode, machineId, fingerprint, clientVersion);
        
        QMetaObject::invokeMethod(safeThis, [safeThis, resp, cardCode, machineId, fingerprint]() {
            if (!safeThis) return;
            if (resp.ok()) {
                safeThis->m_fingerprint = fingerprint;
                safeThis->m_sessionToken = resp.data->sessionToken;
                safeThis->m_machineID = machineId;
                safeThis->m_cardExpiresAt = resp.data->cardExpiresAt;
                safeThis->m_lastCardCode = cardCode;
                safeThis->setState(SessionState::Active);
                safeThis->saveSession(fingerprint);
                
                if (resp.data->updateHint) {
                    emit safeThis->onUpdateHint(*resp.data->updateHint);
                }
                emit safeThis->onActivated(safeThis->m_sessionToken, safeThis->m_cardExpiresAt);
            } else {
                safeThis->setState(SessionState::Inactive);
                if (resp.error.category == ApiErrorCategory::Business && !resp.error.rawMessage.isEmpty() && resp.error.rawMessage.contains("download_url")) {
                    QJsonDocument doc = QJsonDocument::fromJson(resp.error.rawMessage.toUtf8());
                    QJsonObject obj = doc.object();
                    emit safeThis->onVersionRejected(resp.error.detail, obj["download_url"].toString(), obj["sha256"].toString());
                } else {
                    emit safeThis->onActivationFailed(resp.error);
                }
            }
        }, Qt::QueuedConnection);
    });
}

bool SessionController::tryRestoreSession(const QString& fingerprint, const QString& clientVersion) {
    flushPendingDeactivations(fingerprint);
    
    QSettings s(injectorOrg(), injectorApp());
    QString encToken = s.value("session_token").toString();
    QString mid = s.value("machine_id").toString();
    qint64 exp = s.value("card_expires_at").toLongLong();
    QString card = s.value("card_code").toString();

    if (fingerprint.isEmpty() && !encToken.isEmpty()) {
        return false;
    }
    
    QString token = encToken.isEmpty() ? QString() : unprotectToken(encToken, fingerprint);
    if (token.isEmpty() || mid.isEmpty()) return false;
    
    if (exp > 0 && exp < QDateTime::currentSecsSinceEpoch()) {
        m_sessionToken = token;
        m_machineID = mid;
        deactivateSessionOnServer(fingerprint);
        resetToInactive("Saved session expired before restore");
        return false;
    }
    
    m_sessionToken = token;
    m_fingerprint = fingerprint;
    m_machineID = mid;
    m_cardExpiresAt = exp;
    m_lastCardCode = card;
    
    QPointer<SessionController> safeThis(this);
    ApiClient clientCopy = *m_apiClient;
    
    QtConcurrent::run([safeThis, clientCopy, token, mid, fingerprint, clientVersion]() {
        auto resp = clientCopy.heartbeat(token, mid, clientVersion);
        
        QMetaObject::invokeMethod(safeThis, [safeThis, resp, fingerprint]() {
            if (!safeThis) return;
            if (resp.ok()) {
                qint64 restoredExp = resp.data->cardExpiresAt;
                if (restoredExp > 0 && restoredExp <= QDateTime::currentSecsSinceEpoch()) {
                    safeThis->deactivateSessionOnServer(fingerprint);
                    safeThis->resetToInactive("Restored heartbeat returned expired card");
                    return;
                }
                
                safeThis->m_cardExpiresAt = restoredExp;
                safeThis->setState(SessionState::Active);
                
                if (resp.data->updateHint) {
                    emit safeThis->onUpdateHint(*resp.data->updateHint);
                }
                emit safeThis->onActivated(safeThis->m_sessionToken, safeThis->m_cardExpiresAt);
            } else {
                safeThis->deactivateSessionOnServer(fingerprint);
                safeThis->resetToInactive("Heartbeat failed during async restore");
            }
        }, Qt::QueuedConnection);
    });
    
    return true;
}

void SessionController::performHeartbeat(const QString& clientVersion) {
    if (m_state != SessionState::Active || m_sessionToken.isEmpty() || m_heartbeatInProgress) return;
    
    m_heartbeatInProgress = true;
    QPointer<SessionController> safeThis(this);
    ApiClient clientCopy = *m_apiClient;
    QString token = m_sessionToken;
    QString mid = m_machineID;
    QString fp = m_fingerprint;
    
    QtConcurrent::run([safeThis, clientCopy, token, mid, fp, clientVersion]() {
        auto resp = clientCopy.heartbeat(token, mid, clientVersion);
        
        QMetaObject::invokeMethod(safeThis, [safeThis, resp, fp]() {
            if (!safeThis) return;
            safeThis->m_heartbeatInProgress = false;
            if (safeThis->m_state != SessionState::Active) return;
            
            if (resp.ok()) {
                safeThis->m_heartbeatFailCount = 0;
                qint64 exp = resp.data->cardExpiresAt;
                if (exp > 0 && exp <= QDateTime::currentSecsSinceEpoch()) {
                    safeThis->deactivateSessionOnServer(fp);
                    safeThis->resetToInactive("Heartbeat returned expired card");
                    return;
                }
                safeThis->m_cardExpiresAt = exp;
                if (resp.data->updateHint) {
                    emit safeThis->onUpdateHint(*resp.data->updateHint);
                }
                emit safeThis->onHeartbeatOk(exp);
            } else {
                if (resp.error.category == ApiErrorCategory::Business && !resp.error.rawMessage.isEmpty() && resp.error.rawMessage.contains("download_url")) {
                    QJsonDocument doc = QJsonDocument::fromJson(resp.error.rawMessage.toUtf8());
                    QJsonObject obj = doc.object();
                    emit safeThis->onVersionRejected(resp.error.detail, obj["download_url"].toString(), obj["sha256"].toString());
                } else if (resp.error.category == ApiErrorCategory::Business || resp.error.status == 401 || resp.error.status == 403) {
                    safeThis->deactivateSessionOnServer(fp);
                    safeThis->resetToInactive("Heartbeat rejected: " + resp.error.detail);
                } else {
                    safeThis->m_heartbeatFailCount++;
                    emit safeThis->onHeartbeatFailed(resp.error, safeThis->m_heartbeatFailCount);
                    if (safeThis->m_heartbeatFailCount >= 3) {
                        safeThis->m_heartbeatFailCount = 0;
                        safeThis->setState(SessionState::HeartbeatFailed);
                        // Auto-reconnect will be handled by UI or we can do it here. 
                        // The user requested to "preserve the current behavior of attempting re-activation after three heartbeat failures when a saved card is available."
                        // We emit onSessionExpired with a specific reason, and MainWindow can call activate again,
                        // OR we can do it automatically if we have m_lastCardCode and fingerprint.
                        // We'll just let the UI handle the auto-reconnect or we do it here. Wait, we don't have fingerprint here.
                        // So emitting onSessionExpired or a new signal onHeartbeatMaxFailures.
                        // Let's emit onHeartbeatFailed, the MainWindow will check if failureCount == 3 and do the reactivation.
                    }
                }
            }
        }, Qt::QueuedConnection);
    });
}
