    void downloadDllAsync() {
        setStatus("info", QString::fromUtf8(_S("正在下载核心组件...")));
        m_lastDllStatus = QString::fromUtf8(_S("下载中"));
        m_injectBtn->setEnabled(false);
        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, token = m_sessionToken, machine = m_machineID,
                           card = m_cardInput ? m_cardInput->text().trimmed() : QString()]() {
            QString err;
            for (int attempt = 1; attempt <= 3; ++attempt) {
                err = DownloadDll(SERVER_HOST, SERVER_PORT,
                    (const wchar_t*)token.utf16(),
                    (const wchar_t*)machine.utf16(),
                    (const wchar_t*)card.utf16());
                if (err.isEmpty()) break;
                if (err.contains("HTTP 401") || err.contains("HTTP 403")) break;
                Sleep(800 * attempt);
            }
            QMetaObject::invokeMethod(safeThis, [safeThis, err]() {
                if (!safeThis) return;
                if (err.isEmpty()) {
                    safeThis->m_lastDllStatus = QString::fromUtf8(_S("正常"));
                    if (!safeThis->m_targetInput->text().trimmed().isEmpty()) {
                        safeThis->setStatus("ok", QString::fromUtf8(_S("就绪，可启动注入")));
                    } else {
                        safeThis->setStatus("ok", QString::fromUtf8(_S("就绪，请选择目标程序")));
                    }
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    safeThis->logEvent("DLL", "Download OK");
                } else {
                    QString readable = safeThis->readableHttpFailure(QString::fromUtf8(_S("核心组件下载")), err);
                    safeThis->m_lastDllStatus = readable;
                    safeThis->setStatus("error", readable);
                    safeThis->logEvent("DLL", "Download failed: " + readable);
                }
            });
        });
    }

    void fetchBalance() {
        const auto& provider = currentProvider();
        if (!provider.supportsBalance) {
            m_balanceLabel->setVisible(false);
            return;
        }
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
                    safeThis->m_lastServerStatus = QString::fromUtf8(_S("公告接口正常"));
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    if (parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                        safeThis->m_balanceLabel->setVisible(false);
                        return;
                    }
                    QJsonObject obj = doc.object();
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
        connect(t, &QTimer::timeout, this, [this]() {
            if (m_sessionController && m_activated && !m_sessionToken.isEmpty()) {
                m_sessionController->performHeartbeat(GetClientVersion());
            }
            fetchAnnouncement();
        });
        t->start(55000);
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
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    if (parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                        safeThis->logEvent("ANNOUNCE", "Rejected malformed announcement response");
                        return;
                    }
                    QJsonObject obj = doc.object();
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
                            safeThis->applyForceUpdateBlock(minVer, anno["download_url"].toString(), anno["sha256"].toString());
                            return;
                        }

                        // Soft push: delegate to centralized handler
                        QString latest = anno["latest_version"].toString();
                        if (latest.startsWith('v') || latest.startsWith('V'))
                            latest = latest.mid(1);
                        if (!latest.isEmpty()) {
                            safeThis->handleUpdateCheck(latest, anno["download_url"].toString(),
                                              anno["force_update"].toBool(false),
                                              anno["sha256"].toString());
                        }
                    } else safeThis->m_announcement->setVisible(false);
                } else if (resp.statusCode != 0) {
                    safeThis->m_lastServerStatus = safeThis->readableHttpFailure(QString::fromUtf8(_S("公告接口")), QString("HTTP %1").arg(resp.statusCode));
                } else if (!resp.error.isEmpty()) {
                    safeThis->m_lastServerStatus = safeThis->readableHttpFailure(QString::fromUtf8(_S("公告接口")), resp.error);
                }
            }, Qt::QueuedConnection);
            thread->quit();
        });
        connect(thread, &QThread::finished, worker, &QObject::deleteLater);
        connect(thread, &QThread::finished, thread, &QObject::deleteLater);
        thread->start();
    }
