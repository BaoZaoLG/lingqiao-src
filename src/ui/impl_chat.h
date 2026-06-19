    QWidget* buildChatSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(16, 14, 16, 14);
        o->setSpacing(8);
        m_chatHeading = heading(QString::fromUtf8(_S("公共交流")));
        m_chatHeading->setStyleSheet("font-size: 12px; font-weight: 700; color: #1e293b;");
        o->addWidget(m_chatHeading);

        m_chatView = new QTextBrowser();
        m_chatView->setReadOnly(true);
        m_chatView->setAcceptRichText(true);
        m_chatView->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Expanding);
        m_chatView->setPlaceholderText(QString::fromUtf8(_S("激活后可查看公共消息")));
        m_chatView->setTextInteractionFlags(Qt::TextBrowserInteraction);
        m_chatView->setOpenExternalLinks(false);
        m_chatView->setOpenLinks(false);
        m_chatView->setStyleSheet(
            "QTextBrowser { background: rgba(255, 255, 255, 0.35); border: 1px solid rgba(200, 200, 210, 0.40); "
            "border-radius: 8px; padding: 10px; color: #1a1a2e; font-size: 12px; }");
        connect(m_chatView, &QTextBrowser::anchorClicked, this, &MainWindow::onChatAnchorClicked);
        o->addWidget(m_chatView, 1);

        // Reply Banner
        m_replyBanner = new QFrame();
        m_replyBanner->setObjectName("replyBanner");
        m_replyBanner->setStyleSheet("QFrame#replyBanner { background: rgba(74, 158, 255, 0.12); border: 1px solid rgba(74, 158, 255, 0.25); border-radius: 6px; padding: 4px 8px; }");
        QHBoxLayout* rbLayout = new QHBoxLayout(m_replyBanner);
        rbLayout->setContentsMargins(6, 4, 6, 4);
        m_replyLabel = new QLabel();
        m_replyLabel->setStyleSheet("font-size: 11px; color: #1e40af; font-weight: 500;");
        rbLayout->addWidget(m_replyLabel, 1);
        QPushButton* cancelReplyBtn = new QPushButton("×");
        cancelReplyBtn->setFixedSize(24, 24);
        cancelReplyBtn->setStyleSheet("QPushButton { border: none; background: transparent; color: #1e40af; font-weight: bold; font-size: 16px; margin: 0; padding: 0; } QPushButton:hover { color: #ef4444; }");
        connect(cancelReplyBtn, &QPushButton::clicked, this, &MainWindow::clearReplyTarget);
        rbLayout->addWidget(cancelReplyBtn);
        m_replyBanner->setVisible(false);
        o->addWidget(m_replyBanner);

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(8);

        m_chatInput = new QLineEdit();
        m_chatInput->setPlaceholderText(QString::fromUtf8(_S("输入消息，最多 300 字")));
        m_chatInput->setMaxLength(300);
        m_chatInput->setFixedHeight(34);
        m_chatInput->setStyleSheet(
            "QLineEdit { background: rgba(255, 255, 255, 0.50); border: 1px solid rgba(200, 200, 210, 0.45); "
            "border-radius: 8px; padding: 6px 10px; font-size: 12px; color: #0f172a; } "
            "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }");
        installThemedLineEditMenu(m_chatInput);
        connect(m_chatInput, &QLineEdit::returnPressed, this, &MainWindow::sendChatMessage);
        row->addWidget(m_chatInput, 1);

        m_chatSendBtn = new AnimatedButton(QString::fromUtf8(_S("发送")), AnimatedButton::GhostStyle);
        m_chatSendBtn->setFixedSize(64, 34);
        m_chatSendBtn->setStyleSheet(
            "QPushButton { background: rgba(74, 158, 255, 0.85); border: 1px solid rgba(74, 158, 255, 0.60); "
            "border-radius: 8px; color: #ffffff; font-weight: 600; font-size: 12px; } "
            "QPushButton:hover { background: rgba(109, 179, 255, 0.90); } "
            "QPushButton:disabled { background: rgba(74, 158, 255, 0.40); color: rgba(255,255,255,0.5); }");
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
        m_localMessages = QJsonArray();
        clearReplyTarget();
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

    void clearReplyTarget() {
        m_chatReplyToID = 0;
        m_chatReplyAuthor.clear();
        m_chatReplyPreview.clear();
        if (m_replyBanner) m_replyBanner->setVisible(false);
    }

    void onChatAnchorClicked(const QUrl& link) {
        QString url = link.toString();
        if (url.startsWith("reply:")) {
            qint64 id = url.mid(6).toLongLong();
            for (const QJsonValue& val : m_localMessages) {
                QJsonObject msg = val.toObject();
                if ((qint64)msg["id"].toDouble() == id) {
                    m_chatReplyToID = id;
                    m_chatReplyAuthor = msg["author"].toString();
                    m_chatReplyPreview = msg["content"].toString();
                    if (m_chatReplyPreview.length() > 30) {
                        m_chatReplyPreview = m_chatReplyPreview.left(30) + "...";
                    }
                    if (m_replyLabel) {
                        m_replyLabel->setText(QString::fromUtf8(_S("回复 @%1: \"%2\""))
                            .arg(m_chatReplyAuthor, m_chatReplyPreview));
                    }
                    if (m_replyBanner) m_replyBanner->setVisible(true);
                    break;
                }
            }
        } else if (url.startsWith("react:")) {
            QStringList parts = url.split(':');
            if (parts.size() >= 3) {
                qint64 messageID = parts[1].toLongLong();
                QString reaction = parts[2];
                sendChatReaction(messageID, reaction);
            }
        } else if (url.startsWith("react-menu:")) {
            qint64 messageID = url.mid(11).toLongLong();
            showReactionMenu(messageID);
        }
    }

    void showReactionMenu(qint64 messageID) {
        QMenu menu(this);
        menu.setStyleSheet(POPUP_CSS);
        
        QStringList whitelist = { "👍", "❤️", "😂", "？", "收到" };
        for (const QString& emo : whitelist) {
            QAction* action = menu.addAction(emo);
            connect(action, &QAction::triggered, this, [this, messageID, emo]() {
                sendChatReaction(messageID, emo);
            });
        }
        
        menu.exec(QCursor::pos());
    }

    void sendChatReaction(qint64 messageID, const QString& reaction) {
        if (!m_activated || m_sessionToken.isEmpty() || m_machineID.isEmpty()) return;
        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        req["message_id"] = messageID;
        req["reaction"] = reaction;
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/react", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (resp.statusCode == 200) {
                    safeThis->fetchChatMessages();
                } else if (resp.statusCode == 403) {
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    QString errText = QString::fromUtf8(_S("你已被禁言，暂时无法进行表情反应"));
                    if (parseError.error == QJsonParseError::NoError && doc.isObject()) {
                        QString msg = doc.object()["message"].toString();
                        if (!msg.isEmpty()) errText = msg;
                    }
                    safeThis->setStatus("warn", errText);
                }
            }, Qt::QueuedConnection);
        });
    }

    QString renderChatHtml(const QJsonArray& messages) {
        QString html = QString("<html><head><style>"
            "body { font-family: 'Microsoft YaHei', 'Segoe UI', Arial, sans-serif; font-size: %1px; color: #1a1a2e; margin: 0; padding: %2px; }"
            "a { text-decoration: none; }"
            ".react-chip { background-color: rgba(0, 0, 0, 0.04); border: 1px solid rgba(0, 0, 0, 0.06); border-radius: %3px; padding: %4px %5px; color: #475569; font-size: %6px; }"
            ".react-chip-active { background-color: rgba(74, 158, 255, 0.15); border: 1px solid rgba(74, 158, 255, 0.3); border-radius: %3px; padding: %4px %5px; color: #1e40af; font-weight: bold; font-size: %6px; }"
            ".react-add-btn { color: #64748b; font-size: %7px; padding: %4px %5px; font-weight: bold; }"
            "</style></head><body>")
            .arg(spx(12)).arg(spx(4)).arg(spx(4)).arg(spx(1)).arg(spx(4)).arg(spx(10)).arg(spx(11));

        for (int i = 0; i < messages.size(); ++i) {
            QJsonObject msg = messages[i].toObject();
            qint64 id = (qint64)msg["id"].toDouble();
            QString author = msg["author"].toString();
            QString content = msg["content"].toString();
            QString created = msg["created_at"].toString();
            QString type = msg["type"].toString("user");
            
            QString timeText = QDateTime::fromString(created, Qt::ISODateWithMs).toLocalTime().toString("hh:mm");
            if (timeText.isEmpty()) timeText = QDateTime::currentDateTime().toString("hh:mm");

            QString authorID = msg["author_id"].toString();
            bool isSelf = (!authorID.isEmpty() && authorID == m_chatAuthorID);

            if (type == "system") {
                html += QString(
                    "<table align=\"center\" style=\"margin-top: %1px; margin-bottom: %1px;\">"
                    "  <tr>"
                    "    <td style=\"background-color: #f8fafc; color: #64748b; padding: %2px %3px; font-size: %4px; border: 1px solid #e2e8f0; border-radius: %2px;\">"
                    "      [系统] %5"
                    "    </td>"
                    "  </tr>"
                    "</table>"
                ).arg(spx(6)).arg(spx(4)).arg(spx(12)).arg(spx(11)).arg(content.toHtmlEscaped());
            } else {
                QString align = isSelf ? "right" : "left";
                QString bgColor = isSelf ? "#4a9eff" : "rgba(0,0,0,0.05)";
                QString textColor = isSelf ? "#ffffff" : "#1e293b";
                QString displayName = isSelf ? QString::fromUtf8(_S("我")) : author;

                html += QString(
                    "<table align=\"%1\" style=\"margin-top: %2px; margin-bottom: %3px; max-width: 85%;\" cellpadding=\"0\" cellspacing=\"0\">"
                    "  <tr>"
                    "    <td align=\"%1\" style=\"color: #64748b; font-size: %4px; padding-bottom: %5px;\">"
                ).arg(align).arg(spx(8)).arg(spx(4)).arg(spx(11)).arg(spx(2));

                if (isSelf) {
                    html += QString(
                        "      <span style=\"color: #64748b; font-size: %1px;\">%2</span>&nbsp; "
                        "      <b>%3</b>&nbsp; "
                        "      <a href=\"react-menu:%4\" style=\"color: #64748b; font-size: %5px; font-weight: bold; text-decoration: none; font-family: 'Segoe UI Symbol'; vertical-align: baseline;\">☺</a>"
                    ).arg(spx(10)).arg(timeText).arg(displayName.toHtmlEscaped()).arg(QString::number(id)).arg(spx(11));
                } else {
                    html += QString(
                        "      <b>%1</b>&nbsp; "
                        "      <span style=\"color: #64748b; font-size: %2px;\">%3</span>&nbsp; "
                        "      <a href=\"reply:%4\" style=\"color: #4a9eff; font-size: %5px; text-decoration: none; vertical-align: baseline;\">回复</a>&nbsp; "
                        "      <a href=\"react-menu:%4\" style=\"color: #64748b; font-size: %5px; font-weight: bold; text-decoration: none; font-family: 'Segoe UI Symbol'; vertical-align: baseline;\">☺</a>"
                    ).arg(displayName.toHtmlEscaped()).arg(spx(10)).arg(timeText).arg(QString::number(id)).arg(spx(11));
                }

                html += QString(
                    "    </td>"
                    "  </tr>"
                    "  <tr>"
                    "    <td style=\"background-color: %1; color: %2; padding: %3px %4px; border-radius: %5px; font-size: %6px; line-height: 1.4;\">"
                ).arg(bgColor, textColor).arg(spx(8)).arg(spx(12)).arg(spx(8)).arg(spx(12));

                // Quoted reply
                QJsonObject replyPreview = msg["reply_preview"].toObject();
                if (!replyPreview.isEmpty()) {
                    QString replyAuthor = replyPreview["author"].toString();
                    QString replyContent = replyPreview["content"].toString();
                    QString replyBg = isSelf ? "rgba(255, 255, 255, 0.16)" : "rgba(0, 0, 0, 0.03)";
                    QString replyBorderColor = isSelf ? "#ffffff" : "#cbd5e1";
                    QString replyTextColor = isSelf ? "#f8fafc" : "#475569";
                    html += QString(
                        "<div style=\"background-color: %1; border-left: 3px solid %2; padding: %3px %4px; margin-bottom: %5px; border-radius: %3px; font-size: %6px; color: %7;\">"
                        "  @%8: %9"
                        "</div><br>"
                    ).arg(replyBg, replyBorderColor).arg(spx(4)).arg(spx(8)).arg(spx(6)).arg(spx(11)).arg(replyTextColor, replyAuthor.toHtmlEscaped(), replyContent.toHtmlEscaped());
                }

                html += content.toHtmlEscaped();
                html += "</td></tr>";

                // Reactions
                QJsonObject reactions = msg["reactions"].toObject();
                QJsonArray reacted = msg["reacted"].toArray();
                
                QStringList reactionChips;
                QStringList whitelist = { "👍", "❤️", "😂", "？", "收到" };
                
                for (const QString& emo : whitelist) {
                    int count = reactions[emo].toInt(0);
                    if (count > 0) {
                        bool hasReacted = false;
                        for (int r = 0; r < reacted.size(); ++r) {
                            if (reacted[r].toString() == emo) {
                                hasReacted = true;
                                break;
                            }
                        }
                        QString chipClass = hasReacted ? "react-chip-active" : "react-chip";
                        reactionChips.append(QString("<a href=\"react:%1:%2\" style=\"text-decoration: none;\"><span class=\"%3\">%4 %5</span></a>")
                            .arg(QString::number(id), emo, chipClass, emo, QString::number(count)));
                    }
                }

                if (!reactionChips.isEmpty()) {
                    reactionChips.append(QString("<a href=\"react-menu:%1\" style=\"text-decoration: none;\"><span class=\"react-add-btn\">+☺</span></a>").arg(QString::number(id)));
                    html += QString(
                        "  <tr>"
                        "    <td align=\"%1\" style=\"padding-top: 4px; padding-bottom: 2px;\">"
                        "      %2"
                        "    </td>"
                        "  </tr>"
                    ).arg(align, reactionChips.join(" "));
                }

                html += "</table><div style=\"clear: both;\"></div>";
            }
        }
        html += "</body></html>";
        return html;
    }

    void updateChatView(const QJsonArray& messages, bool force = false) {
        bool changed = force || (messages.size() != m_localMessages.size());
        if (!changed) {
            for (int i = 0; i < messages.size(); ++i) {
                QJsonObject a = messages[i].toObject();
                QJsonObject b = m_localMessages[i].toObject();
                if (a["id"].toDouble() != b["id"].toDouble() ||
                    a["reactions"] != b["reactions"] ||
                    a["reacted"] != b["reacted"] ||
                    a["author"] != b["author"] ||
                    a["content"] != b["content"]) {
                    changed = true;
                    break;
                }
            }
        }

        if (!changed && !m_localMessages.isEmpty()) {
            return;
        }

        int scrollVal = 0;
        int scrollMax = 0;
        QScrollBar* bar = m_chatView ? m_chatView->verticalScrollBar() : nullptr;
        if (bar) {
            scrollVal = bar->value();
            scrollMax = bar->maximum();
        }

        m_localMessages = messages;
        QString html = renderChatHtml(messages);
        if (m_chatView) {
            m_chatView->setHtml(html);
        }

        qint64 lastReadID = m_chatLastID;
        qint64 maxID = lastReadID;
        int newUnreads = 0;

        for (const QJsonValue& val : messages) {
            QJsonObject msg = val.toObject();
            qint64 id = (qint64)msg["id"].toDouble();
            maxID = qMax(maxID, id);
            if (id > lastReadID) {
                newUnreads++;
            }
        }

        const bool communityVisible = m_pageStack && m_pageStack->currentIndex() == 1
            && isActiveWindow() && !isMinimized() && isVisible();
        if (communityVisible) {
            m_chatLastID = maxID;
            m_chatUnreadCount = 0;
            if (m_chatHeading) m_chatHeading->setText(QString::fromUtf8(_S("公共聊天")));
        } else {
            if (newUnreads > 0) {
                m_chatUnreadCount = newUnreads;
                if (m_chatHeading) {
                    m_chatHeading->setText(QString::fromUtf8(_S("公共聊天（%1 条未读）")).arg(m_chatUnreadCount));
                }
            }
        }

        updatePageNav();

        if (bar) {
            if (scrollVal >= scrollMax - 15) {
                bar->setValue(bar->maximum());
            } else {
                bar->setValue(scrollVal);
            }
        }
    }

    void fetchChatMessages() {
        if (!m_activated || m_sessionToken.isEmpty() || m_machineID.isEmpty() || m_chatFetchInProgress) return;
        QString card = m_cardInput ? m_cardInput->text().trimmed() : QString();
        if (card.isEmpty()) return;
        m_chatFetchInProgress = true;
        QPointer<MainWindow> safeThis(this);
        QJsonObject req = buildChatAuthRequest();
        req["after_id"] = 0;
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
                safeThis->updateChatView(messages);
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
                        QJsonParseError parseError{};
                        QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                        QString errText = safeThis->readableHttpFailure(QString::fromUtf8(_S("昵称保存")), 
                            resp.statusCode != 0 ? QString("HTTP %1").arg(resp.statusCode) : resp.error);
                        if (resp.statusCode == 403) {
                            errText = QString::fromUtf8(_S("昵称保存失败：你已被禁言，暂时无法修改昵称"));
                        }
                        if (parseError.error == QJsonParseError::NoError && doc.isObject()) {
                            QString msg = doc.object()["message"].toString();
                            if (!msg.isEmpty()) {
                                if (resp.statusCode == 403) errText = msg;
                                else errText = QString::fromUtf8(_S("昵称保存失败：")) + msg;
                            }
                        }
                        safeThis->setStatus("warn", errText);
                    }
                    return;
                }
                QJsonParseError parseError{};
                QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                if (parseError.error != QJsonParseError::NoError || !doc.isObject()) return;
                QJsonObject profile = doc.object()["profile"].toObject();
                
                QString authorID = profile["author_id"].toString();
                if (!authorID.isEmpty()) {
                    safeThis->m_chatAuthorID = authorID;
                }
                
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
        if (m_chatReplyToID > 0) {
            req["reply_to_id"] = m_chatReplyToID;
        }
        QByteArray body = QJsonDocument(req).toJson(QJsonDocument::Compact);
        QtConcurrent::run([safeThis, body]() {
            HttpResponse resp = HttpPostJson(SERVER_HOST, SERVER_PORT, L"/api/v1/chat/send", body);
            QMetaObject::invokeMethod(safeThis, [safeThis, resp]() {
                if (!safeThis) return;
                if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setEnabled(safeThis->m_activated);
                if (resp.statusCode == 200) {
                    safeThis->m_chatRetryContent.clear();
                    safeThis->clearReplyTarget();
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
                } else if (resp.statusCode == 403) {
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    QString errText = QString::fromUtf8(_S("消息发送失败：你已被禁言，暂时无法发送消息"));
                    if (parseError.error == QJsonParseError::NoError && doc.isObject()) {
                        QString msg = doc.object()["message"].toString();
                        if (!msg.isEmpty()) errText = msg;
                    }
                    safeThis->m_lastChatStatus = errText;
                    safeThis->setStatus("warn", errText);
                    if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setText(QString::fromUtf8(_S("重试")));
                } else {
                    QJsonParseError parseError{};
                    QJsonDocument doc = QJsonDocument::fromJson(resp.body, &parseError);
                    QString detail = resp.statusCode != 0 ? QString("HTTP %1").arg(resp.statusCode) : resp.error;
                    QString msg = safeThis->readableHttpFailure(QString::fromUtf8(_S("消息发送")), detail);
                    if (parseError.error == QJsonParseError::NoError && doc.isObject()) {
                        QString serverMsg = doc.object()["message"].toString();
                        if (!serverMsg.isEmpty()) msg = QString::fromUtf8(_S("消息发送失败：")) + serverMsg;
                    }
                    safeThis->m_lastChatStatus = msg;
                    safeThis->setStatus("warn", msg);
                    if (safeThis->m_chatSendBtn) safeThis->m_chatSendBtn->setText(QString::fromUtf8(_S("重试")));
                }
            }, Qt::QueuedConnection);
        });
    }
