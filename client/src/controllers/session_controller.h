#pragma once

#include <QObject>
#include <QString>
#include "services/api_client.h"

enum class SessionState {
    Inactive,
    Activating,
    Active,
    HeartbeatFailed,
    Expired,
    Deactivated
};

class SessionController : public QObject {
    Q_OBJECT
public:
    explicit SessionController(ApiClient* apiClient, QObject* parent = nullptr);
    ~SessionController() override;

    void activate(const QString& cardCode, const QString& machineId, const QString& fingerprint, const QString& clientVersion);
    bool tryRestoreSession(const QString& fingerprint, const QString& clientVersion);
    void performHeartbeat(const QString& clientVersion);
    void deactivateSessionOnServer(const QString& fingerprint);
    void deactivateSpecificSessionOnServer(const QString& token, const QString& mid, const QString& fingerprint);
    void resetToInactive(const QString& reason);
    
    SessionState state() const { return m_state; }
    QString machineID() const { return m_machineID; }
    QString lastCardCode() const { return m_lastCardCode; }

signals:
    void onActivated(const QString& sessionToken, qint64 cardExpiresAt);
    void onActivationFailed(const ApiError& error);
    void onHeartbeatOk(qint64 cardExpiresAt);
    void onHeartbeatFailed(const ApiError& error, int failureCount);
    void onSessionExpired(const QString& reason);
    void onDeactivated();
    void onUpdateHint(const UpdateHint& hint);
    // Legacy support for version rejection
    void onVersionRejected(const QString& detail, const QString& downloadUrl, const QString& sha256);

private:
    void setState(SessionState state);
    void flushPendingDeactivations(const QString& fingerprint);
    void queuePendingDeactivation(const QString& token, const QString& mid, const QString& fingerprint);
    void saveSession(const QString& fingerprint);
    void clearSavedSession();
    
    static QString protectToken(const QString& token);
    static QString unprotectToken(const QString& enc, const QString& fingerprint);
    static QString xorDecrypt(const QString& encBase64, const QString& key);

    ApiClient* m_apiClient;
    SessionState m_state = SessionState::Inactive;
    
    QString m_fingerprint;
    QString m_sessionToken;
    QString m_machineID;
    qint64 m_cardExpiresAt = 0;
    QString m_lastCardCode;
    
    int m_heartbeatFailCount = 0;
    bool m_heartbeatInProgress = false;
};
