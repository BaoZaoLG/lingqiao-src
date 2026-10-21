    void downloadDllAsync() {
        if (!m_networkService) return;
        setStatus("info", QString::fromUtf8(_S("正在下载核心组件...")));
        m_lastDllStatus = QString::fromUtf8(_S("下载中"));
        m_injectBtn->setEnabled(false);
        m_networkService->downloadDllAsync(
            m_sessionToken,
            m_machineID,
            m_cardInput ? m_cardInput->text().trimmed() : QString());
    }

    void updateFetchModelsButtonState() {
        if (!m_fetchModelsBtn) return;
        m_fetchModelsBtn->setEnabled(m_activated && !m_fetchingModels &&
            !m_baseUrlInput->text().trimmed().isEmpty() &&
            !m_apiKeyInput->text().trimmed().isEmpty());
    }

    void fetchModels() {
        if (!m_networkService) return;
        const QString baseUrl = m_baseUrlInput->text().trimmed();
        const QString apiKey = m_apiKeyInput->text().trimmed();
        if (baseUrl.isEmpty() || apiKey.isEmpty() || m_fetchingModels) return;

        QString modelsUrl = baseUrl;
        if (modelsUrl.endsWith(QStringLiteral("/chat/completions"), Qt::CaseInsensitive))
            modelsUrl.chop(QStringLiteral("/chat/completions").size());
        if (!modelsUrl.endsWith(QStringLiteral("/models"), Qt::CaseInsensitive))
            modelsUrl = modelsUrl + QStringLiteral("/models");

        QUrl url(modelsUrl);
        if (!url.isValid() || (url.scheme() != QStringLiteral("https") && url.scheme() != QStringLiteral("http"))) {
            setStatus("warn", QString::fromUtf8(_S("BaseURL 格式无效")));
            return;
        }
        saveAiConfig();
        m_fetchingModels = true;
        m_fetchModelsBtn->setEnabled(false);
        m_fetchModelsBtn->setText(QString::fromUtf8(_S("获取中...")));
        setStatus("idle", QString::fromUtf8(_S("正在获取模型列表...")));
        m_networkService->fetchModelsAsync(url, apiKey);
    }

    void startHeartbeatTimer() {
        m_heartbeatRunning = true;
        QTimer* t = new QTimer(this);
        connect(t, &QTimer::timeout, this, [this]() {
            if (m_sessionController && m_activated && !m_sessionToken.isEmpty()) {
                m_sessionController->performHeartbeat(GetClientVersion());
            }
            fetchAnnouncement();
        });
        t->start(55000);
    }

    void fetchAnnouncement() {
        if (m_networkService) m_networkService->fetchAnnouncementAsync();
    }
