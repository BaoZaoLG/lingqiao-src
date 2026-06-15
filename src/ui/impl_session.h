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
        if (keyBytes.isEmpty()) return QString();
        QByteArray result;
        for (int i = 0; i < dataBytes.size(); i++) {
            result.append(dataBytes[i] ^ keyBytes[i % keyBytes.size()] ^ (char)((i * 0x9D) & 0xFF));
        }
        return result.toBase64();
    }

    static QString xorDecrypt(const QString& encBase64, const QString& key) {
        QByteArray dataBytes = QByteArray::fromBase64(encBase64.toUtf8());
        QByteArray keyBytes = key.toUtf8();
        if (keyBytes.isEmpty()) return QString();
        QByteArray result;
        for (int i = 0; i < dataBytes.size(); i++) {
            result.append(dataBytes[i] ^ keyBytes[i % keyBytes.size()] ^ (char)((i * 0x9D) & 0xFF));
        }
        return QString::fromUtf8(result);
    }

    void saveSession() {
        QString fp = GetMachineFingerprint();
        if (fp.isEmpty()) {
            logEvent("SESSION", "Cannot save session: machine fingerprint unavailable");
            return;
        }
        QSettings s(_S("LingQiao"), _S("Injector"));
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

    static void queuePendingDeactivation(const QString& token, const QString& mid) {
        QString fp = GetMachineFingerprint();
        if (token.isEmpty() || mid.isEmpty() || fp.isEmpty()) return;
        QSettings s(_S("LingQiao"), _S("Injector"));
        QStringList tokens = s.value("pending_deactivate_tokens").toStringList();
        QStringList machines = s.value("pending_deactivate_machines").toStringList();
        if (!tokens.contains(xorEncrypt(token, fp))) {
            tokens.append(xorEncrypt(token, fp));
            machines.append(mid);
            s.setValue("pending_deactivate_tokens", tokens);
            s.setValue("pending_deactivate_machines", machines);
        }
    }

    void flushPendingDeactivations() {
        QString fp = GetMachineFingerprint();
        if (fp.isEmpty()) return;
        QSettings s(_S("LingQiao"), _S("Injector"));
        QStringList tokens = s.value("pending_deactivate_tokens").toStringList();
        QStringList machines = s.value("pending_deactivate_machines").toStringList();
        if (tokens.isEmpty() || tokens.size() != machines.size()) {
            s.remove("pending_deactivate_tokens");
            s.remove("pending_deactivate_machines");
            return;
        }
        QtConcurrent::run([tokens, machines, fp]() {
            QStringList keepTokens;
            QStringList keepMachines;
            for (int i = 0; i < tokens.size(); ++i) {
                QString token = xorDecrypt(tokens[i], fp);
                QString mid = machines[i];
                if (token.isEmpty() || mid.isEmpty()) continue;
                QJsonObject req;
                req["client_id"] = QString::fromWCharArray(CLIENT_ID);
                req["session_token"] = token;
                req["machine_id"] = mid;
                QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
                HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathDeact, body);
                if (resp.statusCode == 0 || resp.statusCode >= 500) {
                    keepTokens.append(tokens[i]);
                    keepMachines.append(mid);
                }
            }
            QSettings out(_S("LingQiao"), _S("Injector"));
            if (keepTokens.isEmpty()) {
                out.remove("pending_deactivate_tokens");
                out.remove("pending_deactivate_machines");
            } else {
                out.setValue("pending_deactivate_tokens", keepTokens);
                out.setValue("pending_deactivate_machines", keepMachines);
            }
        });
    }

    void deactivateSessionOnServer(const QString& token, const QString& mid) {
        if (token.isEmpty() || mid.isEmpty()) return;
        QtConcurrent::run([token, mid]() {
            QJsonObject req;
            req["client_id"] = QString::fromWCharArray(CLIENT_ID);
            req["session_token"] = token;
            req["machine_id"] = mid;
            QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathDeact, body);
            if (resp.statusCode == 0 || resp.statusCode >= 500) {
                queuePendingDeactivation(token, mid);
            }
        });
    }

    void resetToInactiveState(const QString& reason, const QString& statusText,
                              bool notifyServer = false,
                              const QString& tokenOverride = QString(),
                              const QString& midOverride = QString(),
                              const QString& statusState = QStringLiteral("error")) {
        QString token = tokenOverride.isEmpty() ? m_sessionToken : tokenOverride;
        QString mid = midOverride.isEmpty() ? m_machineID : midOverride;
        if (notifyServer) deactivateSessionOnServer(token, mid);

        logEvent("SESSION", reason);
        m_sessionToken.clear();
        m_machineID.clear();
        m_cardExpiresAt = 0;
        m_heartbeatFailCount = 0;
        m_heartbeatInProgress = false;
        SetEnvironmentVariableW(L"INJECTOR_SESSION_TOKEN", NULL);
        clearSavedSession();
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
        flushPendingDeactivations();
        QSettings s(_S("LingQiao"), _S("Injector"));
        QString encToken = s.value("session_token").toString();
        QString mid = s.value("machine_id").toString();
        qint64 exp = s.value("card_expires_at").toLongLong();
        QString card = s.value("card_code").toString();

        // Decrypt session token — guard against empty fingerprint
        QString fp = GetMachineFingerprint();
        if (fp.isEmpty() && !encToken.isEmpty()) {
            logEvent("SESSION", "Cannot restore: machine fingerprint unavailable");
            return false;
        }
        QString token = encToken.isEmpty() ? QString() : xorDecrypt(encToken, fp);

        if (token.isEmpty() || mid.isEmpty()) return false;
        // Check if card already expired
        if (exp > 0 && exp < QDateTime::currentSecsSinceEpoch()) {
            resetToInactiveState("Saved session expired before restore",
                QString::fromUtf8(_S("卡密已过期，请输入新卡密")),
                true, token, mid);
            return false;
        }

        // Restore UI state
        m_sessionToken = token;
        m_machineID = mid;
        m_cardExpiresAt = exp;
        if (!card.isEmpty() && m_cardInput) m_cardInput->setText(card);

        // Async heartbeat to validate the session — don't block UI thread
        QPointer<MainWindow> safeThis(this);
        auto hbToken = token;
        auto hbMid = mid;
        QtConcurrent::run([safeThis, hbToken, hbMid]() {
            QJsonObject req;
            req["client_id"] = QString::fromWCharArray(CLIENT_ID);
            req["session_token"] = hbToken;
            req["machine_id"] = hbMid;
            req["client_version"] = GetClientVersion();
            QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, g_pathHb, body);

            QMetaObject::invokeMethod(safeThis, [safeThis, resp, hbToken, hbMid]() {
                if (!safeThis) return;
                if (resp.statusCode == 200) {
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    if (parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                        safeThis->logEvent("SESSION", "Rejected malformed heartbeat response during restore");
                    } else {
                    QJsonObject obj = doc.object();
                    if (obj["status"].toString() == "ok") {
                        qint64 restoredExp = (qint64)obj["card_expires_at"].toDouble();
                        if (restoredExp > 0 && restoredExp <= QDateTime::currentSecsSinceEpoch()) {
                            safeThis->resetToInactiveState("Restored heartbeat returned expired card",
                                QString::fromUtf8(_S("卡密已过期，请输入新卡密")),
                                true, hbToken, hbMid);
                            return;
                        }
                        safeThis->m_activated = true;
                        safeThis->m_cardExpiresAt = restoredExp;
                        safeThis->setUiLocked(false);
                        safeThis->setConnDot("ok");
                        safeThis->downloadDllAsync();
                        safeThis->fetchBalance();
                        safeThis->startChatPolling();
                        safeThis->CheckEnterpriseUpdateAsync();
                        if (safeThis->m_cardExpiresAt > 0) {
                            safeThis->setExpiryStatus(QString::fromUtf8(_S("到期：%1"))
                                .arg(QDateTime::fromSecsSinceEpoch(safeThis->m_cardExpiresAt).toString("yyyy-MM-dd hh:mm")));
                        }
                        safeThis->updateTrayIcon();
                        safeThis->logEvent("SESSION", "Session restored via heartbeat");
                        return;
                    }
                    }
                }
                // Heartbeat failed — clear saved session, user needs to re-activate
                safeThis->resetToInactiveState("Heartbeat failed during async restore",
                    QString::fromUtf8(_S("会话已失效，请输入新卡密")),
                    true, hbToken, hbMid);
            }, Qt::QueuedConnection);
        });
        return true; // token was valid enough to attempt restore
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
                    m_cardInput->setText(clip);
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
