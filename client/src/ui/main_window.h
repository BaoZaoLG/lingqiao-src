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
#include <QScrollBar>
#include <QStackedWidget>
#include <QLineEdit>
#include <QPushButton>
#include <QComboBox>
#include <QListView>
#include <QLabel>
#include <QFrame>
#include <QTimer>
#include <QThread>
#include <QSettings>
#include <QFileDialog>
#include <QMessageBox>
#include <QGuiApplication>
#include <QMimeData>
#include <windowsx.h>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMouseEvent>
#include <QResizeEvent>
#include <QShowEvent>
#include <QtConcurrent>
#include <QJsonObject>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonParseError>
#include <QFile>
#include <QFileInfo>
#include <QDateTime>
#include <QSystemTrayIcon>
#include <QMenu>
#include <QAction>
#include <QClipboard>
#include <QRegularExpression>
#include <QFileIconProvider>
#include <QPainter>
#include <QPainterPath>
#include <QCryptographicHash>
#include <QUrl>
#include <QTextEdit>
#include <QTextBrowser>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>

#include <windows.h>
#include <dwmapi.h>
#include <shellapi.h>
#include <tlhelp32.h>
#include <psapi.h>
#include <vector>

#include "theme.h"
#include "title_bar.h"
#include "inline_status.h"
#include "animated_button.h"
#include "update_url.h"
#include "config.h"
#include "http_client.h"
#include "machine_fp.h"
#include "dll_extractor.h"
#include "workers.h"
#include "antidebug.h"
#include "strcrypt.h"
#include "controllers/session_controller.h"
#include "services/network_service.h"
#include "injector/injection_runner.h"
#include "injector/update_package_utils.h"
#include "security/process_utils.h"
#include "window_hit_test.h"
#include "layout_scale.h"
#include "button_styles.h"
#include "themed_controls.h"
#include "no_wheel_combo.h"
#include "chat_scale_styles.h"
#include "home_scale_styles.h"

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
    static double systemUiScale() {
        // Process is DPI-aware and Qt high-Dpi scaling is disabled, so
        // physical pixels == logical pixels. Scale the default window up
        // on >96 DPI displays so the first layout matches applyUiScale().
        UINT dpi = 96;
        HDC hdc = GetDC(nullptr);
        if (hdc) {
            dpi = (UINT)GetDeviceCaps(hdc, LOGPIXELSX);
            ReleaseDC(nullptr, hdc);
        }
        if (dpi <= 96) return 1.0;
        return qBound(1.0, (double)dpi / 96.0, 1.8);
    }

    explicit MainWindow(QWidget* parent = nullptr) : QMainWindow(parent) {
        setWindowFlags(Qt::Window | Qt::FramelessWindowHint);
        setAttribute(Qt::WA_TranslucentBackground, true);
        setAutoFillBackground(false);
        const double uiScale = systemUiScale();
        const int initW = qRound(WINDOW_WIDTH * uiScale);
        const int initH = qRound(WINDOW_HEIGHT * uiScale);
        resize(initW, initH);
        setMinimumSize(WINDOW_WIDTH, WINDOW_HEIGHT);
        setAcceptDrops(true);
        setStyleSheet(CSS);
        setObjectName("MainWindow");
        setStyleSheet(styleSheet() +
            QString("#MainWindow { background: transparent; border: 1px solid rgba(200, 200, 210, 0.40); border-radius: 16px; }"));

        applyShadow();
        applyAcrylic();
        buildUI();
        applyUiScale(true);
        initLogger();
        logEvent("APP", "Application started v" + GetClientVersion());

        // Initialize SessionController
        m_sessionController = new SessionController(new ApiClient(SERVER_HOST, SERVER_PORT), this);
        m_networkService = new NetworkService(this);
        connect(m_networkService, &NetworkService::dllFinished, this, [this](const QString& err) {
            if (err.isEmpty()) {
                m_lastDllStatus = QString::fromUtf8(_S("正常"));
                if (!m_targetInput->text().trimmed().isEmpty()) {
                    setStatus("ok", QString::fromUtf8(_S("就绪，可启动注入")));
                } else {
                    setStatus("ok", QString::fromUtf8(_S("就绪，请选择目标程序")));
                }
                m_injectBtn->setEnabled(!m_forceUpdateBlocked);
                logEvent("DLL", "Download OK");
            } else {
                QString readable = readableHttpFailure(QString::fromUtf8(_S("核心组件下载")), err);
                m_lastDllStatus = readable;
                setStatus("error", readable);
                logEvent("DLL", "Download failed: " + readable);
            }
        });
        connect(m_networkService, &NetworkService::modelsFinished, this,
            [this](const QStringList& models, const QString& error) {
                m_fetchingModels = false;
                m_fetchModelsBtn->setText(QString::fromUtf8(_S("获取模型")));
                updateFetchModelsButtonState();
                if (!error.isEmpty()) {
                    setStatus("error", QString::fromUtf8(_S("获取模型失败：%1")).arg(error));
                    return;
                }
                if (models.isEmpty()) {
                    setStatus("warn", QString::fromUtf8(_S("接口返回中没有可用模型")));
                    return;
                }
                const QString selected = m_modelNameInput->currentText();
                m_modelNameInput->clear();
                m_modelNameInput->addItems(models);
                m_modelNameInput->setCurrentText(models.contains(selected) ? selected : models.first());
                saveAiConfig();
                setStatus("ok", QString::fromUtf8(_S("已获取 %1 个模型")).arg(models.size()));
            });
        connect(m_networkService, &NetworkService::announcementFinished, this,
            [this](const QString& content, const QString& minVer, const QString& latest,
                   const QString& downloadUrl, const QString& sha256, bool force,
                   const QString& error) {
                if (!error.isEmpty()) {
                    m_lastServerStatus = readableHttpFailure(QString::fromUtf8(_S("公告接口")), error);
                    return;
                }
                m_announceLabel->setText(content);
                m_announcement->setVisible(!content.isEmpty());
                if (!minVer.isEmpty() && CompareVersion(GetClientVersion(), minVer) < 0) {
                    applyForceUpdateBlock(minVer, downloadUrl, sha256);
                    return;
                }
                if (!latest.isEmpty()) {
                    handleUpdateCheck(latest, downloadUrl, force, sha256);
                }
            });
        connect(m_sessionController, &SessionController::onActivated, this, [this](const QString& token, qint64 exp) {
            m_sessionToken = token;
            m_machineID = GetMachineFingerprint();
            m_cardExpiresAt = exp;
            setStatus("ok", QString::fromUtf8(_S("激活成功")));
            setUiLocked(false);
            setConnDot("ok");
            updateTrayIcon();
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("卡密激活成功")));
            playSound("success");
            logEvent("ACTIVATE", "Success");
            downloadDllAsync();
            startChatPolling();
            CheckEnterpriseUpdateAsync();
            if (exp > 0) {
                setExpiryStatus(QString::fromUtf8(_S("到期：%1"))
                    .arg(QDateTime::fromSecsSinceEpoch(exp).toString("yyyy-MM-dd hh:mm")));
            }
        });
        connect(m_sessionController, &SessionController::onActivationFailed, this, [this](const ApiError& err) {
            setStatus("error", err.detail);
            playSound("error");
            logEvent("ACTIVATE", "Failed: " + err.detail);
            m_activateBtn->setEnabled(true);
            m_cardInput->setEnabled(true);
            m_activateBtn->setText(QString::fromUtf8(_S("激活")));
            m_activateBtn->setProperty("role", "primary");
            m_activateBtn->setStyleSheet(primaryButtonStyle(spx(12), spx(8)));
            setConnDot("error");
        });
        connect(m_sessionController, &SessionController::onHeartbeatOk, this, [this](qint64 exp) {
            setConnDot("ok");
            CheckEnterpriseUpdateAsync();
            if (exp > 0) {
                m_cardExpiresAt = exp;
                setExpiryStatus(QString::fromUtf8(_S("到期：%1"))
                    .arg(QDateTime::fromSecsSinceEpoch(exp).toString("yyyy-MM-dd hh:mm")));
            }
        });
        connect(m_sessionController, &SessionController::onHeartbeatFailed, this, [this](const ApiError& err, int failureCount) {
            setConnDot("error");
            logEvent("HEARTBEAT", QString("Failed (attempt %1): %2").arg(failureCount).arg(err.detail));
            if (failureCount >= 3) {
                logEvent("HEARTBEAT", "3 consecutive failures, attempting auto-reconnect");
                QSettings s(_S("LingQiao"), _S("Injector"));
                QString savedCard = s.value("card_code").toString();
                qint64 savedExp = s.value("card_expires_at").toLongLong();
                if (savedExp > 0 && savedExp < QDateTime::currentSecsSinceEpoch()) {
                    resetToInactiveState("Heartbeat failed and saved card is expired", QString::fromUtf8(_S("卡密已过期，请输入新卡密")), true);
                } else if (!savedCard.isEmpty()) {
                    m_sessionToken.clear();
                    m_machineID.clear();
                    m_cardExpiresAt = 0;
                    setUiLocked(true);
                    m_cardInput->setText(savedCard);
                    setStatus("idle", QString::fromUtf8(_S("连接中断，正在重新激活...")));
                    onActivate();
                } else {
                    resetToInactiveState("Heartbeat failed without saved card", QString::fromUtf8(_S("会话已失效，请输入新卡密")), true);
                }
            }
        });
        connect(m_sessionController, &SessionController::onSessionExpired, this, [this](const QString& reason) {
            resetToInactiveState(reason, QString::fromUtf8(_S("连接中断，请重新激活")), false);
        });
        connect(m_sessionController, &SessionController::onUpdateHint, this, [this](const UpdateHint& hint) {
            handleUpdateCheck(hint.latestVersion, hint.downloadUrl, hint.forceUpdate, hint.sha256);
        });
        connect(m_sessionController, &SessionController::onVersionRejected, this, [this](const QString& detail, const QString& downloadUrl, const QString& sha256) {
            applyForceUpdateBlock(ExtractVersionFromMsg(detail), downloadUrl, sha256);
        });

        // Load saved settings
        QSettings settings(_S("LingQiao"), _S("Injector"));
        loadAiConfig();

        // Restore saved target path
        QString savedTarget = settings.value(_S("targetPath")).toString();
        if (!savedTarget.isEmpty() && QFileInfo::exists(savedTarget))
            m_targetInput->setText(savedTarget);
        connect(m_modelNameInput, &QComboBox::editTextChanged, this, [this]() {
            if (!m_loadingAiConfig) saveAiConfig();
            updateFetchModelsButtonState();
        });
        connect(m_baseUrlInput, &QLineEdit::textChanged, this, [this]() {
            if (!m_loadingAiConfig) saveAiConfig();
            updateFetchModelsButtonState();
        });
        connect(m_apiKeyInput, &QLineEdit::textChanged, this, [this]() {
            if (!m_loadingAiConfig) saveAiConfig();
            updateFetchModelsButtonState();
        });

        // Periodic anti-debug check (every 3 seconds)
        QTimer* antiDbg = new QTimer(this);
        connect(antiDbg, &QTimer::timeout, this, [this]() {
            if (IsBeingDebugged()) {
                static bool logged = false;
                if (!logged) {
                    logged = true;
                    logEvent("SECURITY", "Anti-debug check triggered during runtime; continuing for stability");
                }
            }
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

        QRect screen = QGuiApplication::primaryScreen()->availableGeometry();
        move((screen.width() - width()) / 2, (screen.height() - height()) / 2);
    }

    void showEvent(QShowEvent* event) override {
        QMainWindow::showEvent(event);
        // Frameless + acrylic often paints with a stale layout on first show.
        // Re-apply scale and geometry after the native window exists.
        QTimer::singleShot(0, this, [this]() {
            applyUiScale(true);
            if (QWidget* central = centralWidget()) {
                if (central->layout()) central->layout()->activate();
            }
            if (layout()) layout()->activate();
            updateGeometry();
            update();
            QRect screen = QGuiApplication::primaryScreen()->availableGeometry();
            move((screen.width() - width()) / 2, (screen.height() - height()) / 2);
        });
    }

    ~MainWindow() {
        m_heartbeatRunning = false;
        logEvent("APP", "Application closing");
        // Session is saved internally by SessionController.
        // deactivateSessionOnServer is also called internally if needed, or we just leave it.
        CleanupDll();
        cleanupTempFiles();
    }

    void closeEvent(QCloseEvent* event) override {
        if (m_minimizeToTray) {
            event->ignore();
            hide();
        } else {
            event->accept();
        }
    }

    void changeEvent(QEvent* event) override {
        QMainWindow::changeEvent(event);
        if (event->type() == QEvent::WindowStateChange) {
            if (isMinimized() && m_trayIcon && m_trayIcon->isVisible()) {
                QTimer::singleShot(0, this, [this]() {
                    hide();
                });
            }
        }
        if (event->type() == QEvent::ActivationChange && isActiveWindow()) {
            if (m_pageStack && m_pageStack->currentIndex() == 1) {
                m_chatUnreadCount = 0;
                if (m_chatHeading) m_chatHeading->setText(QString::fromUtf8(_S("公共聊天")));
                updatePageNav();
            }
        }
    }

private:
    TitleBar*      m_titleBar      = nullptr;
    SessionController* m_sessionController = nullptr;
    NetworkService* m_networkService = nullptr;
    InlineStatus*  m_status        = nullptr;
    QStackedWidget* m_pageStack     = nullptr;
    AnimatedButton* m_homeNavBtn    = nullptr;
    AnimatedButton* m_communityNavBtn = nullptr;
    QLabel*        m_communityState = nullptr;
    QLineEdit*     m_chatNicknameInput = nullptr;
    AnimatedButton* m_chatNicknameSaveBtn = nullptr;
    QLineEdit*     m_cardInput     = nullptr;
    AnimatedButton* m_activateBtn   = nullptr;
    QLineEdit*     m_targetInput   = nullptr;
    qint64         m_cardExpiresAt = 0;        // cached card expiry (unix seconds)
    QString        m_logPath;                  // local log file path
    QSystemTrayIcon* m_trayIcon = nullptr;     // system tray
    QMenu*         m_trayMenu = nullptr;
    bool           m_minimizeToTray = true;    // minimize to tray instead of exit
    QStringList    m_injectHistory;            // recent inject targets
    QLabel*        m_historyLabel = nullptr;   // history display
    AnimatedButton* m_browseBtn     = nullptr;
    QLineEdit*     m_baseUrlInput  = nullptr;
    QComboBox*     m_modelNameInput = nullptr;
    QLineEdit*     m_apiKeyInput   = nullptr;
    AnimatedButton* m_apiKeyToggleBtn = nullptr;
    QPushButton*   m_fetchModelsBtn = nullptr;
    AnimatedButton* m_injectBtn     = nullptr;
    QWidget*       m_chatPanel     = nullptr;
    QTextBrowser*   m_chatView      = nullptr;
    QLineEdit*     m_chatInput     = nullptr;
    AnimatedButton* m_chatSendBtn  = nullptr;
    QLabel*        m_chatHeading   = nullptr;
    QTimer*        m_chatTimer     = nullptr;
    qint64         m_chatLastID    = 0;
    bool           m_chatFetchInProgress = false;
    bool           m_chatPresenceInProgress = false;
    bool           m_chatProfileInProgress = false;
    int            m_chatUnreadCount = 0;
    int            m_chatOnlineCount = 0;
    QString        m_chatAuthorID;
    QString        m_chatRetryContent;
    QJsonArray     m_localMessages;
    qint64         m_chatReplyToID = 0;
    QString        m_chatReplyAuthor;
    QString        m_chatReplyPreview;
    QWidget*       m_replyBanner   = nullptr;
    QLabel*        m_replyLabel    = nullptr;
    QWidget*       m_announcement  = nullptr;
    QLabel*        m_announceLabel = nullptr;
    QWidget*       m_updateBanner  = nullptr;
    QLabel*        m_updateLabel   = nullptr;
    AnimatedButton* m_remindLaterBtn = nullptr;
    AnimatedButton* m_updateNowBtn   = nullptr;
    AnimatedButton* m_diagnosticsBtn = nullptr;
    QString  m_pendingUpdateVersion, m_pendingUpdateUrl, m_pendingUpdateSha256, m_pendingUpdatePackageKind, m_pendingUpdateReleaseID;
    bool     m_pendingUpdateInstaller = false;
    QString  m_lastDllStatus = QString::fromUtf8(_S("未开始"));
    QString  m_lastUpdateStatus = QString::fromUtf8(_S("未检查"));
    QString  m_lastChatStatus = QString::fromUtf8(_S("未连接"));
    QString  m_lastServerStatus = QString::fromUtf8(_S("未检查"));
    double   m_uiScale = 0.0;
    bool          m_loadingAiConfig = false;
    bool          m_fetchingModels = false;

    QString  m_sessionToken, m_machineID;
    bool     m_activated        = false;
    bool     m_forceUpdateBlocked = false;
    bool     m_heartbeatRunning = false;
    bool     m_updateDismissed  = false;   // session-level: suppress update banner after "稍后提醒"
    bool     m_injectInProgress = false;
    QString  m_lastClipboardCardNotified;

    void saveAiConfig() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        s.setValue(_S("aiBaseUrl"), m_baseUrlInput->text().trimmed());
        s.setValue(_S("aiModel"), m_modelNameInput->currentText().trimmed());
        s.setValue(_S("apiKeyEnc"), DpapiProtect(m_apiKeyInput->text().trimmed().toUtf8()));
    }

    void loadAiConfig() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        m_loadingAiConfig = true;
        m_baseUrlInput->setText(s.value(_S("aiBaseUrl")).toString());
        m_modelNameInput->setCurrentText(s.value(_S("aiModel")).toString());
        QByteArray plain = DpapiUnprotect(s.value(_S("apiKeyEnc")).toString());
        m_apiKeyInput->setText(QString::fromUtf8(plain));
        m_loadingAiConfig = false;
        updateFetchModelsButtonState();
    }

    // ── Session, Logging, Tray, History ──
    #include "impl_session.h"

    // ── UI Construction ──
    #include "impl_builder.h"

    // ── Update Logic ──
    #include "impl_update.h"

    // ── Network Operations ──
    #include "impl_network.h"

    // ── Public Chat ──
    #include "impl_chat.h"

    // ── User Actions ──
    #include "impl_actions.h"


    QLayout* buildActionRow() {
        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->addStretch();

        m_injectBtn = new AnimatedButton(QString::fromUtf8(_S("▶  启动注入")), AnimatedButton::PrimaryStyle);
        m_injectBtn->setFixedSize(200, 44);
        m_injectBtn->setEnabled(false);
        m_injectBtn->setStyleSheet(primaryButtonStyle(14, 10));
        connect(m_injectBtn, &QPushButton::clicked, this, &MainWindow::onInject);
        row->addWidget(m_injectBtn);
        row->addStretch();
        return row;
    }

    void setUiLocked(bool locked) {
        m_activated = !locked;
        m_targetInput->setEnabled(!locked);
        m_browseBtn->setEnabled(!locked);
        if (m_fetchModelsBtn) m_fetchModelsBtn->setEnabled(!locked && !m_fetchingModels &&
            !m_baseUrlInput->text().trimmed().isEmpty() && !m_apiKeyInput->text().trimmed().isEmpty());
        if (m_baseUrlInput) m_baseUrlInput->setEnabled(!locked);
        if (m_modelNameInput) m_modelNameInput->setEnabled(!locked);
        m_apiKeyInput->setEnabled(!locked);
        m_injectBtn->setEnabled(!locked && g_dllReady && !m_forceUpdateBlocked);
        if (m_chatInput) m_chatInput->setEnabled(!locked);
        if (m_chatSendBtn) m_chatSendBtn->setEnabled(!locked);
        if (m_chatNicknameInput) m_chatNicknameInput->setEnabled(!locked);
        if (m_chatNicknameSaveBtn) m_chatNicknameSaveBtn->setEnabled(!locked);
        updateCommunityStateText();
        m_cardInput->setEnabled(locked);
        m_activateBtn->setEnabled(locked);

        if (!locked) {
            m_activateBtn->setText(QString::fromUtf8(_S("✓")));
            m_activateBtn->setStyleSheet(buttonStyle(ButtonSuccess, spx(18), spx(6)));
        } else {
            m_activateBtn->setText(QString::fromUtf8(_S("激活")));
            m_activateBtn->setProperty("role", "primary");
            m_activateBtn->setStyleSheet(primaryButtonStyle(spx(12), spx(8)));
        }
    }

    void setStatus(const QString& state, const QString& msg) {
        if (m_status) m_status->setState(state, msg);
    }
    QString readableHttpFailure(const QString& context, const QString& err) const {
        QString text = err.trimmed();
        if (text.contains("HTTP 401")) return context + QString::fromUtf8(_S("失败：会话已失效，请重新激活"));
        if (text.contains("HTTP 403")) return context + QString::fromUtf8(_S("失败：服务器拒绝请求，可能是会话失效或更新入口不兼容"));
        if (text.contains("HTTP 404")) return context + QString::fromUtf8(_S("失败：服务器未找到对应文件"));
        if (text.contains("HTTP 429")) return context + QString::fromUtf8(_S("失败：请求过于频繁，请稍后再试"));
        if (text.contains("HTTP 5")) return context + QString::fromUtf8(_S("失败：服务器暂时不可用"));
        if (text.contains(QString::fromUtf8(_S("超时")))) return context + QString::fromUtf8(_S("失败：网络超时，请检查网络后重试"));
        if (text.contains(QString::fromUtf8(_S("证书")))) return context + QString::fromUtf8(_S("失败：服务器证书校验失败"));
        return text.isEmpty() ? context + QString::fromUtf8(_S("失败：未知错误")) : context + QString::fromUtf8(_S("失败：")) + text;
    }
    QString serverFingerprint() const {
        QString source = QString::fromWCharArray(SERVER_HOST) + ":" + QString::number(SERVER_PORT);
        QByteArray hash = QCryptographicHash::hash(source.toUtf8(), QCryptographicHash::Sha256).toHex();
        return "srv-" + QString::fromLatin1(hash.left(6));
    }
    void showDiagnostics() {
        QString text = QString::fromUtf8(_S(
            "客户端版本：%1\n"
            "服务器标识：%2\n"
            "服务器状态：%3\n"
            "激活状态：%4\n"
            "会话：%5\n"
            "核心组件：%6\n"
            "更新检查：%7\n"
            "聊天室：%8\n"
            "目标程序：%9"))
            .arg(GetClientVersion(),
                 serverFingerprint(),
                 m_lastServerStatus,
                 m_activated ? QString::fromUtf8(_S("已激活")) : QString::fromUtf8(_S("未激活")),
                 m_sessionToken.isEmpty() ? QString::fromUtf8(_S("无")) : QString::fromUtf8(_S("已建立")),
                 m_lastDllStatus,
                 m_lastUpdateStatus,
                 m_lastChatStatus,
                 m_targetInput && !m_targetInput->text().trimmed().isEmpty() ? QString::fromUtf8(_S("已选择")) : QString::fromUtf8(_S("未选择")));
        showThemedInfo(QString::fromUtf8(_S("诊断信息")), text);
    }
    void setExpiryStatus(const QString& text, const QString& tone = QStringLiteral("normal")) {
        if (m_status) m_status->setExpiryText(text, tone);
    }
    void clearExpiryStatus() {
        if (m_status) m_status->clearExpiryText();
    }
    void setConnDot(const QString& state) {
        if (m_titleBar) m_titleBar->setDot(state);
    }



    int spx(double base) const {
        return SpX(m_uiScale, base);
    }

    enum ButtonTone {
        ButtonNeutral,
        ButtonAccent,
        ButtonSuccess
    };

    QString buttonStyle(ButtonTone tone, int fontPx, int radiusPx = 8, int paddingXPx = 14, int paddingYPx = 8) const {
        UiButtonTone t = UiButtonTone::Neutral;
        if (tone == ButtonAccent) t = UiButtonTone::Accent;
        else if (tone == ButtonSuccess) t = UiButtonTone::Success;
        return BuildButtonStyle(t, fontPx, radiusPx, spx(paddingYPx), spx(paddingXPx));
    }

    QString primaryButtonStyle(int fontPx, int radiusPx = 8, int paddingXPx = 14, int paddingYPx = 8) const {
        return buttonStyle(ButtonNeutral, fontPx, radiusPx, paddingXPx, paddingYPx);
    }

    void installThemedLineEditMenu(QLineEdit* edit) {
        InstallThemedLineEditMenu(this, edit);
    }

    void installThemedComboBox(QComboBox* combo) {
        InstallThemedComboBox(combo);
        InstallThemedComboBoxMenu(this, combo);
    }

    void installThemedTextBrowserMenu(QTextBrowser* browser) {
        InstallThemedTextBrowserMenu(this, browser);
    }

    void applyUiScale(bool force = false) {
        double next = ComputeUiScale(width(), height(), WINDOW_WIDTH, WINDOW_HEIGHT);
        if (!force && qAbs(next - m_uiScale) < 0.03) return;
        m_uiScale = next;

        setStyleSheet(QString(CSS) + BuildScaledMainWindowCssheet(m_uiScale));

        if (m_cardInput) {
            m_cardInput->setFixedHeight(spx(38));
            m_cardInput->setStyleSheet(CardInputMonospaceCss(m_uiScale));
        }
        if (m_targetInput) m_targetInput->setFixedHeight(spx(38));
        if (m_modelNameInput) m_modelNameInput->setFixedHeight(spx(38));
        if (m_fetchModelsBtn) m_fetchModelsBtn->setFixedSize(spx(96), spx(38));
        if (m_baseUrlInput) m_baseUrlInput->setFixedHeight(spx(38));
        if (m_apiKeyInput) m_apiKeyInput->setFixedHeight(spx(38));
        if (m_apiKeyToggleBtn) m_apiKeyToggleBtn->setFixedSize(spx(64), spx(38));
        if (m_activateBtn) {
            m_activateBtn->setFixedSize(spx(72), spx(38));
            if (!m_activated) m_activateBtn->setStyleSheet(primaryButtonStyle(spx(12), spx(8)));
        }
        if (m_browseBtn) m_browseBtn->setFixedSize(spx(64), spx(38));
        if (m_status) m_status->setFixedHeight(spx(26));
        if (m_historyLabel) m_historyLabel->setStyleSheet(HistoryLabelCss(m_uiScale));
        if (m_announceLabel) m_announceLabel->setStyleSheet(AnnounceLabelCss(m_uiScale));
        if (m_updateLabel) m_updateLabel->setStyleSheet(UpdateLabelCss(m_uiScale));
        if (m_updateNowBtn) m_updateNowBtn->setStyleSheet(buttonStyle(ButtonAccent, spx(11), spx(4)));
        if (m_remindLaterBtn) m_remindLaterBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(4)));
        if (m_diagnosticsBtn) {
            m_diagnosticsBtn->setFixedSize(spx(64), spx(28));
            m_diagnosticsBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(6), 8, 4));
        }
        if (m_homeNavBtn) m_homeNavBtn->setFixedSize(spx(80), spx(30));
        if (m_communityNavBtn) m_communityNavBtn->setFixedSize(spx(90), spx(30));
        if (m_communityState) m_communityState->setStyleSheet(CommunityStateCss(m_uiScale));
        if (m_chatNicknameSaveBtn) {
            m_chatNicknameSaveBtn->setFixedSize(spx(48), spx(26));
            m_chatNicknameSaveBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(6), 6, 2));
        }
        updatePageNav();
        ApplyChatScaleStyles(m_uiScale, ChatWidgets{
            m_chatView, m_replyBanner, m_replyLabel, m_chatInput,
            m_chatHeading, m_chatSendBtn, m_chatNicknameInput
        });
        if (m_chatNicknameInput) {
            if (QWidget* p = m_chatNicknameInput->parentWidget()) {
                if (QLabel* nickLabel = p->findChild<QLabel*>("chatNicknameLabel")) {
                    nickLabel->setStyleSheet(QString("font-size: %1px; color: #64748b; font-weight: 500;").arg(spx(11)));
                }
            }
        }
        if (m_chatView) {
            updateChatView(m_localMessages, true);
        }
        if (m_injectBtn) {
            m_injectBtn->setFixedSize(spx(200), spx(44));
            if (!m_injectBtn->text().contains(QString::fromUtf8(_S("成功"))))
                m_injectBtn->setStyleSheet(primaryButtonStyle(spx(14), spx(10)));
        }
    }

    void resizeEvent(QResizeEvent* e) override {
        QMainWindow::resizeEvent(e);
        applyUiScale();
    }

    void paintEvent(QPaintEvent* e) override {
        QMainWindow::paintEvent(e);

        QPainter p(this);
        p.setRenderHint(QPainter::Antialiasing);
        QPainterPath path;
        path.addRoundedRect(rect().adjusted(0.5, 0.5, -0.5, -0.5), 16, 16);
        p.fillPath(path, QColor(255, 255, 255, 24));
    }

    bool nativeEvent(const QByteArray& eventType, void* message, long* result) override {
        if (eventType == "windows_generic_MSG" || eventType == "windows_dispatcher_MSG") {
            MSG* msg = static_cast<MSG*>(message);
            if (msg->message == WM_NCHITTEST) {
                QPoint globalPos(GET_X_LPARAM(msg->lParam), GET_Y_LPARAM(msg->lParam));
                QPoint localPos = mapFromGlobal(globalPos);
                LRESULT ht = FramelessHitTest(
                    localPos, rect(), isMaximized(), TITLE_BAR_H, 8,
                    [this](const QPoint& p) { return childAt(p); },
                    this);
                if (ht != 0) {
                    *result = ht;
                    return true;
                }
            }
        }
        return QMainWindow::nativeEvent(eventType, message, result);
    }

    void dragEnterEvent(QDragEnterEvent* e) override {
        if (e->mimeData()->hasUrls()) {
            QString p = e->mimeData()->urls().first().toLocalFile();
            if (p.endsWith(".exe", Qt::CaseInsensitive)) {
                e->acceptProposedAction();
                if (m_targetInput) m_targetInput->setStyleSheet(
                    "QLineEdit { background: rgba(74, 158, 255, 0.15); border: 2px dashed #4a9eff; border-radius: 8px; padding: 6px 10px; font-size: 12px; color: #1e293b; }"
                );
            }
        }
    }
    void dragLeaveEvent(QDragLeaveEvent* e) override {
        if (m_targetInput) m_targetInput->setStyleSheet("");
    }
    void dropEvent(QDropEvent* e) override {
        if (m_targetInput) m_targetInput->setStyleSheet("");
        const QMimeData* m = e->mimeData();
        if (m->hasUrls() && !m->urls().isEmpty()) {
            QString p = m->urls().first().toLocalFile();
            if (p.endsWith(".exe", Qt::CaseInsensitive)) {
                m_targetInput->setText(p);
                QSettings(_S("LingQiao"), _S("Injector")).setValue("targetPath", p);
            } else {
                setStatus("warn", QString::fromUtf8(_S("仅支持拖拽 .exe 文件")));
                playSound("warning");
            }
        }
    }
};
