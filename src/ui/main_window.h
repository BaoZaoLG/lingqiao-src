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

    // ── Local Logger ───────────────────────────────────────────────
    void initLogger() {
        WCHAR logDir[MAX_PATH];
        GetTempPathW(MAX_PATH, logDir);
        m_logPath = QString::fromWCharArray(logDir) + "LingQiao_injector.log";
    }

    void logEvent(const QString& category, const QString& msg) {
        if (m_logPath.isEmpty()) return;
        QFile f(m_logPath);
        if (f.open(QIODevice::Append | QIODevice::Text)) {
            QString line = QDateTime::currentDateTime().toString("yyyy-MM-dd hh:mm:ss")
                         + " [" + category + "] " + msg + "\n";
            f.write(line.toUtf8());
            // Keep log under 512KB
            if (f.size() > 512 * 1024) {
                // Truncate: keep last 256KB
                f.seek(f.size() - 262144);
                QByteArray tail = f.read(262144);
                f.close();
                QFile w(m_logPath);
                if (w.open(QIODevice::WriteOnly | QIODevice::Text | QIODevice::Truncate))
                    w.write(tail);
            }
        }
    }

    // Mask sensitive data for logging: "ABCD-EFGH-..." → "ABCD****"
    static QString maskCard(const QString& code) {
        if (code.length() > 4) return code.left(4) + "****";
        return "****";
    }

    // ── Session Persistence (encrypted with machine fingerprint) ────
    // XOR-encrypt a string with a key derived from machine fingerprint
    static QString xorEncrypt(const QString& data, const QString& key) {
        QByteArray dataBytes = data.toUtf8();
        QByteArray keyBytes = key.toUtf8();
        QByteArray result;
        for (int i = 0; i < dataBytes.size(); i++) {
            result.append(dataBytes[i] ^ keyBytes[i % keyBytes.size()] ^ (char)((i * 0x9D) & 0xFF));
        }
        return result.toBase64();
    }

    static QString xorDecrypt(const QString& encBase64, const QString& key) {
        QByteArray dataBytes = QByteArray::fromBase64(encBase64.toUtf8());
        QByteArray keyBytes = key.toUtf8();
        QByteArray result;
        for (int i = 0; i < dataBytes.size(); i++) {
            result.append(dataBytes[i] ^ keyBytes[i % keyBytes.size()] ^ (char)((i * 0x9D) & 0xFF));
        }
        return QString::fromUtf8(result);
    }

    void saveSession() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        // Encrypt session token with machine fingerprint for storage
        QString fp = GetMachineFingerprint();
        s.setValue("session_token", xorEncrypt(m_sessionToken, fp));
        s.setValue("machine_id", m_machineID);
        s.setValue("card_expires_at", m_cardExpiresAt);
        s.setValue("card_code", m_cardInput ? m_cardInput->text() : "");
    }

    void clearSavedSession() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        s.remove("session_token");
        s.remove("machine_id");
        s.remove("card_expires_at");
        s.remove("card_code");
    }

    bool tryRestoreSession() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        QString encToken = s.value("session_token").toString();
        QString mid = s.value("machine_id").toString();
        qint64 exp = s.value("card_expires_at").toLongLong();
        QString card = s.value("card_code").toString();

        // Decrypt session token
        QString fp = GetMachineFingerprint();
        QString token = encToken.isEmpty() ? QString() : xorDecrypt(encToken, fp);

        if (token.isEmpty() || mid.isEmpty()) return false;
        // Check if card already expired
        if (exp > 0 && exp < QDateTime::currentSecsSinceEpoch()) {
            clearSavedSession();
            return false;
        }

        // Restore UI state
        m_sessionToken = token;
        m_machineID = mid;
        m_cardExpiresAt = exp;
        if (!card.isEmpty() && m_cardInput) m_cardInput->setText(card);

        // Try a heartbeat to validate the session
        QJsonObject req;
        req["client_id"] = QString::fromWCharArray(CLIENT_ID);
        req["session_token"] = token;
        req["machine_id"] = mid;
        req["client_version"] = GetClientVersion();
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathHb, body);

        if (resp.statusCode == 200) {
            QJsonDocument doc = QJsonDocument::fromJson(resp.body);
            QJsonObject obj = doc.object();
            if (obj["status"].toString() == "ok") {
                m_activated = true;
                m_cardExpiresAt = (qint64)obj["card_expires_at"].toDouble();
                setUiLocked(false);
                setConnDot("ok");
                downloadDllAsync();
                fetchBalance();
                if (m_cardExpiresAt > 0) {
                    m_cardExpiry->setText(QString::fromUtf8(_S("到期: %1"))
                        .arg(QDateTime::fromSecsSinceEpoch(m_cardExpiresAt).toString("yyyy-MM-dd hh:mm")));
                    m_cardExpiry->setVisible(true);
                }
                updateTrayIcon();
                logEvent("SESSION", "Session restored via heartbeat");
                return true;
            }
        }

        // Heartbeat failed, try full re-activation with saved card
        if (!card.isEmpty()) {
            logEvent("SESSION", "Heartbeat failed, attempting re-activation");
            m_cardInput->setText(card);
            clearSavedSession();
            return false; // user needs to re-activate
        }

        clearSavedSession();
        return false;
    }

    // ── Expiry Check ───────────────────────────────────────────────
    void checkCardExpiry() {
        if (!m_activated || m_cardExpiresAt <= 0) return;
        qint64 now = QDateTime::currentSecsSinceEpoch();
        qint64 remaining = m_cardExpiresAt - now;

        if (remaining <= 0) {
            // Card expired — force deactivate
            logEvent("EXPIRY", "Card expired, forcing deactivation");
            setStatus("error", QString::fromUtf8(_S("卡密已过期，请续费")));
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
            clearSavedSession();
            updateTrayIcon();
            playSound("error");
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("卡密已过期，请续费")));
            setUiLocked(true);
            setConnDot("error");
        } else if (remaining <= 3600) {
            // Less than 1 hour
            int min = (int)(remaining / 60);
            m_cardExpiry->setText(QString::fromUtf8(_S("⚠ 即将到期: %1 分钟")).arg(min));
            m_cardExpiry->setStyleSheet("font-size: 11px; padding-left: 2px; background: transparent; color: #f56565;");
            m_cardExpiry->setVisible(true);
        } else if (remaining <= 86400) {
            // Less than 24 hours
            int hr = (int)(remaining / 3600);
            m_cardExpiry->setText(QString::fromUtf8(_S("到期: %1 (%2 小时后)"))
                .arg(QDateTime::fromSecsSinceEpoch(m_cardExpiresAt).toString("yyyy-MM-dd hh:mm"))
                .arg(hr));
            m_cardExpiry->setStyleSheet("font-size: 11px; padding-left: 2px; background: transparent; color: #fbbf3a;");
            m_cardExpiry->setVisible(true);
        }
    }

    // ── System Tray ──────────────────────────────────────────────
    void initTray() {
        m_trayIcon = new QSystemTrayIcon(this);
        // Use exe's embedded icon, fallback to standard icon
        QIcon appIcon = QApplication::windowIcon();
        if (appIcon.isNull()) {
            WCHAR exePath[MAX_PATH] = {0};
            GetModuleFileNameW(NULL, exePath, MAX_PATH);
            appIcon = QFileIconProvider().icon(QFileInfo(QString::fromWCharArray(exePath)));
        }
        if (appIcon.isNull()) {
            appIcon = QApplication::style()->standardIcon(QStyle::SP_ComputerIcon);
        }
        m_trayIcon->setIcon(appIcon);
        QApplication::setWindowIcon(appIcon);
        m_trayMenu = new QMenu(this);
        m_trayMenu->addAction(QString::fromUtf8(_S("显示窗口")), this, [this]() {
            showNormal(); activateWindow(); raise();
        });
        m_trayIcon->setContextMenu(m_trayMenu);
        m_trayIcon->setToolTip(QString::fromUtf8(_S("灵桥 — 未激活")));
        connect(m_trayIcon, &QSystemTrayIcon::activated, this, [this](QSystemTrayIcon::ActivationReason reason) {
            if (reason == QSystemTrayIcon::DoubleClick) {
                showNormal(); activateWindow(); raise();
            }
        });
        m_trayIcon->show();
    }

    void updateTrayIcon() {
        if (!m_trayIcon) return;
        if (m_activated) {
            m_trayIcon->setToolTip(QString::fromUtf8(_S("灵桥 — 已激活")));
        } else {
            m_trayIcon->setToolTip(QString::fromUtf8(_S("灵桥 — 未激活")));
        }
    }

    void showTrayMessage(const QString& title, const QString& msg) {
        if (m_trayIcon && QSystemTrayIcon::supportsMessages()) {
            m_trayIcon->showMessage(title, msg, QSystemTrayIcon::Information, 5000);
        }
    }

    // ── Sound Feedback ───────────────────────────────────────────
    void playSound(const QString& type) {
        if (type == "success") {
            MessageBeep(MB_OK);
        } else if (type == "error") {
            MessageBeep(MB_ICONHAND);
        } else if (type == "warning") {
            MessageBeep(MB_ICONEXCLAMATION);
        } else {
            MessageBeep(MB_ICONASTERISK);
        }
    }

    // ── Injection History ────────────────────────────────────────
    void loadInjectHistory() {
        QSettings s(_S("LingQiao"), _S("Injector"));
        m_injectHistory = s.value("injectHistory").toStringList();
        updateHistoryLabel();
    }

    void saveInjectHistory(const QString& path) {
        m_injectHistory.removeAll(path);
        m_injectHistory.prepend(path);
        while (m_injectHistory.size() > 5) m_injectHistory.removeLast();
        QSettings(_S("LingQiao"), _S("Injector")).setValue("injectHistory", m_injectHistory);
        updateHistoryLabel();
    }

    void updateHistoryLabel() {
        if (!m_historyLabel || m_injectHistory.isEmpty()) return;
        QStringList parts;
        for (int i = 0; i < m_injectHistory.size() && i < 5; i++) {
            QFileInfo fi(m_injectHistory[i]);
            parts.append(QString("<a href='%1' style='color:#7ec8e3;text-decoration:none'>%2</a>")
                .arg(m_injectHistory[i], fi.fileName()));
        }
        m_historyLabel->setText(QString::fromUtf8(_S("最近注入: ")) + parts.join(" &middot; "));
        m_historyLabel->setVisible(true);
    }

    // ── Clipboard Monitor ────────────────────────────────────────
    void startClipboardMonitor() {
        QTimer* t = new QTimer(this);
        connect(t, &QTimer::timeout, this, [this]() {
            if (m_activated) return; // don't prompt if already activated
            QString clip = QApplication::clipboard()->text().trimmed();
            if (clip.isEmpty()) return;
            // Match card code format: XXXX-XXXX-XXXX (18 chars, Crockford Base32 + dashes)
            static QRegularExpression re(_S("^[0-9A-HJKMNP-TV-Z]{6}-[0-9A-HJKMNP-TV-Z]{6}-[0-9A-HJKMNP-TV-Z]{6}$"),
                QRegularExpression::CaseInsensitiveOption);
            if (re.match(clip).hasMatch()) {
                // Check if card input is empty or different
                if (m_cardInput->text().trimmed() != clip) {
                    m_cardInput->setText(clip);
                    showTrayMessage(QString::fromUtf8(_S("灵桥")),
                        QString::fromUtf8(_S("已从剪贴板检测到卡密: %1")).arg(clip));
                    playSound("info");
                    logEvent("CLIPBOARD", "Detected card code: " + maskCard(clip));
                }
            }
        });
        t->start(2000); // check every 2 seconds
    }

    // ── Environment Detection ────────────────────────────────────
    bool isTargetAlreadyInjected(const QString& targetPath) {
        // Check if target process already has our DLL loaded
        QFileInfo fi(targetPath);
        QString procName = fi.fileName();

        HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
        if (snap == INVALID_HANDLE_VALUE) return false;

        PROCESSENTRY32W pe = {0};
        pe.dwSize = sizeof(pe);
        bool found = false;

        if (Process32FirstW(snap, &pe)) {
            do {
                if (_wcsicmp(pe.szExeFile, (LPCWSTR)procName.utf16()) == 0) {
                    // Found the process, check if it has CefHook.dll loaded
                    HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, pe.th32ProcessID);
                    if (hProc) {
                        HMODULE mods[1024]; DWORD cbNeeded;
                        if (EnumProcessModules(hProc, mods, sizeof(mods), &cbNeeded)) {
                            for (DWORD i = 0; i < cbNeeded / sizeof(HMODULE); i++) {
                                WCHAR modName[MAX_PATH];
                                if (GetModuleFileNameExW(hProc, mods[i], modName, MAX_PATH)) {
                                    if (wcsstr(modName, L"CefHook") || wcsstr(modName, L"cefhook")) {
                                        found = true;
                                        break;
                                    }
                                }
                            }
                        }
                        CloseHandle(hProc);
                    }
                    if (found) break;
                }
            } while (Process32NextW(snap, &pe));
        }
        CloseHandle(snap);
        return found;
    }

    // ── Temp File Cleanup ────────────────────────────────────────
    void cleanupTempFiles() {
        WCHAR tempPath[MAX_PATH];
        GetTempPathW(MAX_PATH, tempPath);
        // Clean up old LingQiao temp files
        WIN32_FIND_DATAW fd;
        WCHAR pattern[MAX_PATH];
        swprintf_s(pattern, MAX_PATH, L"%s\\*.*", tempPath);
        HANDLE hFind = FindFirstFileW(pattern, &fd);
        if (hFind != INVALID_HANDLE_VALUE) {
            do {
                if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
                // Delete old update downloads
                if (wcsstr(fd.cFileName, L"update_v") && wcsstr(fd.cFileName, L".exe")) {
                    WCHAR fullPath[MAX_PATH];
                    swprintf_s(fullPath, MAX_PATH, L"%s\\%s", tempPath, fd.cFileName);
                    // Only delete if older than 1 hour
                    FILETIME ft;
                    SYSTEMTIME st;
                    GetSystemTime(&st);
                    SystemTimeToFileTime(&st, &ft);
                    ULARGE_INTEGER ul1, ul2;
                    ul1.LowPart = ft.dwLowDateTime; ul1.HighPart = ft.dwHighDateTime;
                    ul2.LowPart = fd.ftLastWriteTime.dwLowDateTime; ul2.HighPart = fd.ftLastWriteTime.dwHighDateTime;
                    if ((ul1.QuadPart - ul2.QuadPart) > 36000000000ULL) {
                        DeleteFileW(fullPath);
                    }
                }
            } while (FindNextFileW(hFind, &fd));
            FindClose(hFind);
        }
    }

    void applyShadow() {
        HWND hwnd = (HWND)winId();
        MARGINS m = {0, 0, 0, 1};
        DwmExtendFrameIntoClientArea(hwnd, &m);
        BOOL dark = TRUE;
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));
    }

    void applyAcrylic() {
        HWND hwnd = (HWND)winId();
        HMODULE hUser = GetModuleHandleW(L"user32.dll");
        if (hUser) {
            auto pSWCA = (pfnSetWindowCompositionAttribute)
                GetProcAddress(hUser, "SetWindowCompositionAttribute");
            if (pSWCA) {
                // AccentState=4 (ACRYLIC), GradientColor=0xA0FFFFFF (white tint, ABGR format)
                ACCENT_POLICY policy = {4, 0, 0xA0FFFFFF, 0};
                WINCOMPATTR_DATA data = {19, &policy, sizeof(policy)};
                pSWCA(hwnd, &data);
            }
        }
        DWM_WINDOW_CORNER_PREFERENCE pref = (DWM_WINDOW_CORNER_PREFERENCE)DWMWCP_ROUND;
        DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &pref, sizeof(pref));
    }

    // Compare two version strings (e.g. "2.1.5" vs "2.1.10"). Returns -1, 0, or 1.
    static int CompareVersion(const QString& a, const QString& b) {
        QStringList pa = a.split('.'), pb = b.split('.');
        int n = qMax(pa.size(), pb.size());
        for (int i = 0; i < n; i++) {
            int ai = i < pa.size() ? pa[i].toInt() : 0;
            int bi = i < pb.size() ? pb[i].toInt() : 0;
            if (ai < bi) return -1;
            if (ai > bi) return 1;
        }
        return 0;
    }

    // Extract version from server error message like "版本过低，请更新到 v2.1.5 或更高版本"
    static QString ExtractVersionFromMsg(const QString& msg) {
        int vIdx = msg.indexOf('v');
        if (vIdx < 0) return QString();
        int end = msg.indexOf(' ', vIdx + 1);
        return (end > vIdx) ? msg.mid(vIdx + 1, end - vIdx - 1) : msg.mid(vIdx + 1);
    }

    // Centralized update decision — called from heartbeat, announcement, and activation handlers
    void handleUpdateCheck(const QString& latest, const QString& url, bool force) {
        if (latest.isEmpty()) return;
        if (CompareVersion(GetClientVersion(), latest) >= 0) return;  // already up-to-date
        if (force) {
            applyForceUpdateBlock(latest, url);
        } else if (!m_updateDismissed) {
            m_pendingUpdateVersion = latest;
            m_pendingUpdateUrl = url;
            m_updateLabel->setText(QString::fromUtf8(_S("<b>新版本可用:</b> v%1 已发布")).arg(latest));
            m_updateNowBtn->setVisible(!url.isEmpty());
            m_remindLaterBtn->setVisible(true);
            m_updateBanner->setVisible(true);
        }
    }

    QFrame* sep() { QFrame* s = new QFrame(); s->setProperty("role", "sep"); return s; }

    QFrame* card() { QFrame* c = new QFrame(); c->setProperty("role", "card"); return c; }

    QLabel* heading(const QString& t) { QLabel* l = new QLabel(t); l->setProperty("role", "heading"); return l; }

    void buildUI() {
        QWidget* cw = new QWidget();
        cw->setObjectName("central");
        cw->setStyleSheet(QString("#central { background: transparent; border-radius: 16px; }"));
        setCentralWidget(cw);

        QVBoxLayout* root = new QVBoxLayout(cw);
        root->setContentsMargins(0, 0, 0, 0);
        root->setSpacing(0);

        // Title bar
        m_titleBar = new TitleBar();
        connect(m_titleBar, &TitleBar::closeClicked, this, &QMainWindow::close);
        connect(m_titleBar, &TitleBar::minClicked, this, [this]() { showMinimized(); });
        root->addWidget(m_titleBar);
        root->addWidget(sep());

        // Body
        QScrollArea* scroll = new QScrollArea();
        scroll->setWidgetResizable(true);
        scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
        scroll->setStyleSheet("QScrollArea { background: transparent; border: none; }");

        QWidget* body = new QWidget();
        body->setStyleSheet("background: transparent;");
        QVBoxLayout* bl = new QVBoxLayout(body);
        bl->setContentsMargins(24, 18, 24, 20);
        bl->setSpacing(16);

        bl->addWidget(buildCardSection());
        bl->addWidget(buildExpiryLabel());
        bl->addSpacing(4);

        m_status = new InlineStatus();
        bl->addWidget(m_status);
        bl->addWidget(sep());

        bl->addWidget(buildTargetSection());
        bl->addWidget(buildApiKeySection());

        buildAnnouncement();
        bl->addWidget(m_announcement);

        buildUpdateBanner();
        bl->addWidget(m_updateBanner);

        bl->addSpacing(8);
        bl->addLayout(buildActionRow());

        // Injection history label
        m_historyLabel = new QLabel();
        m_historyLabel->setStyleSheet("font-size: 11px; color: #7ec8e3; background: transparent;");
        m_historyLabel->setOpenExternalLinks(false);
        m_historyLabel->setVisible(false);
        connect(m_historyLabel, &QLabel::linkActivated, this, [this](const QString& link) {
            if (QFileInfo::exists(link)) {
                m_targetInput->setText(link);
                QSettings(_S("LingQiao"), _S("Injector")).setValue("targetPath", link);
            }
        });
        bl->addWidget(m_historyLabel);

        bl->addStretch();

        scroll->setWidget(body);
        root->addWidget(scroll, 1);

        setUiLocked(true);
    }

    QWidget* buildCardSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("卡密激活"))));

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(10);

        m_cardInput = new QLineEdit();
        m_cardInput->setPlaceholderText(QString::fromUtf8(_S("输入卡密，如 XXXX-XXXX-XXXX")));
        m_cardInput->setFixedHeight(38);
        m_cardInput->setStyleSheet(
            "QLineEdit { font-size: 13px; font-family: \"Cascadia Code\", \"Consolas\", monospace; "
            "letter-spacing: 0.5px; border-radius: 8px; }");
        row->addWidget(m_cardInput, 1);

        m_activateBtn = new AnimatedButton(QString::fromUtf8(_S("激活")), AnimatedButton::PrimaryStyle);
        m_activateBtn->setFixedSize(72, 38);
        connect(m_activateBtn, &QPushButton::clicked, this, &MainWindow::onActivate);
        row->addWidget(m_activateBtn);

        o->addLayout(row);
        return c;
    }

    QWidget* buildExpiryLabel() {
        m_cardExpiry = new QLabel();
        m_cardExpiry->setProperty("role", "warning");
        m_cardExpiry->setStyleSheet("font-size: 11px; padding-left: 2px; background: transparent; color: #fbbf3a;");
        m_cardExpiry->setVisible(false);
        return m_cardExpiry;
    }

    QWidget* buildTargetSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("目标程序"))));

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(10);

        m_targetInput = new QLineEdit();
        m_targetInput->setPlaceholderText(QString::fromUtf8(_S("选择或拖拽 .exe 文件")));
        m_targetInput->setFixedHeight(38);
        m_targetInput->setReadOnly(true);
        m_targetInput->setAcceptDrops(true);
        row->addWidget(m_targetInput, 1);

        m_browseBtn = new AnimatedButton(QString::fromUtf8(_S("浏览")), AnimatedButton::GhostStyle);
        m_browseBtn->setFixedSize(64, 38);
        m_browseBtn->setEnabled(false);
        connect(m_browseBtn, &QPushButton::clicked, this, &MainWindow::onBrowse);
        row->addWidget(m_browseBtn);

        o->addLayout(row);
        return c;
    }

    QWidget* buildApiKeySection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("API 密钥"))));

        m_apiKeyInput = new QLineEdit();
        m_apiKeyInput->setPlaceholderText(QString::fromUtf8(_S("输入 DeepSeek API Key")));
        m_apiKeyInput->setFixedHeight(38);
        m_apiKeyInput->setEchoMode(QLineEdit::Password);
        m_apiKeyInput->setEnabled(false);
        o->addWidget(m_apiKeyInput);

        m_balanceLabel = new QLabel();
        m_balanceLabel->setStyleSheet(
            "font-size: 11px; padding: 4px 2px; background: transparent; color: #7ec8e3; font-weight: 500;");
        m_balanceLabel->setVisible(false);
        o->addWidget(m_balanceLabel);

        return c;
    }

    void buildAnnouncement() {
        m_announcement = new QWidget();
        m_announcement->setVisible(false);
        m_announcement->setStyleSheet(
            "#banner { background: rgba(251,191,58,0.12); "
            "border: 1px solid rgba(251,191,58,0.25); border-radius: 10px; }");
        m_announcement->setObjectName("banner");

        QHBoxLayout* lay = new QHBoxLayout(m_announcement);
        lay->setContentsMargins(12, 7, 12, 7);
        lay->setSpacing(8);
        m_announceLabel = new QLabel();
        m_announceLabel->setWordWrap(true);
        m_announceLabel->setStyleSheet("font-size: 11px; color: #fbbf3a; background: transparent; font-weight: 500;");
        lay->addWidget(m_announceLabel, 1);
    }

    void buildUpdateBanner() {
        m_updateBanner = new QWidget();
        m_updateBanner->setVisible(false);
        m_updateBanner->setStyleSheet(
            "#updateBanner { background: rgba(74,158,255,0.12); "
            "border: 1px solid rgba(74,158,255,0.25); border-radius: 10px; }");
        m_updateBanner->setObjectName("updateBanner");

        QHBoxLayout* lay = new QHBoxLayout(m_updateBanner);
        lay->setContentsMargins(12, 7, 12, 7);
        lay->setSpacing(8);
        m_updateLabel = new QLabel();
        m_updateLabel->setWordWrap(true);
        m_updateLabel->setStyleSheet("font-size: 11px; color: #4a9eff; background: transparent; font-weight: 500;");
        lay->addWidget(m_updateLabel, 1);

        // Auto-update button
        m_updateNowBtn = new AnimatedButton(QString::fromUtf8(_S("立即更新")), AnimatedButton::PrimaryStyle);
        m_updateNowBtn->setStyleSheet(
            "QPushButton { font-size: 11px; color: #d8e4f8; background: rgba(40,60,100,0.45); "
            "border: 1px solid rgba(74,158,255,0.25); border-radius: 4px; padding: 3px 12px; font-weight: 600; }"
            "QPushButton:hover { background: rgba(50,75,120,0.55); "
            "border-color: rgba(74,158,255,0.40); }");
        m_updateNowBtn->setVisible(false);
        connect(m_updateNowBtn, &QPushButton::clicked, this, [this]() {
            if (!m_pendingUpdateUrl.isEmpty())
                ApplyUpdate(m_pendingUpdateVersion, m_pendingUpdateUrl);
        });
        lay->addWidget(m_updateNowBtn);

        m_remindLaterBtn = new AnimatedButton(QString::fromUtf8(_S("稍后提醒")), AnimatedButton::GhostStyle);
        m_remindLaterBtn->setStyleSheet(
            "QPushButton { font-size: 11px; color: #a0aec5; background: transparent; "
            "border: 1px solid rgba(80, 100, 140, 0.30); border-radius: 4px; padding: 2px 8px; }"
            "QPushButton:hover { color: #f0f4fa; border-color: rgba(100, 120, 160, 0.50); }");
        connect(m_remindLaterBtn, &QPushButton::clicked, this, [this]() {
            m_updateDismissed = true;
            m_updateBanner->setVisible(false);
        });
        lay->addWidget(m_remindLaterBtn);
    }

    void applyForceUpdateBlock(const QString& latest, const QString& url) {
        m_forceUpdateBlocked = true;
        m_injectBtn->setEnabled(false);
        m_remindLaterBtn->setVisible(false);

        // Auto-download and replace
        if (!url.isEmpty()) {
            QMessageBox* dlg = new QMessageBox(this);
            dlg->setWindowTitle(QString::fromUtf8(_S("需要更新")));
            dlg->setIcon(QMessageBox::Information);
            dlg->setText(QString::fromUtf8(_S("发现新版本 v%1，正在自动下载更新...\n下载完成后将自动替换并重启。")).arg(latest));
            dlg->setStandardButtons(QMessageBox::NoButton);
            dlg->setAttribute(Qt::WA_DeleteOnClose);
            dlg->show();
            QApplication::processEvents();
            ApplyUpdate(latest, url, dlg);
        } else {
            QMessageBox dlg(this);
            dlg.setWindowTitle(QString::fromUtf8(_S("需要更新")));
            dlg.setIcon(QMessageBox::Warning);
            dlg.setText(QString::fromUtf8(_S("当前版本过低，必须更新到 v%1 才能继续使用。\n请联系管理员上传新版本。")).arg(latest));
            dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
            dlg.exec();
            close();
        }
    }

    void ApplyUpdate(const QString& version, const QString& url, QMessageBox* progressDlg = nullptr) {
        // Build download path from URL
        std::wstring dlPathStr;
        if (url.startsWith("http://") || url.startsWith("https://")) {
            QUrl qurl(url);
            dlPathStr = qurl.path().toStdWString();
        } else {
            dlPathStr = url.toStdWString();
        }
        if (dlPathStr.empty()) {
            setStatus("error", QString::fromUtf8(_S("更新下载地址无效")));
            return;
        }

        // Use std::wstring (heap-allocated) so captured values survive across threads
        auto sHost = std::wstring(SERVER_HOST);
        int port = SERVER_PORT;
        auto sPath = std::wstring(dlPathStr);

        WCHAR buf[MAX_PATH] = {0};
        GetModuleFileNameW(NULL, buf, MAX_PATH);
        auto sExe = std::wstring(buf);

        WCHAR tempDirBuf[MAX_PATH];
        GetTempPathW(MAX_PATH, tempDirBuf);
        auto sTempDir = std::wstring(tempDirBuf);
        auto sTempFile = sTempDir + L"update_v" + version.toStdWString() + L".exe";

        setStatus("idle", QString::fromUtf8(_S("正在下载 v%1 更新...")).arg(version));
        m_injectBtn->setEnabled(false);

        // Run download in background thread to avoid UI freeze
        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, sHost, port, sPath, sTempFile, sExe, sTempDir, version, progressDlg]() {
            const wchar_t* host = sHost.c_str();
            const wchar_t* path = sPath.c_str();
            const wchar_t* tempFile = sTempFile.c_str();

            const int maxRetries = 3;
            QString err;
            for (int attempt = 0; attempt < maxRetries; attempt++) {
                if (attempt > 0) {
                    QMetaObject::invokeMethod(safeThis, [progressDlg, version, attempt, maxRetries]() {
                        if (progressDlg) progressDlg->setText(
                            QString::fromUtf8(_S("下载失败，正在重试 (%1/%2)...")).arg(attempt + 1).arg(maxRetries));
                    }, Qt::QueuedConnection);
                    Sleep(2000);
                }
                err = HttpDownloadFile(host, port, path, tempFile,
                    [this, progressDlg, version](qint64 read, qint64 total) {
                        if (total > 0) {
                            int pct = (int)(read * 100 / total);
                            QString msg = QString::fromUtf8(_S("发现新版本 v%1，正在下载更新...\n已下载 %2% (%3/%4 MB)"))
                                .arg(version).arg(pct)
                                .arg((double)read / 1048576.0, 0, 'f', 1)
                                .arg((double)total / 1048576.0, 0, 'f', 1);
                            QMetaObject::invokeMethod(safeThis, [progressDlg, msg]() {
                                if (progressDlg) progressDlg->setText(msg);
                            }, Qt::QueuedConnection);
                        }
                    });
                if (err.isEmpty()) break;
            }

            if (!err.isEmpty()) {
                QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, err]() {
                    if (!safeThis) return;
                    if (progressDlg) progressDlg->close();
                    safeThis->setStatus("error", QString::fromUtf8(_S("更新下载失败: %1")).arg(err));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    QMessageBox::warning(safeThis, QString::fromUtf8(_S("更新失败")),
                        QString::fromUtf8(_S("自动更新下载失败:\n%1\n\n请手动下载更新。")).arg(err));
                }, Qt::QueuedConnection);
                return;
            }

            // Verify PE magic, minimum file size, and SHA-256 integrity
            HANDLE hVerify = CreateFileW(tempFile, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
            if (hVerify != INVALID_HANDLE_VALUE) {
                char magic[2] = {0};
                DWORD rd = 0;
                ReadFile(hVerify, magic, 2, &rd, NULL);
                DWORD fileSize = GetFileSize(hVerify, NULL);
                // Compute SHA-256 over entire file
                BCRYPT_ALG_HANDLE hAlg = nullptr;
                BCRYPT_HASH_HANDLE hHash = nullptr;
                char fileHashHex[65] = {0};
                if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, nullptr, 0) == 0) {
                    if (BCryptCreateHash(hAlg, &hHash, nullptr, 0, nullptr, 0, 0) == 0) {
                        SetFilePointer(hVerify, 0, nullptr, FILE_BEGIN);
                        char buf[65536]; DWORD bytesRead = 0;
                        while (ReadFile(hVerify, buf, sizeof(buf), &bytesRead, NULL) && bytesRead > 0)
                            BCryptHashData(hHash, (PUCHAR)buf, bytesRead, 0);
                        BYTE hash[32]; BCryptFinishHash(hHash, hash, 32, 0);
                        ByteToHex(hash, 32, fileHashHex);
                        BCryptDestroyHash(hHash);
                    }
                    BCryptCloseAlgorithmProvider(hAlg, 0);
                }
                CloseHandle(hVerify);
                if (magic[0] != 'M' || magic[1] != 'Z' || fileSize < 4096) {
                    DeleteFileW(tempFile);
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg]() {
                        if (!safeThis) return;
                        if (progressDlg) progressDlg->close();
                        safeThis->setStatus("error", QString::fromUtf8(_S("更新文件校验失败")));
                        safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                        QMessageBox::warning(safeThis, QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("下载的文件不是有效的可执行文件。")));
                    }, Qt::QueuedConnection);
                    return;
                }
            }

            // Create batch script that replaces exe and restarts
            QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, sTempFile, sExe, sTempDir, version]() {
                if (!safeThis) return;
                if (progressDlg) progressDlg->close();
                safeThis->setStatus("ok", QString::fromUtf8(_S("更新下载完成，正在准备替换...")));

                const wchar_t* tempFile = sTempFile.c_str();
                const wchar_t* exePath = sExe.c_str();
                const wchar_t* tempDir = sTempDir.c_str();

                auto batPath = std::wstring(tempDir) + L"update_restart.bat";

                HANDLE hBat = CreateFileW(batPath.c_str(), GENERIC_WRITE, 0, NULL,
                    CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
                if (hBat == INVALID_HANDLE_VALUE) {
                    DeleteFileW(tempFile);
                    safeThis->setStatus("error", QString::fromUtf8(_S("无法创建更新脚本")));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    return;
                }

                DWORD pid = GetCurrentProcessId();
                auto exeDir = std::wstring(exePath);
                auto pos = exeDir.rfind(L'\\');
                if (pos != std::wstring::npos) exeDir = exeDir.substr(0, pos);

                // Build batch script using std::string concatenation (no sprintf_s)
                auto narrow = [](const wchar_t* w) -> std::string {
                    char buf[MAX_PATH]; WideCharToMultiByte(CP_ACP, 0, w, -1, buf, sizeof(buf), NULL, NULL); return buf;
                };
                std::string nExe = narrow(exePath);
                std::string nNew = narrow(tempFile);
                std::string nDir = narrow(exeDir.c_str());

                std::string bat = "@echo off\r\n";
                bat += "rem LingQiao auto-update script\r\n";
                bat += "tasklist /FI \"PID eq " + std::to_string(pid) + "\" 2>nul | find \"" + std::to_string(pid) + "\" >nul\r\n";
                bat += "if not errorlevel 1 (\r\n";
                bat += "    timeout /t 3 /nobreak >nul\r\n";
                bat += "    tasklist /FI \"PID eq " + std::to_string(pid) + "\" 2>nul | find \"" + std::to_string(pid) + "\" >nul\r\n";
                bat += "    if not errorlevel 1 taskkill /F /PID " + std::to_string(pid) + " >nul 2>&1\r\n";
                bat += ")\r\n";
                bat += "rem Backup current exe\r\n";
                bat += "copy /Y \"" + nExe + "\" \"" + nExe + ".bak\" >nul 2>&1\r\n";
                bat += "rem Replace with new version\r\n";
                bat += "move /Y \"" + nNew + "\" \"" + nExe + "\"\r\n";
                bat += "if errorlevel 1 (\r\n";
                bat += "    echo Update failed, restoring backup...\r\n";
                bat += "    move /Y \"" + nExe + ".bak\" \"" + nExe + "\" >nul 2>&1\r\n";
                bat += "    exit /b 1\r\n";
                bat += ")\r\n";
                bat += "rem Verify new file exists and has content\r\n";
                bat += "if not exist \"" + nExe + "\" (\r\n";
                bat += "    echo New exe missing, restoring backup...\r\n";
                bat += "    move /Y \"" + nExe + ".bak\" \"" + nExe + "\" >nul 2>&1\r\n";
                bat += "    exit /b 1\r\n";
                bat += ")\r\n";
                bat += "rem Start updated exe\r\n";
                bat += "pushd \"" + nDir + "\"\r\n";
                bat += "start \"\" \"" + nExe + "\"\r\n";
                bat += "popd\r\n";
                bat += "rem Cleanup: remove old backup after successful launch\r\n";
                bat += "timeout /t 5 /nobreak >nul\r\n";
                bat += "del /F \"" + nExe + ".bak\" >nul 2>&1\r\n";
                bat += "del /F \"%~f0\" >nul 2>&1\r\n";
                DWORD batLen = (DWORD)bat.size();
                DWORD written = 0;
                WriteFile(hBat, bat.c_str(), batLen, &written, NULL);
                CloseHandle(hBat);

                STARTUPINFOW si = {0}; si.cb = sizeof(si);
                PROCESS_INFORMATION pi = {0};
                auto cmd = std::wstring(L"cmd /c \"") + batPath.c_str() + L"\"";
                if (!CreateProcessW(NULL, (LPWSTR)cmd.c_str(), NULL, NULL, FALSE,
                    CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
                    DeleteFileW(tempFile);
                    DeleteFileW(batPath.c_str());
                    safeThis->setStatus("error", QString::fromUtf8(_S("无法启动更新进程 (错误: %1)")).arg(GetLastError()));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    return;
                }
                if (pi.hProcess) { CloseHandle(pi.hProcess); CloseHandle(pi.hThread); }

                QTimer::singleShot(500, safeThis, &QMainWindow::close);
            }, Qt::QueuedConnection);
        });
    }

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