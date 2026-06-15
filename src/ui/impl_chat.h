    QWidget* buildChatSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(10);
        m_chatHeading = heading(QString::fromUtf8(_S("公共聊天")));
        o->addWidget(m_chatHeading);

        m_chatView = new QTextEdit();
        m_chatView->setReadOnly(true);
        m_chatView->setAcceptRichText(false);
        m_chatView->setFixedHeight(120);
        m_chatView->setPlaceholderText(QString::fromUtf8(_S("激活后可查看公共消息")));
        m_chatView->setStyleSheet(
            "QTextEdit { background: rgba(255,255,255,0.26); border: 1px solid rgba(200,200,210,0.32); "
            "border-radius: 8px; padding: 8px; color: #334155; font-size: 12px; }");
        o->addWidget(m_chatView);

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(8);

        m_chatInput = new QLineEdit();
        m_chatInput->setPlaceholderText(QString::fromUtf8(_S("输入消息，最多 300 字")));
        m_chatInput->setMaxLength(300);
        m_chatInput->setFixedHeight(34);
        installThemedLineEditMenu(m_chatInput);
        connect(m_chatInput, &QLineEdit::returnPressed, this, &MainWindow::sendChatMessage);
        row->addWidget(m_chatInput, 1);

        m_chatSendBtn = new AnimatedButton(QString::fromUtf8(_S("发送")), AnimatedButton::GhostStyle);
        m_chatSendBtn->setFixedSize(72, 34);
        m_chatSendBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(12), spx(8)));
        connect(m_chatSendBtn, &QPushButton::clicked, this, &MainWindow::sendChatMessage);
        row->addWidget(m_chatSendBtn);

        o->addLayout(row);
        m_chatPanel = c;
        return c;
    }

    void startChatPolling() {
        if (!m_chatTimer) {
            m_chatTimer = new QTimer(this);
            m_chatTimer->setInterval(3000);
            connect(m_chatTimer, &QTimer::timeout, this, [this]() {
                fetchChatMessages();
                touchChatPresence();
            });
        }
        m_lastChatStatus = QString::fromUtf8(_S("已连接"));
        updateCommunityStateText();
        if (!m_chatTimer->isActive()) m_chatTimer->start();
        fetchChatProfile();
        fetchChatMessages();
        touchChatPresence();
    }

    void stopChatPolling() {
        if (m_chatTimer) m_chatTimer->stop();
        m_chatFetchInProgress = false;
        m_chatPresenceInProgress = false;
        m_chatProfileInProgress = false;
        m_chatLastID = 0;
        m_chatUnreadCount = 0;
        m_chatOnlineCount = 0;
        m_lastChatStatus = QString::fromUtf8(_S("未连接"));
        if (m_chatHeading) m_chatHeading->setText(QString::fromUtf8(_S("公共聊天")));
        if (m_chatView) m_chatView->clear();
        if (m_chatNicknameInput) m_chatNicknameInput->clear();
        updateCommunityStateText();
        updatePageNav();
    }

    QJsonObject buildChatAuthRequest() const {
        QJsonObject req;
        req["client_id"] = QString::fromWCharArray(CLIENT_ID);
        req["session_token"] = m_sessionToken;
        req["machine_id"] = m_machineID;
        req["card"] = m_cardInput ? m_cardInput->text().trimmed() : QString();
        return req;
    }

    void fetchChatMessages() {
        if (!m_activated || m_sessionToken.isEmpty() || m_machineID.isEmpty() || m_chatFetchInProgress) return;
        QString card = m_cardInput ? m_cardInput->text().trimmed() : QString();
        if (card.isEmpty()) return;
        m_chatFetchInProgress = true;
        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        req["after_id"] = m_chatLastID;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/messages", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                safeThis->m_chatFetchInProgress = false;
                if (resp.statusCode != 200 || resp.body.isEmpty()) {
                    if (resp.statusCode != 0)
                        safeThis->m_lastChatStatus = safeThis->readableHttpFailure(QString::fromUtf8(_S("聊天拉取")), QString("HTTP %1").arg(resp.statusCode));
                    else if (!resp.error.isEmpty())
                        safeThis->m_lastChatStatus = safeThis->readableHttpFailure(QString::fromUtf8(_S("聊天拉取")), resp.error);
                    safeThis->updateCommunityStateText();
                    return;
                }
                safeThis->m_lastChatStatus = QString::fromUtf8(_S("正常"));
                safeThis->updateCommunityStateText();
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) return;
                QJsonArray messages = doc.object()["messages"].toArray();
                for (const QJsonValue& value : messages) {
                    if (!value.isObject()) continue;
                    QJsonObject msg = value.toObject();
                    qint64 id = (qint64)msg["id"].toDouble();
                    QString author = msg["author"].toString();
                    QString content = msg["content"].toString();
                    QString created = msg["created_at"].toString();
                    QString type = msg["type"].toString("user");
                    if (id <= safeThis->m_chatLastID || content.isEmpty()) continue;
                    safeThis->appendChatMessage(id, type, author, content, created);
                }
            }, Qt::QueuedConnection);
        });
    }

    void touchChatPresence() {
        if (!m_activated || m_sessionToken.isEmpty() || m_machineID.isEmpty() || m_chatPresenceInProgress) return;
        QString card = m_cardInput ? m_cardInput->text().trimmed() : QString();
        if (card.isEmpty()) return;
        m_chatPresenceInProgress = true;
        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/presence", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                safeThis->m_chatPresenceInProgress = false;
                if (resp.statusCode != 200 || resp.body.isEmpty()) return;
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) return;
                int online = doc.object()["online"].toInt();
                if (online > 0) {
                    safeThis->m_chatOnlineCount = online;
                    safeThis->updateCommunityStateText();
                }
            }, Qt::QueuedConnection);
        });
    }

    void fetchChatProfile() {
        syncChatProfile(QString());
    }

    void saveChatProfile() {
        QString nickname = m_chatNicknameInput ? m_chatNicknameInput->text().trimmed() : QString();
        if (nickname.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请输入社区昵称")));
            return;
        }
        syncChatProfile(nickname);
    }

    void syncChatProfile(const QString& nickname) {
        if (!m_activated || m_sessionToken.isEmpty() || m_machineID.isEmpty() || m_chatProfileInProgress) return;
        QString card = m_cardInput ? m_cardInput->text().trimmed() : QString();
        if (card.isEmpty()) return;
        m_chatProfileInProgress = true;
        const bool saving = !nickname.isEmpty();
        if (saving && m_chatNicknameSaveBtn) {
            m_chatNicknameSaveBtn->setEnabled(false);
            m_chatNicknameSaveBtn->setText(QString::fromUtf8(_S("保存中")));
        }
        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        if (saving) req["nickname"] = nickname;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body, saving]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/profile", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp, saving]() {
                if (!safeThis) return;
                safeThis->m_chatProfileInProgress = false;
                if (safeThis->m_chatNicknameSaveBtn) {
                    safeThis->m_chatNicknameSaveBtn->setEnabled(safeThis->m_activated);
                    safeThis->m_chatNicknameSaveBtn->setText(QString::fromUtf8(_S("保存")));
                }
                if (resp.statusCode != 200 || resp.body.isEmpty()) {
                    if (saving) {
                        QString detail = resp.statusCode != 0 ? QString("HTTP %1").arg(resp.statusCode) : resp.error;
                        safeThis->setStatus("warn", safeThis->readableHttpFailure(QString::fromUtf8(_S("昵称保存")), detail));
                    }
                    return;
                }
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) return;
                QJsonObject profile = doc.object()["profile"].toObject();
                QString nextNickname = profile["nickname"].toString();
                if (!nextNickname.isEmpty() && safeThis->m_chatNicknameInput) {
                    safeThis->m_chatNicknameInput->setText(nextNickname);
                }
                if (saving) safeThis->setStatus("success", QString::fromUtf8(_S("社区昵称已保存")));
            }, Qt::QueuedConnection);
        });
    }

    void sendChatMessage() {
        if (!m_activated || m_sessionToken.isEmpty()) {
            setStatus("warn", QString::fromUtf8(_S("请先激活卡密")));
            return;
        }
        QString content = m_chatInput ? m_chatInput->text().trimmed() : QString();
        if (content.isEmpty()) return;
        if (m_chatSendBtn) m_chatSendBtn->setEnabled(false);
        m_chatRetryContent = content;

        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        req["content"] = content;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/send", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setEnabled(safeThis->m_activated);
                if (resp.statusCode == 200) {
                    safeThis->m_chatRetryContent.clear();
                    safeThis->m_lastChatStatus = QString::fromUtf8(_S("消息已发送"));
                    if (safeThis->m_chatInput) safeThis->m_chatInput->clear();
                    if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setText(QString::fromUtf8(_S("发送")));
                    safeThis->fetchChatMessages();
                } else if (resp.statusCode == 429) {
                    QString msg = QString::fromUtf8(_S("消息发送失败：发送过于频繁，请稍后再试"));
                    safeThis->m_lastChatStatus = msg;
                    safeThis->setStatus("warn", msg);
                    if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setText(QString::fromUtf8(_S("重试")));
                } else if (resp.statusCode == 401) {
                    safeThis->resetToInactiveState("Chat auth rejected",
                        QString::fromUtf8(_S("会话已失效，请输入新卡密")), true);
                } else {
                    QString detail = resp.statusCode != 0 ? QString("HTTP %1").arg(resp.statusCode) : resp.error;
                    QString msg = safeThis->readableHttpFailure(QString::fromUtf8(_S("消息发送")), detail);
                    safeThis->m_lastChatStatus = msg;
                    safeThis->setStatus("warn", msg);
                    if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setText(QString::fromUtf8(_S("重试")));
                }
            }, Qt::QueuedConnection);
        });
    }

    void appendChatMessage(qint64 id, const QString& type, const QString& author, const QString& content, const QString& created) {
        if (!m_chatView) return;
        m_chatLastID = qMax(m_chatLastID, id);
        QString timeText = QDateTime::fromString(created, Qt::ISODateWithMs).toLocalTime().toString("hh:mm");
        if (timeText.isEmpty()) timeText = QDateTime::currentDateTime().toString("hh:mm");
        QString label = type == "system" ? QString::fromUtf8(_S("系统")) : author;
        QString line = QString("[%1] %2: %3")
            .arg(timeText, label.toHtmlEscaped(), content.toHtmlEscaped());
        m_chatView->append(line);
        QScrollBar* bar = m_chatView->verticalScrollBar();
        if (bar) bar->setValue(bar->maximum());
        const bool communityVisible = m_pageStack && m_pageStack->currentIndex() == 1
            && isActiveWindow() && !isMinimized() && isVisible();
        if (!communityVisible) {
            m_chatUnreadCount++;
            if (m_chatHeading) {
                m_chatHeading->setText(QString::fromUtf8(_S("公共聊天（%1 条未读）")).arg(m_chatUnreadCount));
            }
            updatePageNav();
        }
    }
