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

    // ── Session Persistence ────
    // Session persistence and network orchestration is now handled by SessionController.
    // The following methods bridge the UI state to the new controller.

    void resetToInactiveState(const QString& reason, const QString& statusText,
                              bool notifyServer = false,
                              const QString& tokenOverride = QString(),
                              const QString& midOverride = QString(),
                              const QString& statusState = QStringLiteral("error")) {
        if (notifyServer && m_sessionController) {
            QString token = tokenOverride.isEmpty() ? m_sessionToken : tokenOverride;
            QString mid = midOverride.isEmpty() ? m_machineID : midOverride;
            m_sessionController->deactivateSpecificSessionOnServer(token, mid, GetMachineFingerprint());
        }
        
        if (m_sessionController) {
            m_sessionController->resetToInactive(reason);
        }

        logEvent("SESSION", reason);
        m_sessionToken.clear();
        m_machineID.clear();
        m_cardExpiresAt = 0;
        m_heartbeatFailCount = 0;
        m_heartbeatInProgress = false;
        SetEnvironmentVariableW(L"INJECTOR_SESSION_TOKEN", NULL);
        if (m_cardInput) m_cardInput->clear();
        clearExpiryStatus();
        if (m_balanceLabel) m_balanceLabel->setVisible(false);
        stopChatPolling();
        setUiLocked(true);
        updateTrayIcon();
        setConnDot("error");
        if (!statusText.isEmpty()) setStatus(statusState, statusText);
    }

    bool tryRestoreSession() {
        if (!m_sessionController) return false;
        QString fp = GetMachineFingerprint();
        if (fp.isEmpty()) {
            logEvent("SESSION", "Cannot restore: machine fingerprint unavailable");
            return false;
        }
        return m_sessionController->tryRestoreSession(fp, GetClientVersion());
    }

    // ── Expiry Check ───────────────────────────────────────────────
    void checkCardExpiry() {
        if (!m_activated || m_cardExpiresAt <= 0) return;
        qint64 now = QDateTime::currentSecsSinceEpoch();
        qint64 remaining = m_cardExpiresAt - now;

        if (remaining <= 0) {
            // Card expired — force deactivate
            playSound("error");
            resetToInactiveState("Card expired, forcing deactivation",
                QString::fromUtf8(_S("卡密已过期，请输入新卡密")),
                true);
        } else if (remaining <= 3600) {
            // Less than 1 hour
            int min = (int)(remaining / 60);
            setExpiryStatus(QString::fromUtf8(_S("即将到期：%1 分钟")).arg(min), QStringLiteral("danger"));
        } else if (remaining <= 86400) {
            // Less than 24 hours
            int hr = (int)(remaining / 3600);
            setExpiryStatus(QString::fromUtf8(_S("到期：%1（%2 小时后）"))
                .arg(QDateTime::fromSecsSinceEpoch(m_cardExpiresAt).toString("yyyy-MM-dd hh:mm"))
                .arg(hr), QStringLiteral("warn"));
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
        m_trayMenu->setStyleSheet(POPUP_CSS);
        m_trayMenu->addAction(QString::fromUtf8(_S("显示窗口")), this, [this]() {
            showNormal(); activateWindow(); raise();
        });
        m_trayMenu->addSeparator();
        m_trayMenu->addAction(QString::fromUtf8(_S("退出灵桥")), this, [this]() {
            m_minimizeToTray = false;
            close();
        });
        m_trayIcon->setContextMenu(m_trayMenu);
        m_trayIcon->setToolTip(QString::fromUtf8(_S("灵桥 — 未激活")));
        connect(m_trayIcon, &QSystemTrayIcon::activated, this, [this](QSystemTrayIcon::ActivationReason reason) {
            if (reason == QSystemTrayIcon::DoubleClick || reason == QSystemTrayIcon::Trigger) {
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
            QString escapedPath = m_injectHistory[i].toHtmlEscaped();
            QString escapedName = fi.fileName().toHtmlEscaped();
            parts.append(QString("<a href='%1' style='color:#7ec8e3;text-decoration:none'>%2</a>")
                .arg(escapedPath, escapedName));
        }
        m_historyLabel->setText(QString::fromUtf8(_S("最近注入: ")) + parts.join(QString::fromUtf8(_S(" · "))));
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
                    if (m_cardInput->text().trimmed().isEmpty()) {
                        m_cardInput->setText(clip);
                        playSound("info");
                        setStatus("info", QString::fromUtf8(_S("已自动填入剪贴板中的卡密")));
                        logEvent("CLIPBOARD", "Auto-filled empty input with card code: " + maskCard(clip));
                    } else {
                        setStatus("info", QString::fromUtf8(_S("剪贴板包含卡密，可手动粘贴")));
                        logEvent("CLIPBOARD", "Detected card code in clipboard but input not empty: " + maskCard(clip));
                    }
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
