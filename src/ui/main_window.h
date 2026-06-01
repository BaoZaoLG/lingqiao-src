#pragma once
// ============================================================================
// MainWindow
// ============================================================================
#include <QMainWindow>
#include <QApplication>
#include <QScreen>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QScrollArea>
#include <QLineEdit>
#include <QPushButton>
#include <QLabel>
#include <QFrame>
#include <QTimer>
#include <QThread>
#include <QSettings>
#include <QFileDialog>
#include <QMessageBox>
#include <QGuiApplication>
#include <QMimeData>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMouseEvent>
#include <QtConcurrent>
#include <QJsonObject>
#include <QJsonDocument>
#include <QFile>
#include <QFileInfo>
#include <QDateTime>
#include <QSystemTrayIcon>
#include <QMenu>
#include <QAction>
#include <QClipboard>
#include <QRegularExpression>
#include <QFileIconProvider>

#include <windows.h>
#include <dwmapi.h>
#include <shellapi.h>
#include <tlhelp32.h>
#include <psapi.h>

#include "theme.h"
#include "title_bar.h"
#include "inline_status.h"
#include "animated_button.h"
#include "../config.h"
#include "../http_client.h"
#include "../machine_fp.h"
#include "../dll_extractor.h"
#include "../workers.h"
#include "../antidebug.h"
#include "../strcrypt.h"

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

#ifndef DWMWA_WINDOW_CORNER_PREFERENCE
#define DWMWA_WINDOW_CORNER_PREFERENCE 33
#endif
#ifndef DWMWCP_ROUND
#define DWMWCP_ROUND 2
#endif

struct ACCENT_POLICY {
    int AccentState;
    int AccentFlags;
    int GradientColor;
    int AnimationId;
};

struct WINCOMPATTR_DATA {
    int Attrib;
    void* pvData;
    UINT cbData;
};

typedef BOOL(WINAPI* pfnSetWindowCompositionAttribute)(HWND, WINCOMPATTR_DATA*);

class MainWindow : public QMainWindow {
    Q_OBJECT
public:
    explicit MainWindow(QWidget* parent = nullptr) : QMainWindow(parent) {
        setWindowFlags(Qt::Window | Qt::FramelessWindowHint);
        setAttribute(Qt::WA_TranslucentBackground, true);
        setFixedSize(WINDOW_WIDTH, WINDOW_HEIGHT);
        setStyleSheet(CSS);
        setObjectName("MainWindow");
        setStyleSheet(styleSheet() +
            QString("#MainWindow { background: transparent; border: 1px solid rgba(200, 200, 210, 0.40); border-radius: 16px; }"));

        applyShadow();
        applyAcrylic();
        buildUI();
        initLogger();
        logEvent("APP", "Application started v" + GetClientVersion());

        // Load saved settings
        QSettings settings(_S("LingQiao"), _S("Injector"));
        // API Key: stored as DPAPI-encrypted Base64
        QString encKey = settings.value(_S("apiKeyEnc")).toString();
        if (!encKey.isEmpty()) {
            QByteArray plain = DpapiUnprotect(encKey);
            if (!plain.isEmpty()) m_apiKeyInput->setText(QString::fromUtf8(plain));
        } else {
            // Migration: read legacy plaintext key and re-encrypt
            QString legacyKey = settings.value(_S("apiKey")).toString();
            if (!legacyKey.isEmpty()) {
                m_apiKeyInput->setText(legacyKey);
                settings.setValue("apiKeyEnc", DpapiProtect(legacyKey.toUtf8()));
                settings.remove("apiKey");
            }
        }

        // Restore saved target path
        QString savedTarget = settings.value(_S("targetPath")).toString();
        if (!savedTarget.isEmpty() && QFileInfo::exists(savedTarget))
            m_targetInput->setText(savedTarget);
        connect(m_apiKeyInput, &QLineEdit::textChanged, [](const QString& text) {
            QSettings s(_S("LingQiao"), _S("Injector"));
            s.setValue("apiKeyEnc", DpapiProtect(text.toUtf8()));
        });

        // Debounced balance fetch on API key change
        QTimer* balanceTimer = new QTimer(this);
        balanceTimer->setSingleShot(true);
        balanceTimer->setInterval(800);
        connect(m_apiKeyInput, &QLineEdit::textChanged, this, [this, balanceTimer]() {
            balanceTimer->start();
        });
        connect(balanceTimer, &QTimer::timeout, this, &MainWindow::fetchBalance);

        // Periodic anti-debug check (every 3 seconds)
        QTimer* antiDbg = new QTimer(this);
        connect(antiDbg, &QTimer::timeout, this, []() {
            if (IsBeingDebugged()) ExitProcess(0);
        });
        antiDbg->start(3000);

        startHeartbeatTimer();
        fetchAnnouncement();
        initTray();
        loadInjectHistory();
        startClipboardMonitor();
        cleanupTempFiles();

        // Periodic card expiry check (every 30 seconds)
        QTimer* expiryTimer = new QTimer(this);
        connect(expiryTimer, &QTimer::timeout, this, &MainWindow::checkCardExpiry);
        expiryTimer->start(30000);

        // Try to restore previous session (silent heartbeat check)
        QTimer::singleShot(500, this, [this]() {
            if (tryRestoreSession()) {
                // Session restored successfully, skip card input
            }
        });

        // Periodic balance refresh (every 5 minutes)
        QTimer* balanceRefresh = new QTimer(this);
        connect(balanceRefresh, &QTimer::timeout, this, [this]() {
            if (m_activated) fetchBalance();
        });
        balanceRefresh->start(300000);

        QRect screen = QGuiApplication::primaryScreen()->availableGeometry();
        move((screen.width() - WINDOW_WIDTH) / 2, (screen.height() - WINDOW_HEIGHT) / 2);
    }

    ~MainWindow() {
        m_heartbeatRunning = false;
        logEvent("APP", "Application closing");
        if (m_activated) saveSession();
        CleanupDll();
        cleanupTempFiles();
    }

    void closeEvent(QCloseEvent* event) override {
        // Close button → actually exit
        m_minimizeToTray = false;
        event->accept();
    }

    void changeEvent(QEvent* event) override {
        QMainWindow::changeEvent(event);
        if (event->type() == QEvent::WindowStateChange) {
            if (isMinimized() && m_trayIcon && m_trayIcon->isVisible()) {
                QTimer::singleShot(0, this, [this]() {
                    hide();
                    showTrayMessage(QString::fromUtf8(_S("灵桥")),
                        QString::fromUtf8(_S("已最小化到托盘，双击恢复")));
                });
            }
        }
    }

private:
    TitleBar*      m_titleBar      = nullptr;
    InlineStatus*  m_status        = nullptr;
    QLineEdit*     m_cardInput     = nullptr;
    AnimatedButton* m_activateBtn   = nullptr;
    QLabel*        m_cardExpiry    = nullptr;
    QLineEdit*     m_targetInput   = nullptr;
    int            m_heartbeatFailCount = 0;   // consecutive heartbeat failures
    qint64         m_cardExpiresAt = 0;        // cached card expiry (unix seconds)
    QString        m_logPath;                  // local log file path
    QSystemTrayIcon* m_trayIcon = nullptr;     // system tray
    QMenu*         m_trayMenu = nullptr;
    bool           m_minimizeToTray = true;    // minimize to tray instead of exit
    QStringList    m_injectHistory;            // recent inject targets
    QLabel*        m_historyLabel = nullptr;   // history display
    AnimatedButton* m_browseBtn     = nullptr;
    QLineEdit*     m_apiKeyInput   = nullptr;
    QLabel*        m_balanceLabel  = nullptr;
    AnimatedButton* m_injectBtn     = nullptr;
    QWidget*       m_announcement  = nullptr;
    QLabel*        m_announceLabel = nullptr;
    QWidget*       m_updateBanner  = nullptr;
    QLabel*        m_updateLabel   = nullptr;
    AnimatedButton* m_remindLaterBtn = nullptr;
    AnimatedButton* m_updateNowBtn   = nullptr;
    QString  m_pendingUpdateVersion, m_pendingUpdateUrl;

    QString  m_sessionToken, m_machineID;
    bool     m_activated        = false;
    bool     m_forceUpdateBlocked = false;
    bool     m_heartbeatRunning = false;
    bool     m_heartbeatInProgress = false;
    bool     m_updateDismissed  = false;   // session-level: suppress update banner after "稍后提醒"
    QPoint   m_dragPos;

    // ── Session, Logging, Tray, History ──
    #include "impl_session.h"

    // ── UI Construction ──
    #include "impl_builder.h"

    // ── Update Logic ──
    #include "impl_update.h"


    QLayout* buildActionRow() {
        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->addStretch();

        m_injectBtn = new AnimatedButton(QString::fromUtf8(_S("▶  启动注入")), AnimatedButton::PrimaryStyle);
        m_injectBtn->setFixedSize(200, 44);
        m_injectBtn->setEnabled(false);
        m_injectBtn->setStyleSheet(
            "QPushButton { font-size: 14px; font-weight: 600; "
            "border-radius: 10px; letter-spacing: 1.5px; }"
            "QPushButton:hover { background: rgba(50,75,120,0.55); "
            "border-color: rgba(74,158,255,0.40); }"
            "QPushButton:pressed { background: rgba(30,45,80,0.55); "
            "border-color: rgba(74,158,255,0.20); }"
            "QPushButton:disabled { background: rgba(30,40,65,0.30); "
            "border-color: rgba(60,80,110,0.15); color: rgba(216,228,248,0.35); }");
        connect(m_injectBtn, &QPushButton::clicked, this, &MainWindow::onInject);
        row->addWidget(m_injectBtn);
        row->addStretch();
        return row;
    }

    void setUiLocked(bool locked) {
        m_activated = !locked;
        m_targetInput->setEnabled(!locked);
        m_browseBtn->setEnabled(!locked);
        m_apiKeyInput->setEnabled(!locked);
        m_injectBtn->setEnabled(!locked && g_dllReady && !m_forceUpdateBlocked);
        m_cardInput->setEnabled(locked);
        m_activateBtn->setEnabled(locked);

        if (!locked) {
            m_activateBtn->setText(QString::fromUtf8(_S("✓")));
            m_activateBtn->setStyleSheet(
                "QPushButton { background: #34d27b; border: 1px solid #34d27b; "
                "color: #ffffff; font-size: 18px; font-weight: bold; border-radius: 6px; } "
                "QPushButton:hover { background: #3ae085; border-color: #3ae085; }");
            // Pop-bounce on successful activation
            QRect base = m_activateBtn->geometry();
            int dx = 4, dy = 2;
            QRect expanded(base.x() - dx, base.y() - dy,
                           base.width() + dx * 2, base.height() + dy * 2);
            auto* seq = new QSequentialAnimationGroup(m_activateBtn);
            auto* grow = new QPropertyAnimation(m_activateBtn, "geometry", seq);
            grow->setDuration(180);
            grow->setStartValue(base);
            grow->setEndValue(expanded);
            grow->setEasingCurve(QEasingCurve::OutBack);
            auto* back = new QPropertyAnimation(m_activateBtn, "geometry", seq);
            back->setDuration(150);
            back->setStartValue(expanded);
            back->setEndValue(base);
            back->setEasingCurve(QEasingCurve::OutBounce);
            seq->addAnimation(grow);
            seq->addAnimation(back);
            seq->start(QAbstractAnimation::DeleteWhenStopped);
        } else {
            m_activateBtn->setText(QString::fromUtf8(_S("激活")));
            m_activateBtn->setProperty("role", "primary");
            m_activateBtn->setStyleSheet(
                "QPushButton { background: rgba(40,60,100,0.45); "
                "border: 1px solid rgba(74,158,255,0.25); color: #d8e4f8; "
                "font-weight: 600; letter-spacing: 0.5px; border-radius: 8px; padding: 9px 18px; }"
                "QPushButton:hover { background: rgba(50,75,120,0.55); "
                "border-color: rgba(74,158,255,0.40); }"
                "QPushButton:pressed { background: rgba(30,45,80,0.55); "
                "border-color: rgba(74,158,255,0.20); }"
                "QPushButton:disabled { background: rgba(30,40,65,0.30); "
                "border-color: rgba(60,80,110,0.15); color: rgba(216,228,248,0.35); }");
        }
    }

    void setStatus(const QString& state, const QString& msg) {
        if (m_status) m_status->setState(state, msg);
    }
    void setConnDot(const QString& state) {
        if (m_titleBar) m_titleBar->setDot(state);
    }

    void downloadDllAsync() {
        setStatus("info", QString::fromUtf8(_S("正在下载核心组件...")));
        m_injectBtn->setEnabled(false);
        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, token = m_sessionToken]() {
            QString err = DownloadDll(SERVER_HOST, SERVER_PORT,
                (const wchar_t*)token.utf16());
            QMetaObject::invokeMethod(safeThis, [safeThis, err]() {
                if (!safeThis) return;
                if (err.isEmpty()) {
                    if (!safeThis->m_targetInput->text().trimmed().isEmpty()) {
                        safeThis->setStatus("ok", QString::fromUtf8(_S("就绪 — 点击启动注入")));
                    } else {
                        safeThis->setStatus("ok", QString::fromUtf8(_S("就绪 — 请选择目标程序")));
                    }
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    safeThis->logEvent("DLL", "Download OK");
                } else {
                    safeThis->setStatus("error", err);
                    safeThis->logEvent("DLL", "Download failed: " + err);
                }
            });
        });
    }

    void fetchBalance() {
        QString key = m_apiKeyInput->text().trimmed();
        if (key.isEmpty()) {
            m_balanceLabel->setVisible(false);
            return;
        }
        m_balanceLabel->setText(QString::fromUtf8(_S("正在查询余额...")));
        m_balanceLabel->setStyleSheet(
            "font-size: 11px; padding: 4px 2px; background: transparent; color: #7ec8e3; font-weight: 500;");
        m_balanceLabel->setVisible(true);
        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, key]() {
            HttpResponse resp = HttpGetBearer(L"api.deepseek.com", L"/user/balance", key);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (resp.statusCode == 200 && !resp.body.isEmpty()) {
                    QJsonObject obj = QJsonDocument::fromJson(resp.body).object();
                    bool avail = obj["is_available"].toBool(true);
                    QJsonArray infos = obj["balance_infos"].toArray();
                    if (!infos.isEmpty()) {
                        QJsonObject info = infos[0].toObject();
                        QString total = info["total_balance"].toString();
                        QString granted = info["granted_balance"].toString();
                        QString topped = info["topped_up_balance"].toString();
                        QString currency = info["currency"].toString("CNY");
                        QString symbol = (currency == "CNY") ? "¥" : "$";
                        if (!total.isEmpty()) {
                            QString text = QString::fromUtf8(_S("余额: %1%2"))
                                .arg(symbol).arg(total);
                            if (!avail) text += QString::fromUtf8(_S(" (不可用)"));
                            safeThis->m_balanceLabel->setText(text);
                            safeThis->m_balanceLabel->setStyleSheet(QString(
                                "font-size: 11px; padding: 4px 2px; background: transparent; "
                                "color: %1; font-weight: 500;")
                                .arg(avail ? "#7ec8e3" : "#f56565"));
                            safeThis->m_balanceLabel->setVisible(true);
                        } else {
                            safeThis->m_balanceLabel->setVisible(false);
                        }
                    } else {
                        safeThis->m_balanceLabel->setVisible(false);
                    }
                } else if (resp.statusCode == 401) {
                    safeThis->m_balanceLabel->setText(QString::fromUtf8(_S("API Key 无效")));
                    safeThis->m_balanceLabel->setStyleSheet(
                        "font-size: 11px; padding: 4px 2px; background: transparent; color: #f56565; font-weight: 500;");
                    safeThis->m_balanceLabel->setVisible(true);
                } else {
                    safeThis->m_balanceLabel->setText(QString::fromUtf8(_S("余额查询失败")));
                    safeThis->m_balanceLabel->setStyleSheet(
                        "font-size: 11px; padding: 4px 2px; background: transparent; color: #f56565; font-weight: 500;");
                    safeThis->m_balanceLabel->setVisible(true);
                }
            });
        });
    }

    void startHeartbeatTimer() {
        m_heartbeatRunning = true;
        QTimer* t = new QTimer(this);
        connect(t, &QTimer::timeout, this, [this]() { performHeartbeat(); fetchAnnouncement(); });
        t->start(55000);
    }

    void performHeartbeat() {
        if (!m_activated || m_sessionToken.isEmpty() || m_heartbeatInProgress) return;
        m_heartbeatInProgress = true;
        QThread* thread = new QThread();
        HeartbeatWorker* w = new HeartbeatWorker();
        w->sessionToken  = m_sessionToken;
        w->machineID     = m_machineID;
        w->clientVersion = GetClientVersion();
        w->moveToThread(thread);
        connect(thread, &QThread::started, w, &HeartbeatWorker::process);
        connect(w, &HeartbeatWorker::heartbeatOk, this, [this,thread,w](qint64 exp) {
            m_heartbeatInProgress = false;
            m_heartbeatFailCount = 0;
            setConnDot("ok");
            if (exp > 0) {
                m_cardExpiresAt = exp;
                m_cardExpiry->setText(QString::fromUtf8(_S("到期: %1"))
                    .arg(QDateTime::fromSecsSinceEpoch(exp).toString("yyyy-MM-dd hh:mm")));
                m_cardExpiry->setVisible(true);
            }
            thread->quit(); w->deleteLater(); thread->deleteLater();
        });
        connect(w, &HeartbeatWorker::heartbeatFail, this, [this,thread,w]() {
            m_heartbeatInProgress = false;
            m_heartbeatFailCount++;
            setConnDot("error");
            logEvent("HEARTBEAT", QString("Failed (attempt %1)").arg(m_heartbeatFailCount));
            // Auto-reconnect after 3 consecutive failures
            if (m_heartbeatFailCount >= 3) {
                logEvent("HEARTBEAT", "3 consecutive failures, attempting auto-reconnect");
                m_heartbeatFailCount = 0;
                // Try to re-activate with saved card code
                QSettings s(_S("LingQiao"), _S("Injector"));
                QString savedCard = s.value("card_code").toString();
                if (!savedCard.isEmpty()) {
                    m_sessionToken.clear();
                    m_activated = false;
                    m_cardInput->setText(savedCard);
                    setStatus("idle", QString::fromUtf8(_S("连接中断，正在重新激活...")));
                    onActivate();
                }
            }
            thread->quit(); w->deleteLater(); thread->deleteLater();
        });
        connect(w, &HeartbeatWorker::updateAvailable, this, [this,thread](const QString& latest, const QString& url, bool force) {
            Q_UNUSED(thread);
            handleUpdateCheck(latest, url, force);
        }, Qt::QueuedConnection);
        connect(w, &HeartbeatWorker::versionRejected, this, [this,thread,w](const QString& msg, const QString& dlUrl) {
            m_heartbeatInProgress = false;
            applyForceUpdateBlock(ExtractVersionFromMsg(msg), dlUrl);
            thread->quit(); w->deleteLater(); thread->deleteLater();
        }, Qt::QueuedConnection);
        thread->start();
    }

    void fetchAnnouncement() {
        QThread* thread = new QThread();
        QObject* worker = new QObject();
        worker->moveToThread(thread);
        auto host = SERVER_HOST; auto port = SERVER_PORT; auto path = g_pathAnn;
        QPointer<MainWindow> safeThis(this);
        connect(thread, &QThread::started, worker, [safeThis, thread, worker, host, port, path]() {
            HttpResponse resp = WinHttpGet(host, port, path);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (resp.statusCode == 200 && !resp.body.isEmpty()) {
                    QJsonObject obj = QJsonDocument::fromJson(resp.body).object();
                    QJsonValue av = obj["announcement"];
                    if (!av.isNull() && av.isObject()) {
                        QJsonObject anno = av.toObject();
                        QString ct = anno["content"].toString();
                        safeThis->m_announceLabel->setText(ct);
                        safeThis->m_announcement->setVisible(!ct.isEmpty());

                        // Hard block: min_version check
                        QString minVer = anno["min_version"].toString();
                        if (minVer.startsWith('v') || minVer.startsWith('V'))
                            minVer = minVer.mid(1);
                        if (!minVer.isEmpty() && CompareVersion(GetClientVersion(), minVer) < 0) {
                            safeThis->applyForceUpdateBlock(minVer, anno["download_url"].toString());
                            return;
                        }

                        // Soft push: delegate to centralized handler
                        QString latest = anno["latest_version"].toString();
                        if (latest.startsWith('v') || latest.startsWith('V'))
                            latest = latest.mid(1);
                        if (!latest.isEmpty()) {
                            safeThis->handleUpdateCheck(latest, anno["download_url"].toString(),
                                              anno["force_update"].toBool(false));
                        }
                    } else safeThis->m_announcement->setVisible(false);
                }
            }, Qt::QueuedConnection);
            thread->quit();
        });
        connect(thread, &QThread::finished, worker, &QObject::deleteLater);
        connect(thread, &QThread::finished, thread, &QObject::deleteLater);
        thread->start();
    }

    void onActivate() {
        // If already activated, deactivate and allow re-activation
        if (m_activated) {
            if (!m_sessionToken.isEmpty() && !m_machineID.isEmpty()) {
                QtConcurrent::run([token = m_sessionToken, mid = m_machineID]() {
                    QJsonObject req;
                    req["client_id"] = QString::fromWCharArray(CLIENT_ID);
                    req["session_token"] = token;
                    req["machine_id"] = mid;
                    QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
                    HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathDeact, body);
                });
            }
            m_sessionToken.clear();
            m_machineID.clear();
            m_activated = false;
            m_cardExpiresAt = 0;
            m_forceUpdateBlocked = false;
            m_balanceLabel->setVisible(false);
            clearSavedSession();
            updateTrayIcon();
            playSound("warning");
            logEvent("DEACTIVATE", "User deactivated");
            setUiLocked(true);
            setStatus("idle", QString::fromUtf8(_S("已注销，请输入新卡密")));
            return;
        }
        QString code = m_cardInput->text().trimmed();
        if (code.isEmpty()) { setStatus("warn", QString::fromUtf8(_S("请先输入卡密"))); return; }
        m_activateBtn->setEnabled(false);
        m_cardInput->setEnabled(false);
        m_activateBtn->setText("...");
        setStatus("idle", QString::fromUtf8(_S("正在连接服务器验证卡密...")));

        QThread* thread = new QThread();
        ActivateWorker* w = new ActivateWorker();
        w->cardCode = code;
        w->machineID = GetMachineFingerprint();
        w->fingerprint = w->machineID;
        w->moveToThread(thread);
        connect(thread, &QThread::started, w, &ActivateWorker::process);
        connect(w, &ActivateWorker::updateAvailable, this, [this](const QString& latest, const QString& url, bool force) {
            handleUpdateCheck(latest, url, force);
        });
        connect(w, &ActivateWorker::versionRejected, this, [this,thread,w](const QString& msg, const QString& dlUrl) {
            applyForceUpdateBlock(ExtractVersionFromMsg(msg), dlUrl);
            thread->quit(); w->deleteLater(); thread->deleteLater();
        });
        connect(w, &ActivateWorker::activationSuccess, this, [this,thread,w](const QString& token, qint64 exp) {
            m_sessionToken = token;
            m_machineID = w->machineID;
            m_cardExpiresAt = exp;
            setStatus("ok", QString::fromUtf8(_S("激活成功")));
            setUiLocked(false);
            setConnDot("ok");
            saveSession();
            updateTrayIcon();
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("卡密激活成功")));
            playSound("success");
            logEvent("ACTIVATE", "Success: card=" + maskCard(w->cardCode));
            downloadDllAsync();
            fetchBalance();
            if (exp > 0) {
                m_cardExpiry->setText(QString::fromUtf8(_S("到期: %1"))
                    .arg(QDateTime::fromSecsSinceEpoch(exp).toString("yyyy-MM-dd hh:mm")));
                m_cardExpiry->setVisible(true);
            }
            thread->quit(); w->deleteLater(); thread->deleteLater();
        });
        connect(w, &ActivateWorker::activationFailed, this, [this,thread,w](const QString& err) {
            setStatus("error", err);
            playSound("error");
            logEvent("ACTIVATE", "Failed: card=" + maskCard(w->cardCode) + " err=" + err);
            m_activateBtn->setEnabled(true);
            m_cardInput->setEnabled(true);
            m_activateBtn->setText(QString::fromUtf8(_S("激活")));
            m_activateBtn->setProperty("role", "primary");
            m_activateBtn->setStyleSheet(
                "QPushButton { background: rgba(40,60,100,0.45); "
                "border: 1px solid rgba(74,158,255,0.25); color: #d8e4f8; "
                "font-weight: 600; letter-spacing: 0.5px; border-radius: 8px; padding: 9px 18px; }"
                "QPushButton:hover { background: rgba(50,75,120,0.55); "
                "border-color: rgba(74,158,255,0.40); }"
                "QPushButton:pressed { background: rgba(30,45,80,0.55); "
                "border-color: rgba(74,158,255,0.20); }"
                "QPushButton:disabled { background: rgba(30,40,65,0.30); "
                "border-color: rgba(60,80,110,0.15); color: rgba(216,228,248,0.35); }");
            setConnDot("error");
            thread->quit(); w->deleteLater(); thread->deleteLater();
        });
        thread->start();
    }

    void onBrowse() {
        QString path = QFileDialog::getOpenFileName(this,
            QString::fromUtf8(_S("选择目标程序")), QString(),
            QString::fromUtf8(_S("可执行文件 (*.exe);;所有文件 (*.*)")));
        if (!path.isEmpty()) {
            m_targetInput->setText(path);
            QSettings(_S("LingQiao"), _S("Injector")).setValue("targetPath", path);
        }
    }

    void onInject() {
        logEvent("INJECT", "Starting injection");
        if (!m_activated || m_sessionToken.isEmpty()) { setStatus("warn", QString::fromUtf8(_S("请先激活卡密"))); return; }
        if (!g_dllReady) { setStatus("error", QString::fromUtf8(_S("初始化失败：请重新启动程序"))); return; }
        QString targetPath = m_targetInput->text().trimmed();
        if (targetPath.isEmpty()) { setStatus("warn", QString::fromUtf8(_S("请先选择目标程序（支持拖拽 .exe）"))); return; }
        if (GetFileAttributesW((LPCWSTR)targetPath.utf16()) == INVALID_FILE_ATTRIBUTES) {
            setStatus("error", QString::fromUtf8(_S("目标文件不存在"))); return;
        }

        // Environment detection: warn if target already injected
        if (isTargetAlreadyInjected(targetPath)) {
            if (QMessageBox::question(this, QString::fromUtf8(_S("检测到已注入")),
                QString::fromUtf8(_S("目标程序似乎已经被注入过，继续注入可能导致冲突。\n是否仍要注入？")),
                QMessageBox::Yes | QMessageBox::No) == QMessageBox::No) {
                return;
            }
        }

        m_injectBtn->setEnabled(false);
        m_injectBtn->setText(QString::fromUtf8(_S("注入中...")));
        setStatus("idle", QString::fromUtf8(_S("正在准备注入引擎...")));
        saveInjectHistory(targetPath);

        QString apiKey = m_apiKeyInput->text().trimmed();
        if (apiKey.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请先输入 API 密钥")));
            m_injectBtn->setEnabled(true);
            m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }
        SetEnvironmentVariableW(L"INJECTOR_API_KEY", (LPCWSTR)apiKey.utf16());

        PROCESS_INFORMATION pi = {0};
        STARTUPINFOW si = {0}; si.cb = sizeof(si);
        if (!CreateProcessW((LPCWSTR)targetPath.utf16(), NULL, NULL, NULL, FALSE,
                           CREATE_SUSPENDED, NULL, NULL, &si, &pi)) {
            DWORD err = GetLastError();
            if (err == 740) {
                WCHAR exePath[MAX_PATH] = {0};
                GetModuleFileNameW(NULL, exePath, MAX_PATH);
                SHELLEXECUTEINFOW sei = {sizeof(sei)};
                sei.lpVerb = L"runas";
                sei.lpFile = exePath;
                sei.lpParameters = L"--reinject";
                sei.nShow = SW_SHOWNORMAL;
                if (ShellExecuteExW(&sei)) {
                    QTimer::singleShot(500, this, &QMainWindow::close);
                    return;
                }
            }
            logEvent("INJECT", QString("CreateProcess failed: error %1").arg(err));
            setStatus("error", QString::fromUtf8(_S("无法启动目标程序 (错误: %1)")).arg(err));
            m_injectBtn->setEnabled(true);
            m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }
        size_t cb = (wcslen(g_dllPath) + 1) * sizeof(WCHAR);
        PVOID pRemote = VirtualAllocEx(pi.hProcess, NULL, cb, MEM_COMMIT, PAGE_READWRITE);
        if (!pRemote) {
            TerminateProcess(pi.hProcess, 1); CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
            setStatus("error", QString::fromUtf8(_S("内存分配失败")));
            m_injectBtn->setEnabled(true); m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }
        if (!WriteProcessMemory(pi.hProcess, pRemote, g_dllPath, cb, NULL)) {
            VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
            TerminateProcess(pi.hProcess, 1); CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
            setStatus("error", QString::fromUtf8(_S("内存写入失败 (错误: %1)")).arg(GetLastError()));
            m_injectBtn->setEnabled(true); m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }

        // Injection: try NtCreateThreadEx first (bypasses user-mode hooks on CreateRemoteThread),
        // fall back to CreateRemoteThread if NtCreateThreadEx is unavailable.
        HMODULE hK32 = GetModuleHandleW(L"kernel32.dll");
        LPTHREAD_START_ROUTINE pLoadLib = (LPTHREAD_START_ROUTINE)GetProcAddress(hK32, "LoadLibraryW");
        HANDLE hThread = NULL;

        // Method 1: NtCreateThreadEx (undocumented, available on Vista+)
        typedef NTSTATUS(NTAPI* pNtCreateThreadEx)(
            PHANDLE, ACCESS_MASK, PVOID, HANDLE, LPTHREAD_START_ROUTINE,
            PVOID, ULONG, SIZE_T, SIZE_T, SIZE_T, PVOID);
        HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
        auto NtCreateThreadEx = hNtdll ? (pNtCreateThreadEx)
            GetProcAddress(hNtdll, "NtCreateThreadEx") : NULL;

        if (NtCreateThreadEx) {
            NTSTATUS st = NtCreateThreadEx(
                &hThread,
                THREAD_ALL_ACCESS,
                NULL,           // ObjectAttributes
                pi.hProcess,
                pLoadLib,
                pRemote,
                0,              // CreateFlags (0 = run immediately)
                0, 0, 0,        // Stack sizes (use defaults)
                NULL            // AttributeList
            );
            if (st < 0 || !hThread) {
                logEvent("INJECT", QString("NtCreateThreadEx failed: 0x%1").arg(st, 0, 16));
                hThread = NULL; // will fall through to CreateRemoteThread
            }
        }

        // Method 2: CreateRemoteThread (fallback)
        if (!hThread) {
            logEvent("INJECT", "Falling back to CreateRemoteThread");
            hThread = CreateRemoteThread(pi.hProcess, NULL, 0, pLoadLib, pRemote, 0, NULL);
        }

        if (!hThread) {
            VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
            TerminateProcess(pi.hProcess, 1); CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
            setStatus("error", QString::fromUtf8(_S("注入失败 — 目标程序可能不兼容")));
            m_injectBtn->setEnabled(true); m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }
        WaitForSingleObject(hThread, 15000);
        DWORD result = 0; GetExitCodeThread(hThread, &result); CloseHandle(hThread);
        VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
        ResumeThread(pi.hThread); CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
        CleanupDll();

        if (result) {
            logEvent("INJECT", "Injection successful");
            playSound("success");
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("注入成功")));
            setStatus("ok", QString::fromUtf8(_S("注入成功 — 1.5 秒后自动退出")));
            m_injectBtn->setText(QString::fromUtf8(_S("✓  注入成功")));
            m_injectBtn->setStyleSheet(
                "QPushButton { background: #34d27b; border: 1px solid #34d27b; "
                "color: #ffffff; font-size: 14px; font-weight: 600; border-radius: 10px; letter-spacing: 1.5px; }");
            // Pop-bounce on inject success
            QRect base = m_injectBtn->geometry();
            int dx = 6, dy = 3;
            QRect expanded(base.x() - dx, base.y() - dy,
                           base.width() + dx * 2, base.height() + dy * 2);
            auto* seq = new QSequentialAnimationGroup(m_injectBtn);
            auto* grow = new QPropertyAnimation(m_injectBtn, "geometry", seq);
            grow->setDuration(200);
            grow->setStartValue(base);
            grow->setEndValue(expanded);
            grow->setEasingCurve(QEasingCurve::OutBack);
            auto* back = new QPropertyAnimation(m_injectBtn, "geometry", seq);
            back->setDuration(160);
            back->setStartValue(expanded);
            back->setEndValue(base);
            back->setEasingCurve(QEasingCurve::OutQuad);
            seq->addAnimation(grow);
            seq->addAnimation(back);
            seq->start(QAbstractAnimation::DeleteWhenStopped);
            QTimer::singleShot(1500, this, &QMainWindow::close);
        } else {
            logEvent("INJECT", "Injection failed — DLL rejected by target");
            playSound("error");
            setStatus("error", QString::fromUtf8(_S("注入失败 — DLL 加载被目标程序拒绝")));
            m_injectBtn->setEnabled(true); m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
        }
    }

    void mousePressEvent(QMouseEvent* e) override {
        if (e->button() == Qt::LeftButton) m_dragPos = e->globalPos() - frameGeometry().topLeft();
        QMainWindow::mousePressEvent(e);
    }
    void mouseMoveEvent(QMouseEvent* e) override {
        if (e->buttons() & Qt::LeftButton) move(e->globalPos() - m_dragPos);
        QMainWindow::mouseMoveEvent(e);
    }
    void dragEnterEvent(QDragEnterEvent* e) override {
        if (e->mimeData()->hasUrls()) e->acceptProposedAction();
    }
    void dropEvent(QDropEvent* e) override {
        const QMimeData* m = e->mimeData();
        if (m->hasUrls() && !m->urls().isEmpty()) {
            QString p = m->urls().first().toLocalFile();
            if (p.endsWith(".exe", Qt::CaseInsensitive)) {
                m_targetInput->setText(p);
                QSettings(_S("LingQiao"), _S("Injector")).setValue("targetPath", p);
            }
        }
    }
};
