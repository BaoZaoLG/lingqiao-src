    void ApplyUpdate(const QString& version, const QString& url, const QString& expectedSha256 = QString(), QMessageBox* progressDlg = nullptr) {
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
        auto expectedSha = expectedSha256;
        QtConcurrent::run([safeThis, sHost, port, sPath, sTempFile, sExe, sTempDir, version, progressDlg, expectedSha]() {
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
                // Verify SHA-256 if server provided expected hash
                if (!expectedSha.isEmpty() && _stricmp(fileHashHex, expectedSha.toUtf8().constData()) != 0) {
                    DeleteFileW(tempFile);
                    QMetaObject::invokeMethod(safeThis, [safeThis, progressDlg]() {
                        if (!safeThis) return;
                        if (progressDlg) progressDlg->close();
                        safeThis->setStatus("error", QString::fromUtf8(_S("更新文件哈希不匹配")));
                        safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                        QMessageBox::warning(safeThis, QString::fromUtf8(_S("更新失败")), QString::fromUtf8(_S("下载的文件完整性校验失败，可能被篡改。")));
                    }, Qt::QueuedConnection);
                    return;
                }
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
                BOOL writeOk = WriteFile(hBat, bat.c_str(), batLen, &written, NULL);
                CloseHandle(hBat);
                if (!writeOk || written != batLen) {
                    DeleteFileW(tempFile);
                    DeleteFileW(batPath.c_str());
                    safeThis->setStatus("error", QString::fromUtf8(_S("写入更新脚本失败")));
                    safeThis->m_injectBtn->setEnabled(!safeThis->m_forceUpdateBlocked);
                    return;
                }

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
