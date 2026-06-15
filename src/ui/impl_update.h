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
        dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
        dlg.setStyleSheet(POPUP_CSS);
        dlg.exec();
    }

    void ApplyUpdate(const QString& version, const QString& url, const QString& expectedSha256 = QString(), QMessageBox* progressDlg = nullptr) {
        UpdateDownloadTarget target = ResolveUpdateDownloadTarget(url, QString::fromUtf8(_S("更新下载")), SERVER_HOST, SERVER_PORT);
        if (!target.error.isEmpty()) {
            dismissProgressBox(progressDlg);
            setStatus("error", target.error);
            showThemedWarning(QString::fromUtf8(_S("更新失败")), target.error);
            return;
        }

        auto sHost = target.host;
        int port = target.port;
        auto sPath = target.path;
        bool secure = target.secure;

        WCHAR buf[MAX_PATH] = {0};
        GetModuleFileNameW(NULL, buf, MAX_PATH);
        auto sExe = std::wstring(buf);

        WCHAR tempDirBuf[MAX_PATH];
        GetTempPathW(MAX_PATH, tempDirBuf);
        auto sTempDir = std::wstring(tempDirBuf);
        auto sTempFile = sTempDir + L"update_v" + version.toStdWString() + L".zip";

        setStatus("idle", QString::fromUtf8(_S("正在下载 v%1 更新...")).arg(version));
        m_injectBtn->setEnabled(false);

        // Run download in background thread to avoid UI freeze
        QPointer<MainWindow> safeThis(this);
        auto expectedSha = expectedSha256;
        QtConcurrent::run([safeThis, sHost, port, sPath, sTempFile, sExe, sTempDir, version, progressDlg, expectedSha, secure]() {
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
                    [safeThis, progressDlg, version](qint64 read, qint64 total) {
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
                    }, secure);
                if (err.isEmpty()) break;
            }

            if (!err.isEmpty()) {
                QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, err]() {
                    if (!safeThis) return;
                    safeThis->dismissProgressBox(progressDlg);
                    safeThis->setStatus("error", QString::fromUtf8(_S("更新下载失败: %1")).arg(err));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")),
                        QString::fromUtf8(_S("自动更新下载失败:\n%1\n\n请检查网络后重试，或联系管理员获取安装包。")).arg(err));
                }, Qt::QueuedConnection);
                return;
            }

            // Check if downloaded file is a ZIP archive (update package)
            HANDLE hCheck = CreateFileW(tempFile, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
            bool isZip = false;
            if (hCheck != INVALID_HANDLE_VALUE) {
                char magic[2] = {0}; DWORD rd = 0;
                ReadFile(hCheck, magic, 2, &rd, NULL);
                CloseHandle(hCheck);
                isZip = (magic[0] == 'P' && magic[1] == 'K');
            }

            // Verify SHA-256 integrity (for both ZIP and EXE)
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
                // Verify SHA-256 if server provided expected hash
                if (!expectedSha.isEmpty() && _stricmp(fileHashHex, expectedSha.toUtf8().constData()) != 0) {
                    DeleteFileW(tempFile);
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg]() {
                        if (!safeThis) return;
                        safeThis->dismissProgressBox(progressDlg);
                        safeThis->setStatus("error", QString::fromUtf8(_S("更新文件哈希不匹配")));
                        safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                        safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("下载的文件完整性校验失败，可能被篡改。")));
                    }, Qt::QueuedConnection);
                    return;
                }
                if (!isZip && (magic[0] != 'M' || magic[1] != 'Z' || fileSize < 4096)) {
                    DeleteFileW(tempFile);
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg]() {
                        if (!safeThis) return;
                        safeThis->dismissProgressBox(progressDlg);
                        safeThis->setStatus("error", QString::fromUtf8(_S("更新文件校验失败")));
                        safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                        safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("下载的文件不是有效的可执行文件。")));
                    }, Qt::QueuedConnection);
                    return;
                }
            }

            // Replace process — different for ZIP (package) vs EXE (single file)
            QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, sTempFile, sExe, sTempDir, version, isZip]() {
                if (!safeThis) return;
                safeThis->dismissProgressBox(progressDlg);
                safeThis->setStatus("ok", QString::fromUtf8(_S("更新下载完成，正在准备替换...")));

                const wchar_t* tempFile = sTempFile.c_str();
                const wchar_t* exePath = sExe.c_str();

                if (isZip) {
                    // ZIP update: extract to app directory, replacing all files
                    WCHAR appDir[MAX_PATH] = {0};
                    wcscpy_s(appDir, exePath);
                    WCHAR* slash = wcsrchr(appDir, L'\\');
                    if (slash) *slash = 0;

                    // Use PowerShell to extract ZIP to app directory
                    WCHAR cmdLine[1024];
                    swprintf_s(cmdLine, _countof(cmdLine),
                        L"powershell -NoProfile -Command \"Expand-Archive -Path '%s' -DestinationPath '%s' -Force\"",
                        tempFile, appDir);

                    STARTUPINFOW si = {0}; si.cb = sizeof(si); si.dwFlags = STARTF_USESHOWWINDOW; si.wShowWindow = SW_HIDE;
                    PROCESS_INFORMATION pi = {0};
                    BOOL ok = CreateProcessW(NULL, cmdLine, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
                    if (ok) {
                        WaitForSingleObject(pi.hProcess, 30000);
                        CloseHandle(pi.hProcess); CloseHandle(pi.hThread);
                    }
                    DeleteFileW(tempFile);

                    // Launch new EXE
                    STARTUPINFOW si2 = {0}; si2.cb = sizeof(si2);
                    PROCESS_INFORMATION pi2 = {0};
                    if (CreateProcessW(exePath, NULL, NULL, NULL, FALSE, 0, NULL, NULL, &si2, &pi2)) {
                        if (pi2.hProcess) { CloseHandle(pi2.hProcess); CloseHandle(pi2.hThread); }
                    }
                    QTimer::singleShot(500, safeThis, []() { ExitProcess(0); });
                } else {
                    // Single EXE update (legacy path)
                    auto bakPath = std::wstring(exePath) + L".bak";
                    CopyFileW(exePath, bakPath.c_str(), FALSE);

                    BOOL moved = MoveFileExW(tempFile, exePath, MOVEFILE_REPLACE_EXISTING);
                    if (!moved && GetLastError() == ERROR_ACCESS_DENIED) {
                        moved = MoveFileExW(tempFile, exePath, MOVEFILE_REPLACE_EXISTING | MOVEFILE_DELAY_UNTIL_REBOOT);
                        if (!moved) {
                            DeleteFileW(tempFile);
                            safeThis->setStatus("error", QString::fromUtf8(_S("无法替换程序文件，请手动替换")));
                            safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                            return;
                        }
                        safeThis->showThemedInfo(QString::fromUtf8(_S("更新")),
                            QString::fromUtf8(_S("更新已准备好，将在下次重启时完成。")));
                        return;
                    }
                    if (!moved) {
                        MoveFileExW(bakPath.c_str(), exePath, MOVEFILE_REPLACE_EXISTING);
                        DeleteFileW(tempFile);
                        safeThis->setStatus("error", QString::fromUtf8(_S("替换程序文件失败 (错误: %1)")).arg(GetLastError()));
                        safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                        return;
                    }

                    STARTUPINFOW si = {0}; si.cb = sizeof(si);
                    PROCESS_INFORMATION pi = {0};
                    if (!CreateProcessW(exePath, NULL, NULL, NULL, FALSE,
                        0, NULL, NULL, &si, &pi)) {
                        safeThis->setStatus("error", QString::fromUtf8(_S("更新完成但无法启动新版本 (错误: %1)")).arg(GetLastError()));
                    } else {
                        if (pi.hProcess) { CloseHandle(pi.hProcess); CloseHandle(pi.hThread); }
                    }

                    DeleteFileW(bakPath.c_str());
                    QTimer::singleShot(500, safeThis, []() { ExitProcess(0); });
                }
            }, Qt::QueuedConnection);
        });
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
                QString pinnedKey = QString::fromUtf8(UPDATE_MANIFEST_PUBLIC_KEY_HEX);
                if (payloadB64.isEmpty() || manifestHmac.isEmpty() || signature.isEmpty() || publicKey.isEmpty()) {
                    safeThis->logEvent("UPDATE", "Rejected manifest without payload/HMAC/signature/public key");
                    return;
                }
                if (!pinnedKey.isEmpty() && publicKey.compare(pinnedKey, Qt::CaseInsensitive) != 0) {
                    safeThis->logEvent("UPDATE", "Rejected manifest with unexpected public key");
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

            HANDLE hCheck = CreateFileW(sTempFile.c_str(), GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
            if (hCheck == INVALID_HANDLE_VALUE) {
                QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, releaseID, version]() {
                    if (!safeThis) return;
                    safeThis->dismissProgressBox(progressDlg);
                    safeThis->ReportUpdateEvent(releaseID, version, "download_failed", "open_failed");
                    safeThis->setStatus("error", QString::fromUtf8(_S("无法读取安装包")));
                    safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("无法读取安装包。")));
                }, Qt::QueuedConnection);
                return;
            }
            char magic[2] = {0};
            DWORD rd = 0;
            DWORD fileSize = GetFileSize(hCheck, NULL);
            ReadFile(hCheck, magic, 2, &rd, NULL);
            CloseHandle(hCheck);
            bool looksMsi = (magic[0] == (char)0xD0 && magic[1] == (char)0xCF);
            bool looksExe = (magic[0] == 'M' && magic[1] == 'Z');
            bool expectsMsi = packageKind.compare("msi", Qt::CaseInsensitive) == 0;
            if (fileSize < 4096 || (expectsMsi && !looksMsi) || (!expectsMsi && !looksExe)) {
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
                QFile f(QString::fromStdWString(sTempFile));
                if (!f.open(QIODevice::ReadOnly)) {
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg, releaseID, version]() {
                        if (!safeThis) return;
                        safeThis->dismissProgressBox(progressDlg);
                        safeThis->ReportUpdateEvent(releaseID, version, "download_failed", "open_failed");
                        safeThis->setStatus("error", QString::fromUtf8(_S("无法读取安装包")));
                        safeThis->showThemedWarning(QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("无法读取安装包。")));
                    }, Qt::QueuedConnection);
                    return;
                }
                QByteArray actual = QCryptographicHash::hash(f.readAll(), QCryptographicHash::Sha256).toHex();
                if (_stricmp(actual.constData(), expectedSha256.toUtf8().constData()) != 0) {
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
