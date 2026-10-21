public:
    // Called by WinMain when --reinject --target "path" is present (UAC elevation retry)
    void setTargetAndInject(const QString& targetPath) {
        if (m_targetInput) m_targetInput->setText(targetPath);
        // Trigger injection after event loop starts
        QTimer::singleShot(500, this, &MainWindow::onInject);
    }

private:
    void onActivate() {
        if (m_activated) {
            QMessageBox dlg(this);
            dlg.setWindowTitle(QString::fromUtf8(_S("确认注销")));
            dlg.setIcon(QMessageBox::Question);
            dlg.setText(QString::fromUtf8(_S("确认要注销当前设备吗？\n注销后需要重新激活才能使用。")));
            dlg.setStandardButtons(QMessageBox::Yes | QMessageBox::No);
            dlg.setDefaultButton(QMessageBox::No);
            dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
            dlg.setStyleSheet(POPUP_CSS);
            if (dlg.exec() == QMessageBox::No) {
                return;
            }
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

        if (m_sessionController) {
            m_sessionController->activate(code, GetMachineFingerprint(), GetMachineFingerprint(), GetClientVersion());
        }
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
        if (m_injectInProgress) return;
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

        QString baseUrl = m_baseUrlInput->text().trimmed();
        QString modelName = m_modelNameInput->currentText().trimmed();
        QString apiKey = m_apiKeyInput->text().trimmed();
        if (baseUrl.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请先输入 BaseUrl")));
            return;
        }
        if (modelName.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请先输入模型名称")));
            return;
        }
        if (apiKey.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请先输入 API 密钥")));
            return;
        }

        m_injectInProgress = true;
        m_injectBtn->setEnabled(false);
        m_injectBtn->setText(QString::fromUtf8(_S("注入中...")));
        setStatus("idle", QString::fromUtf8(_S("正在准备注入引擎...")));
        saveInjectHistory(targetPath);
        saveAiConfig();

        SetInjectionEnv(
            modelName, baseUrl, apiKey,
            m_sessionToken,
            m_sessionController->machineID(),
            m_sessionController->lastCardCode());

        runInjectionWorker(targetPath);
    }

    void runInjectionWorker(const QString& targetPath) {
        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, targetPath]() {
            InjectOutcome outcome = PerformInjection(targetPath);
            QMetaObject::invokeMethod(safeThis, [safeThis, outcome]() {
                if (!safeThis) return;
                safeThis->finishInjection(outcome);
            }, Qt::QueuedConnection);
        });
    }

    void finishInjection(const InjectOutcome& outcome) {
        m_injectInProgress = false;
        // Child already inherited secrets at CreateProcess; drop them from the injector.
        ClearInjectedSecrets();

        if (outcome.kind == InjectOutcome::NeedElevation) {
            WCHAR exePath[MAX_PATH] = {0};
            GetModuleFileNameW(NULL, exePath, MAX_PATH);
            setStatus("info", QString::fromUtf8(_S("正在请求管理员权限...")));
            QString targetPath = m_targetInput->text().trimmed();
            std::wstring params = std::wstring(L"--reinject --target \"") + (LPCWSTR)targetPath.utf16() + L"\"";
            SHELLEXECUTEINFOW sei = {sizeof(sei)};
            sei.lpVerb = L"runas";
            sei.lpFile = exePath;
            sei.lpParameters = params.c_str();
            sei.nShow = SW_SHOWNORMAL;
            if (ShellExecuteExW(&sei)) {
                setStatus("ok", QString::fromUtf8(_S("提权成功，请在新窗口中继续")));
                QTimer::singleShot(500, this, [this]() {
                    m_minimizeToTray = false;
                    close();
                });
                return;
            }
            setStatus("warn", QString::fromUtf8(_S("提权被取消或失败")));
            m_injectBtn->setEnabled(true);
            m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
            return;
        }

        if (outcome.kind == InjectOutcome::Success) {
            logEvent("INJECT", "Injection successful");
            playSound("success");
            showTrayMessage(QString::fromUtf8(_S("灵桥")), QString::fromUtf8(_S("注入成功")));
            setStatus("ok", QString::fromUtf8(_S("注入成功")));
            m_injectBtn->setText(QString::fromUtf8(_S("✓  注入成功")));
            m_injectBtn->setStyleSheet(buttonStyle(ButtonSuccess, spx(14), spx(10)));
            QTimer::singleShot(1500, this, [this]() {
                m_minimizeToTray = false;
                close();
            });
            return;
        }

        logEvent("INJECT", "Injection failed: " + outcome.message);
        playSound("error");
        setStatus("error", outcome.message);
        m_injectBtn->setEnabled(true);
        m_injectBtn->setText(QString::fromUtf8(_S("▶  启动注入")));
    }
