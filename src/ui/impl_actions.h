public:
    // Called by WinMain when --reinject --target "path" is present (UAC elevation retry)
    void setTargetAndInject(const QString& targetPath) {
        if (m_targetInput) m_targetInput->setText(targetPath);
        // Trigger injection after event loop starts
        QTimer::singleShot(500, this, &MainWindow::onInject);
    }

private:
    void onActivate() {
        // If already activated, deactivate and allow re-activation
        if (m_activated) {
            m_forceUpdateBlocked = false;
            playSound("warning");
            resetToInactiveState("User deactivated", QString::fromUtf8(_S("已注销，请输入新卡密")),
                true, QString(), QString(), QStringLiteral("idle"));
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
        connect(w, &ActivateWorker::updateAvailable, this, [this](const QString& latest, const QString& url, bool force, const QString& sha256) {
            handleUpdateCheck(latest, url, force, sha256);
        });
        connect(w, &ActivateWorker::versionRejected, this, [this,thread,w](const QString& msg, const QString& dlUrl, const QString& sha256) {
            applyForceUpdateBlock(ExtractVersionFromMsg(msg), dlUrl, sha256);
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
            startChatPolling();
            CheckEnterpriseUpdateAsync();
            if (exp > 0) {
                setExpiryStatus(QString::fromUtf8(_S("到期：%1"))
                    .arg(QDateTime::fromSecsSinceEpoch(exp).toString("yyyy-MM-dd hh:mm")));
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
            m_activateBtn->setStyleSheet(primaryButtonStyle(spx(12), spx(8)));
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
            QMessageBox dlg(this);
            dlg.setWindowTitle(QString::fromUtf8(_S("检测到已注入")));
            dlg.setIcon(QMessageBox::Warning);
            dlg.setText(QString::fromUtf8(_S("目标程序似乎已经被注入过，继续注入可能导致冲突。\n是否仍要注入？")));
            dlg.setStandardButtons(QMessageBox::Yes | QMessageBox::No);
            dlg.setDefaultButton(QMessageBox::No);
            dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
            dlg.setStyleSheet(POPUP_CSS);
            if (dlg.exec() == QMessageBox::No) {
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
        const auto& provider = currentProvider();
        SetEnvironmentVariableW(L"INJECTOR_AI_PROVIDER", (LPCWSTR)provider.id.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_KEY", (LPCWSTR)apiKey.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_TEXT_MODEL", (LPCWSTR)provider.textModel.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_VISION_MODEL", (LPCWSTR)provider.visionModel.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_BASE_URL", (LPCWSTR)provider.baseUrl.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_ADAPTER", (LPCWSTR)provider.adapter.utf16());
        SetEnvironmentVariableW(L"INJECTOR_AI_SUPPORTS_VISION", provider.supportsVision ? L"1" : L"0");
        SetEnvironmentVariableW(L"INJECTOR_API_KEY", (LPCWSTR)apiKey.utf16());
        SetEnvironmentVariableW(L"INJECTOR_SESSION_TOKEN", (LPCWSTR)m_sessionToken.utf16());

        PROCESS_INFORMATION pi = {0};
        STARTUPINFOW si = {0}; si.cb = sizeof(si);
        if (!CreateProcessW((LPCWSTR)targetPath.utf16(), NULL, NULL, NULL, FALSE,
                           CREATE_SUSPENDED, NULL, NULL, &si, &pi)) {
            DWORD err = GetLastError();
            if (err == 740) {
                WCHAR exePath[MAX_PATH] = {0};
                GetModuleFileNameW(NULL, exePath, MAX_PATH);
                // Pass target path so elevated process can continue injection
                std::wstring params = std::wstring(L"--reinject --target \"") + (LPCWSTR)targetPath.utf16() + L"\"";
                SHELLEXECUTEINFOW sei = {sizeof(sei)};
                sei.lpVerb = L"runas";
                sei.lpFile = exePath;
                sei.lpParameters = params.c_str();
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
        DWORD waitResult = WaitForSingleObject(hThread, 15000);
        DWORD result = 0;
        if (waitResult == WAIT_OBJECT_0) {
            GetExitCodeThread(hThread, &result);
            VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
        } else {
            // Timeout: do NOT free remote memory — remote thread may still be reading it.
            // The memory will be reclaimed when the target process exits.
            logEvent("INJECT", QString("Inject thread wait result: %1 (timeout/error)").arg(waitResult));
        }
        CloseHandle(hThread);
        ResumeThread(pi.hThread); CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
        CleanupDll();

        if (result) {
            logEvent("INJECT", "Injection successful");
            playSound("success");
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("注入成功")));
            setStatus("ok", QString::fromUtf8(_S("注入成功 — 1.5 秒后自动退出")));
            m_injectBtn->setText(QString::fromUtf8(_S("✓  注入成功")));
            m_injectBtn->setStyleSheet(buttonStyle(ButtonSuccess, spx(14), spx(10)));
            QTimer::singleShot(1500, this, &QMainWindow::close);
        } else {
            logEvent("INJECT", "Injection failed — DLL rejected by target");
            playSound("error");
            setStatus("error", QString::fromUtf8(_S("注入失败 — DLL 加载被目标程序拒绝")));
            m_injectBtn->setEnabled(true); m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
        }
    }
