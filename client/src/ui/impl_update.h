    QMessageBox* createThemedProgressBox(const QString& title, const QString& text) {
        QMessageBox* dlg = new QMessageBox(this);
        dlg->setWindowTitle(title);
        dlg->setIcon(QMessageBox::Information);
        dlg->setText(text);
        dlg->setStandardButtons(QMessageBox::NoButton);
        dlg->setWindowFlags(dlg->windowFlags() & ~Qt::WindowContextHelpButtonHint);
        dlg->setStyleSheet(POPUP_CSS);
        dlg->setAttribute(Qt::WA_DeleteOnClose);
        return dlg;
    }

    void dismissProgressBox(QMessageBox* dlg) {
        if (!dlg) return;
        dlg->hide();
        dlg->deleteLater();
        QApplication::processEvents(QEventLoop::ExcludeUserInputEvents);
    }

    void showThemedWarning(const QString& title, const QString& text) {
        QMessageBox dlg(this);
        dlg.setWindowTitle(title);
        dlg.setIcon(QMessageBox::Warning);
        dlg.setText(text);
        dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
        dlg.setStyleSheet(POPUP_CSS);
        dlg.exec();
    }

    void showThemedInfo(const QString& title, const QString& text) {
        QMessageBox dlg(this);
        dlg.setWindowTitle(title);
        dlg.setIcon(QMessageBox::Information);
        dlg.setText(text);
        dlg.setStandardButtons(QMessageBox::Ok);
        dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
        dlg.setStyleSheet(POPUP_CSS);
        dlg.exec();
    }

    void ReportUpdateEvent(const QString& releaseID, const QString& version,
                           const QString& type, const QString& errorCode = QString(),
                           const QString& detail = QString()) {
        if (releaseID.isEmpty()) return;
        QString machine = m_machineID.isEmpty() ? GetMachineFingerprint() : m_machineID;
        QString card = m_cardInput ? m_cardInput->text().trimmed() : QString();
        QJsonObject event;
        event["release_id"] = releaseID;
        event["version"] = version;
        event["machine_id"] = machine;
        event["card_code"] = card;
        event["session_token"] = m_sessionToken;
        event["type"] = type;
        if (!errorCode.isEmpty()) event["error_code"] = errorCode;
        if (!detail.isEmpty()) event["detail"] = detail.left(300);
        QByteArray body = QJsonDocument(event).toJson(QJsonDocument::Compact);
        QtConcurrent::run([body]() {
            HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/update/events", body);
        });
    }

    void CheckEnterpriseUpdateAsync() {
        QString machine = m_machineID.isEmpty() ? GetMachineFingerprint() : m_machineID;
        if (machine.isEmpty()) return;
        QJsonObject req;
        req["client_id"] = QString::fromWCharArray(CLIENT_ID);
        req["client_version"] = GetClientVersion();
        req["channel"] = "stable";
        req["machine_id"] = machine;
        req["card"] = m_cardInput ? m_cardInput->text().trimmed() : QString();
        req["session_token"] = m_sessionToken;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);

        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/update/check", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (resp.statusCode != 200 || resp.body.isEmpty()) {
                    if (!resp.error.isEmpty()) {
                        QString readable = safeThis->readableHttpFailure(QString::fromUtf8(_S("更新检查")), resp.error);
                        safeThis->m_lastUpdateStatus = readable;
                        safeThis->logEvent("UPDATE", "Manifest check failed: " + readable);
                    } else if (resp.statusCode != 200) {
                        safeThis->m_lastUpdateStatus = safeThis->readableHttpFailure(QString::fromUtf8(_S("更新检查")), QString("HTTP %1").arg(resp.statusCode));
                    } else {
                        safeThis->m_lastUpdateStatus = QString::fromUtf8(_S("更新检查失败：响应为空"));
                    }
                    return;
                }
                safeThis->m_lastUpdateStatus = QString::fromUtf8(_S("更新检查正常"));
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) {
                    safeThis->logEvent("UPDATE", "Rejected malformed update check response");
                    return;
                }
                QJsonObject obj = doc.object();
                if (!obj["update_available"].toBool(false)) return;
                QString payloadB64 = obj["manifest_payload"].toString();
                QString manifestHmac = obj["manifest_hmac"].toString();
                QString signature = obj["signature"].toString();
                QString publicKey = obj["public_key"].toString();
                QString pinnedKey = QString::fromUtf8(UPDATE_MANIFEST_PUBLIC_KEY_HEX).trimmed();
                if (payloadB64.isEmpty() || manifestHmac.isEmpty() || signature.isEmpty() || publicKey.isEmpty()) {
                    safeThis->logEvent("UPDATE", "Rejected manifest without payload/HMAC/signature/public key");
                    return;
                }
                /* Require a build-time pin — empty pin would accept any key. */
                if (pinnedKey.isEmpty()) {
                    safeThis->logEvent("UPDATE", "Rejected manifest: UPDATE_MANIFEST_PUBLIC_KEY_HEX not pinned");
                    return;
                }
                if (publicKey.compare(pinnedKey, Qt::CaseInsensitive) != 0) {
                    safeThis->logEvent("UPDATE", "Rejected manifest with unexpected public key");
                    return;
                }
                /* Ed25519 signature is carried for server-side tooling; integrity
                 * on the wire is enforced by HMAC over manifest_payload. Require
                 * a well-formed 64-byte hex signature (Ed25519). */
                QByteArray sigBytes = QByteArray::fromHex(signature.toUtf8());
                if (sigBytes.size() != 64) {
                    safeThis->logEvent("UPDATE", "Rejected manifest: malformed Ed25519 signature");
                    return;
                }
                QByteArray payload = QByteArray::fromBase64(payloadB64.toUtf8());
                if (payload.isEmpty()) {
                    safeThis->logEvent("UPDATE", "Rejected empty manifest payload");
                    return;
                }
                BYTE mac[32]; DWORD macLen = 0;
                if (!HmacSha256((const char*)HMAC_KEY, 32, payload.constData(), (DWORD)payload.size(), mac, &macLen)) {
                    safeThis->logEvent("UPDATE", "Rejected manifest: local HMAC failed");
                    return;
                }
                char macHex[65]; ByteToHex(mac, macLen, macHex);
                if (_stricmp(macHex, manifestHmac.toUtf8().constData()) != 0) {
                    safeThis->logEvent("UPDATE", "Rejected manifest with invalid HMAC");
                    return;
                }
                QJsonParseError manifestParseError{};
                QJsonDocument manifestDoc = QJsonDocument::fromJson(payload, &manifestParseError);
                if (manifestParseError.error != QJsonParseError::NoError || !manifestDoc.isObject()) {
                    safeThis->logEvent("UPDATE", "Rejected malformed manifest payload");
                    return;
                }
                QJsonObject manifest = manifestDoc.object();
                QString latest = manifest["version"].toString();
                QString url = manifest["package_url"].toString();
                QString sha = manifest["package_sha256"].toString();
                QString kind = manifest["package_kind"].toString("bundle");
                QString releaseID = manifest["release_id"].toString();
                QString notes = manifest["release_notes"].toString();
                bool force = manifest["force_update"].toBool(false);
                safeThis->handleInstallerUpdateCheck(latest, url, force, sha, kind, releaseID, notes);
            }, Qt::QueuedConnection);
        });
    }

    void ApplyInstallerUpdate(const QString& version, const QString& url,
                              const QString& expectedSha256, const QString& packageKind,
                              const QString& releaseID, QMessageBox* progressDlg = nullptr) {
        UpdateDownloadTarget target = ResolveUpdateDownloadTarget(url, QString::fromUtf8(_S("安装包下载")), SERVER_HOST, SERVER_PORT);
        if (!target.error.isEmpty()) {
            dismissProgressBox(progressDlg);
            m_lastUpdateStatus = target.error;
            setStatus("error", target.error);
            showThemedWarning(QString::fromUtf8(_S("更新失败")), target.error);
            return;
        }

        WCHAR tempDirBuf[MAX_PATH];
        GetTempPathW(MAX_PATH, tempDirBuf);
        QString ext = packageKind.compare("msi", Qt::CaseInsensitive) == 0 ? ".msi" : ".exe";
        auto sTempFile = std::wstring(tempDirBuf) + L"LingqiaoSetup_v" + version.toStdWString() + ext.toStdWString();
        auto sHost = target.host;
        int port = target.port;
        auto sPath = target.path;
        bool secure = target.secure;
        auto sToken = m_sessionToken;
        auto sMachine = m_machineID;
        auto sCard = m_cardInput ? m_cardInput->text().trimmed() : QString();

        setStatus("idle", QString::fromUtf8(_S("正在下载安装包 v%1...")).arg(version));
        ReportUpdateEvent(releaseID, version, "download_started");

        QPointer<MainWindow> safeThis(this);
        QtConcurrent::run([safeThis, sHost, port, sPath, sTempFile, version, expectedSha256, packageKind, releaseID, progressDlg, secure, sToken, sMachine, sCard]() {
            const int maxRetries = 3;
            QString err;
            for (int attempt = 0; attempt < maxRetries; ++attempt) {
                if (attempt > 0) {
                    QMetaObject::invokeMethod(safeThis, [progressDlg, attempt, maxRetries]() {
                        if (progressDlg) progressDlg->setText(
                            QString::fromUtf8(_S("下载失败，正在重试 (%1/%2)...")).arg(attempt + 1).arg(maxRetries));
                    }, Qt::QueuedConnection);
                    Sleep(2000);
                }
                err = HttpDownloadFile(sHost.c_str(), port, sPath.c_str(), sTempFile.c_str(),
                    [safeThis, progressDlg, version](qint64 read, qint64 total) {
                        if (total <= 0) return;
                        int pct = (int)(read * 100 / total);
                        QString msg = QString::fromUtf8(_S("正在下载安装包 v%1...\n已下载 %2% (%3/%4 MB)"))
                            .arg(version).arg(pct)
                            .arg((double)read / 1048576.0, 0, 'f', 1)
                            .arg((double)total / 1048576.0, 0, 'f', 1);
                        QMetaObject::invokeMethod(safeThis, [progressDlg, msg]() {
                            if (progressDlg) progressDlg->setText(msg);
                        }, Qt::QueuedConnection);
                    }, secure, (const wchar_t*)sToken.utf16(), (const wchar_t*)sMachine.utf16(), (const wchar_t*)sCard.utf16());
                if (err.isEmpty()) break;
            }
            if (!err.isEmpty()) {
                DeleteFileW(sTempFile.c_str());
                QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, err, releaseID, version]() {
                    if (!safeThis) return;
                    safeThis->dismissProgressBox(progressDlg);
                    QString readable = safeThis->readableHttpFailure(QString::fromUtf8(_S("安装包下载")), err);
                    safeThis->m_lastUpdateStatus = readable;
                    safeThis->ReportUpdateEvent(releaseID, version, "download_failed", "network", readable);
                    safeThis->setStatus("error", readable);
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")),
                        QString::fromUtf8(_S("%1\n\n请检查网络、会话状态，或联系管理员确认更新入口。")).arg(readable));
                }, Qt::QueuedConnection);
                return;
            }

            if (!UpdatePackageMagicOk(sTempFile.c_str(), packageKind, 4096)) {
                DeleteFileW(sTempFile.c_str());
                QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, releaseID, version]() {
                    if (!safeThis) return;
                    safeThis->dismissProgressBox(progressDlg);
                    safeThis->ReportUpdateEvent(releaseID, version, "download_failed", "invalid_package");
                    safeThis->setStatus("error", QString::fromUtf8(_S("安装包格式校验失败")));
                    safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("下载的安装包格式不正确。")));
                }, Qt::QueuedConnection);
                return;
            }

            if (!expectedSha256.isEmpty()) {
                QString actual = Sha256FileHex(sTempFile.c_str());
                if (actual.isEmpty() || !Sha256Matches(actual, expectedSha256)) {
                    DeleteFileW(sTempFile.c_str());
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, releaseID, version]() {
                        if (!safeThis) return;
                        safeThis->dismissProgressBox(progressDlg);
                        safeThis->ReportUpdateEvent(releaseID, version, "download_failed", "sha256_mismatch");
                        safeThis->setStatus("error", QString::fromUtf8(_S("安装包完整性校验失败")));
                        safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("安装包完整性校验失败。")));
                    }, Qt::QueuedConnection);
                    return;
                }
            }

            QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, sTempFile, version, packageKind, releaseID]() {
                if (!safeThis) return;
                safeThis->dismissProgressBox(progressDlg);
                safeThis->ReportUpdateEvent(releaseID, version, "download_completed");
                safeThis->ReportUpdateEvent(releaseID, version, "install_started");
                safeThis->m_lastUpdateStatus = QString::fromUtf8(_S("安装包已下载"));
                safeThis->setStatus("ok", QString::fromUtf8(_S("安装包已下载，正在启动安装器...")));

                std::wstring command;
                std::wstring app;
                if (packageKind.compare("msi", Qt::CaseInsensitive) == 0) {
                    app = L"msiexec.exe";
                    command = L"/i \"" + sTempFile + L"\"";
                } else {
                    app = sTempFile;
                    command.clear();
                }
                HINSTANCE launched = ShellExecuteW(NULL, L"open", app.c_str(),
                    command.empty() ? NULL : command.c_str(), NULL, SW_SHOWNORMAL);
                if ((INT_PTR)launched <= 32) {
                    DWORD e = (DWORD)(INT_PTR)launched;
                    safeThis->ReportUpdateEvent(releaseID, version, "install_failed", "launch_failed", QString::number(e));
                    safeThis->setStatus("error", QString::fromUtf8(_S("无法启动安装器 (错误: %1)")).arg(e));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")),
                        QString::fromUtf8(_S("无法启动安装器 (错误: %1)。请从临时目录手动运行安装包。")).arg(e));
                    return;
                }
                safeThis->m_updateBanner->setVisible(false);
            }, Qt::QueuedConnection);
        });
    }
