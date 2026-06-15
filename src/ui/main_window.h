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
        setAutoFillBackground(false);
        resize(WINDOW_WIDTH, WINDOW_HEIGHT);
        setMinimumSize(WINDOW_WIDTH, WINDOW_HEIGHT);
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

        // Periodic balance refresh (every 5 minutes)
        QTimer* balanceRefresh = new QTimer(this);
        connect(balanceRefresh, &QTimer::timeout, this, [this]() {
            if (m_activated) fetchBalance();
        });
        balanceRefresh->start(300000);

        QRect screen = QGuiApplication::primaryScreen()->availableGeometry();
        move((screen.width() - width()) / 2, (screen.height() - height()) / 2);
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
    InlineStatus*  m_status        = nullptr;
    QStackedWidget* m_pageStack     = nullptr;
    AnimatedButton* m_homeNavBtn    = nullptr;
    AnimatedButton* m_communityNavBtn = nullptr;
    QLabel*        m_communityState = nullptr;
    QLineEdit*     m_chatNicknameInput = nullptr;
    AnimatedButton* m_chatNicknameSaveBtn = nullptr;
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

    QString  m_sessionToken, m_machineID;
    bool     m_activated        = false;
    bool     m_forceUpdateBlocked = false;
    bool     m_heartbeatRunning = false;
    bool     m_heartbeatInProgress = false;
    bool     m_updateDismissed  = false;   // session-level: suppress update banner after "稍后提醒"

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
        if (m_cardExpiry) m_cardExpiry->setVisible(false);
    }
    void clearExpiryStatus() {
        if (m_status) m_status->clearExpiryText();
        if (m_cardExpiry) m_cardExpiry->setVisible(false);
    }
    void setConnDot(const QString& state) {
        if (m_titleBar) m_titleBar->setDot(state);
    }



    int spx(double base) const {
        double scale = m_uiScale > 0.0 ? m_uiScale : 1.0;
        return qMax(1, (int)qRound(base * scale));
    }

    enum ButtonTone {
        ButtonNeutral,
        ButtonAccent,
        ButtonSuccess
    };

    QString buttonStyle(ButtonTone tone, int fontPx, int radiusPx = 8, int paddingXPx = 14, int paddingYPx = 8) const {
        if (tone == ButtonAccent) {
            return QString(
                "QPushButton { background: rgba(74,158,255,0.85); "
                "border: 1px solid rgba(74,158,255,0.60); color: #ffffff; "
                "font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
                "border-radius: %2px; padding: %3px %4px; }"
                "QPushButton:hover { background: rgba(109,179,255,0.90); "
                "border-color: rgba(74,158,255,0.80); }"
                "QPushButton:pressed { background: rgba(59,130,246,0.90); "
                "border-color: rgba(74,158,255,0.70); }"
                "QPushButton:disabled { background: rgba(74,158,255,0.40); "
                "border-color: rgba(74,158,255,0.30); color: rgba(255,255,255,0.55); }")
                .arg(fontPx).arg(radiusPx).arg(spx(paddingYPx)).arg(spx(paddingXPx));
        }
        if (tone == ButtonSuccess) {
            return QString(
                "QPushButton { background: #34d27b; border: 1px solid #34d27b; "
                "color: #ffffff; font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
                "border-radius: %2px; padding: %3px %4px; }"
                "QPushButton:hover { background: #3ae085; border-color: #3ae085; }"
                "QPushButton:pressed { background: #22c55e; border-color: #22c55e; }"
                "QPushButton:disabled { background: rgba(52,210,123,0.40); "
                "border-color: rgba(52,210,123,0.30); color: rgba(255,255,255,0.55); }")
                .arg(fontPx).arg(radiusPx).arg(spx(paddingYPx)).arg(spx(paddingXPx));
        }
        return QString(
            "QPushButton { background: transparent; "
            "border: 1px solid rgba(200,200,210,0.50); color: #1a1a2e; "
            "font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
            "border-radius: %2px; padding: %3px %4px; }"
            "QPushButton:hover { background: rgba(255,255,255,0.38); "
            "border-color: rgba(180,180,190,0.65); }"
            "QPushButton:pressed { background: rgba(235,235,242,0.42); }"
            "QPushButton:disabled { background: rgba(255,255,255,0.14); "
            "border-color: rgba(220,220,230,0.26); color: #94a3b8; }")
            .arg(fontPx).arg(radiusPx).arg(spx(paddingYPx)).arg(spx(paddingXPx));
    }

    QString primaryButtonStyle(int fontPx, int radiusPx = 8, int paddingXPx = 14, int paddingYPx = 8) const {
        return buttonStyle(ButtonNeutral, fontPx, radiusPx, paddingXPx, paddingYPx);
    }

    void installThemedLineEditMenu(QLineEdit* edit) {
        if (!edit) return;
        edit->setContextMenuPolicy(Qt::CustomContextMenu);
        connect(edit, &QLineEdit::customContextMenuRequested, this, [edit](const QPoint& pos) {
            QMenu* menu = edit->createStandardContextMenu();
            if (!menu) return;
            menu->setAttribute(Qt::WA_DeleteOnClose);
            menu->setStyleSheet(POPUP_CSS);
            menu->popup(edit->mapToGlobal(pos));
        });
    }

    void applyUiScale(bool force = false) {
        double sx = (double)width() / (double)WINDOW_WIDTH;
        double sy = (double)height() / (double)WINDOW_HEIGHT;
        double next = qBound(1.0, qMin(sx, sy), 1.8);
        if (!force && qAbs(next - m_uiScale) < 0.03) return;
        m_uiScale = next;

        setStyleSheet(QString(CSS) + QString(
            "#MainWindow { background: transparent; border: 1px solid rgba(200, 200, 210, 0.40); border-radius: %1px; }"
            "* { font-size: %2px; }"
            "QLabel[role=\"heading\"] { font-size: %3px; font-weight: 600; color: #334155; letter-spacing: 0.6px; }"
            "QLabel[role=\"caption\"], QLabel[role=\"success\"], QLabel[role=\"danger\"], QLabel[role=\"warning\"] { font-size: %3px; }"
            "QLineEdit { font-size: %4px; border-radius: %5px; padding: %6px %7px; }"
            "QPushButton { font-size: %2px; border-radius: %5px; padding: %6px %8px; }")
            .arg(spx(16)).arg(spx(12)).arg(spx(11)).arg(spx(13))
            .arg(spx(8)).arg(spx(9)).arg(spx(12)).arg(spx(18)));

        if (m_cardInput) {
            m_cardInput->setFixedHeight(spx(38));
            m_cardInput->setStyleSheet(QString(
                "QLineEdit { font-size: %1px; font-family: \"Cascadia Code\", \"Consolas\", monospace; "
                "letter-spacing: 0.5px; border-radius: %2px; }")
                .arg(spx(13)).arg(spx(8)));
        }
        if (m_targetInput) m_targetInput->setFixedHeight(spx(38));
        if (m_apiKeyInput) m_apiKeyInput->setFixedHeight(spx(38));
        if (m_activateBtn) {
            m_activateBtn->setFixedSize(spx(72), spx(38));
            if (!m_activated) m_activateBtn->setStyleSheet(primaryButtonStyle(spx(12), spx(8)));
        }
        if (m_browseBtn) m_browseBtn->setFixedSize(spx(64), spx(38));
        if (m_status) m_status->setFixedHeight(spx(26));
        if (m_balanceLabel) {
            m_balanceLabel->setStyleSheet(QString(
                "font-size: %1px; padding: %2px %3px; background: transparent; color: #7ec8e3; font-weight: 500;")
                .arg(spx(11)).arg(spx(4)).arg(spx(2)));
        }
        if (m_historyLabel) {
            m_historyLabel->setStyleSheet(QString(
                "font-size: %1px; color: #7ec8e3; background: transparent;")
                .arg(spx(11)));
        }
        if (m_announceLabel) {
            m_announceLabel->setStyleSheet(QString(
                "font-size: %1px; color: #fbbf3a; background: transparent; font-weight: 500;")
                .arg(spx(11)));
        }
        if (m_updateLabel) {
            m_updateLabel->setStyleSheet(QString(
                "font-size: %1px; color: #4a9eff; background: transparent; font-weight: 500;")
                .arg(spx(11)));
        }
        if (m_updateNowBtn) m_updateNowBtn->setStyleSheet(buttonStyle(ButtonAccent, spx(11), spx(4)));
        if (m_remindLaterBtn) m_remindLaterBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(4)));
        if (m_diagnosticsBtn) {
            m_diagnosticsBtn->setFixedSize(spx(64), spx(28));
            m_diagnosticsBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(6), 8, 4));
        }
        if (m_homeNavBtn) m_homeNavBtn->setFixedSize(spx(80), spx(30));
        if (m_communityNavBtn) m_communityNavBtn->setFixedSize(spx(90), spx(30));
        if (m_communityState) {
            m_communityState->setStyleSheet(QString(
                "font-size: %1px; color: #475569; font-weight: 600;")
                .arg(spx(12)));
        }
        if (m_chatNicknameInput) {
            m_chatNicknameInput->setFixedSize(spx(100), spx(26));
            m_chatNicknameInput->setStyleSheet(QString(
                "QLineEdit { font-size: %1px; border-radius: %2px; padding: %3px %4px; "
                "background: rgba(255, 255, 255, 0.45); border: 1px solid rgba(200, 200, 210, 0.45); color: #0f172a; } "
                "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }")
                .arg(spx(11)).arg(spx(6)).arg(spx(2)).arg(spx(4)));
            if (QWidget* p = m_chatNicknameInput->parentWidget()) {
                if (QLabel* nickLabel = p->findChild<QLabel*>("chatNicknameLabel")) {
                    nickLabel->setStyleSheet(QString("font-size: %1px; color: #64748b; font-weight: 500;").arg(spx(11)));
                }
            }
        }
        if (m_chatNicknameSaveBtn) {
            m_chatNicknameSaveBtn->setFixedSize(spx(48), spx(26));
            m_chatNicknameSaveBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(6), 6, 2));
        }
        updatePageNav();
        if (m_chatView) {
            m_chatView->setStyleSheet(QString(
                "QTextBrowser { background: rgba(255, 255, 255, 0.35); border: 1px solid rgba(200, 200, 210, 0.40); "
                "border-radius: %1px; padding: %2px; color: #1a1a2e; font-size: %3px; }")
                .arg(spx(8)).arg(spx(10)).arg(spx(12)));
            updateChatView(m_localMessages, true);
        }
        if (m_replyBanner) {
            m_replyBanner->setStyleSheet(QString(
                "QFrame#replyBanner { background: rgba(74, 158, 255, 0.12); border: 1px solid rgba(74, 158, 255, 0.25); "
                "border-radius: %1px; padding: %2px %3px; }")
                .arg(spx(6)).arg(spx(4)).arg(spx(8)));
            if (m_replyLabel) {
                m_replyLabel->setStyleSheet(QString("font-size: %1px; color: #1e40af; font-weight: 500;").arg(spx(11)));
            }
            QPushButton* cancelBtn = m_replyBanner->findChild<QPushButton*>();
            if (cancelBtn) {
                cancelBtn->setFixedSize(spx(24), spx(24));
                cancelBtn->setStyleSheet(QString(
                    "QPushButton { border: none; background: transparent; color: #1e40af; font-weight: bold; font-size: %1px; margin: 0; padding: 0; } "
                    "QPushButton:hover { color: #ef4444; }").arg(spx(16)));
            }
        }
        if (m_chatInput) {
            m_chatInput->setFixedHeight(spx(34));
            m_chatInput->setStyleSheet(QString(
                "QLineEdit { font-size: %1px; border-radius: %2px; padding: %3px %4px; "
                "background: rgba(255, 255, 255, 0.50); border: 1px solid rgba(200, 200, 210, 0.45); color: #0f172a; } "
                "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }")
                .arg(spx(12)).arg(spx(8)).arg(spx(6)).arg(spx(10)));
        }
        if (m_chatSendBtn) {
            m_chatSendBtn->setFixedSize(spx(64), spx(34));
            m_chatSendBtn->setStyleSheet(QString(
                "QPushButton { font-size: %1px; border-radius: %2px; "
                "background: rgba(74, 158, 255, 0.85); border: 1px solid rgba(74, 158, 255, 0.60); "
                "color: #ffffff; font-weight: 600; } "
                "QPushButton:hover { background: rgba(109, 179, 255, 0.90); } "
                "QPushButton:disabled { background: rgba(74, 158, 255, 0.40); color: rgba(255,255,255,0.5); }")
                .arg(spx(12)).arg(spx(8)));
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

    bool isNonDraggableWidget(QWidget* w) const {
        if (!w) return false;
        // Walk up to find the actual interactive widget
        while (w && w != this) {
            if (qobject_cast<QPushButton*>(w)   ||
                qobject_cast<QLineEdit*>(w)     ||
                qobject_cast<QScrollBar*>(w)    ||
                qobject_cast<QComboBox*>(w)     ||
                qobject_cast<QMenu*>(w))
                return true;
            // ScrollArea viewport — allow scrolling, not dragging
            if (qobject_cast<QAbstractScrollArea*>(w))
                return true;
            // QLabel with openExternalLinks or rich text links
            if (auto* lbl = qobject_cast<QLabel*>(w)) {
                if (lbl->openExternalLinks() || lbl->textFormat() == Qt::RichText)
                    return true;
            }
            w = w->parentWidget();
        }
        return false;
    }

    bool nativeEvent(const QByteArray& eventType, void* message, long* result) override {
        if (eventType == "windows_generic_MSG" || eventType == "windows_dispatcher_MSG") {
            MSG* msg = static_cast<MSG*>(message);
            if (msg->message == WM_NCHITTEST) {
                QPoint globalPos(GET_X_LPARAM(msg->lParam), GET_Y_LPARAM(msg->lParam));
                QPoint localPos = mapFromGlobal(globalPos);

                // Bounds check — outside client rect, let system handle
                if (!rect().contains(localPos))
                    return QMainWindow::nativeEvent(eventType, message, result);

                constexpr int resizeBorder = 8;
                const bool left = localPos.x() <= resizeBorder;
                const bool right = localPos.x() >= width() - resizeBorder;
                const bool top = localPos.y() <= resizeBorder;
                const bool bottom = localPos.y() >= height() - resizeBorder;

                if (!isMaximized()) {
                    if (top && left) {
                        *result = HTTOPLEFT;
                        return true;
                    }
                    if (top && right) {
                        *result = HTTOPRIGHT;
                        return true;
                    }
                    if (bottom && left) {
                        *result = HTBOTTOMLEFT;
                        return true;
                    }
                    if (bottom && right) {
                        *result = HTBOTTOMRIGHT;
                        return true;
                    }
                    if (left) {
                        *result = HTLEFT;
                        return true;
                    }
                    if (right) {
                        *result = HTRIGHT;
                        return true;
                    }
                    if (top) {
                        *result = HTTOP;
                        return true;
                    }
                    if (bottom) {
                        *result = HTBOTTOM;
                        return true;
                    }
                }

                // Only title bar region is draggable
                if (localPos.y() < TITLE_BAR_H) {
                    QWidget* hit = childAt(localPos);
                    if (!isNonDraggableWidget(hit)) {
                        *result = HTCAPTION;
                        return true;
                    }
                }
            }
        }
        return QMainWindow::nativeEvent(eventType, message, result);
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
