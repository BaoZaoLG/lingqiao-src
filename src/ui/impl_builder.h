    void applyShadow() {
        HWND hwnd = (HWND)winId();
        BOOL dark = FALSE;
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));
    }

    void applyAcrylic() {
        HWND hwnd = (HWND)winId();
        HMODULE hUser = GetModuleHandleW(L"user32.dll");
        if (hUser) {
            auto pSWCA = (pfnSetWindowCompositionAttribute)
                GetProcAddress(hUser, "SetWindowCompositionAttribute");
            if (pSWCA) {
                // ABGR tint: alpha + white. Keeps glassy blur without letting DWM choose the base color.
                ACCENT_POLICY policy = {4, 0, static_cast<int>(0x33FFFFFFu), 0};
                WINCOMPATTR_DATA data = {19, &policy, sizeof(policy)};
                pSWCA(hwnd, &data);
            }
        }
        DWM_WINDOW_CORNER_PREFERENCE pref = (DWM_WINDOW_CORNER_PREFERENCE)DWMWCP_ROUND;
        DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &pref, sizeof(pref));
    }

    // Compare two version strings (e.g. "2.1.5" vs "2.1.10"). Returns -1, 0, or 1.
    static int CompareVersion(const QString& a, const QString& b) {
        QStringList pa = a.split('.'), pb = b.split('.');
        int n = qMax(pa.size(), pb.size());
        for (int i = 0; i < n; i++) {
            int ai = i < pa.size() ? pa[i].toInt() : 0;
            int bi = i < pb.size() ? pb[i].toInt() : 0;
            if (ai < bi) return -1;
            if (ai > bi) return 1;
        }
        return 0;
    }

    // Extract version from server error message like "版本过低，请更新到 v2.1.5 或更高版本"
    static QString ExtractVersionFromMsg(const QString& msg) {
        int vIdx = msg.indexOf('v');
        if (vIdx < 0) return QString();
        int end = msg.indexOf(' ', vIdx + 1);
        return (end > vIdx) ? msg.mid(vIdx + 1, end - vIdx - 1) : msg.mid(vIdx + 1);
    }

    // Centralized update decision — called from heartbeat, announcement, and activation handlers
    void handleUpdateCheck(const QString& latest, const QString& url, bool force, const QString& sha256 = QString()) {
        if (latest.isEmpty()) return;
        if (CompareVersion(GetClientVersion(), latest) >= 0) return;  // already up-to-date
        m_lastUpdateStatus = QString::fromUtf8(_S("发现新版本 v%1")).arg(latest);
        m_pendingUpdateInstaller = true;
        m_pendingUpdatePackageKind = url.endsWith(".msi", Qt::CaseInsensitive) ? "msi" : "bundle";
        m_pendingUpdateReleaseID.clear();
        if (force) {
            applyForceUpdateBlock(latest, url, sha256);
        } else if (!m_updateDismissed) {
            m_pendingUpdateVersion = latest;
            m_pendingUpdateUrl = url;
            m_pendingUpdateSha256 = sha256;
            m_updateLabel->setText(QString::fromUtf8(_S("<b>新版本可用:</b> v%1 已发布")).arg(latest));
            m_updateNowBtn->setVisible(!url.isEmpty());
            m_remindLaterBtn->setVisible(true);
            m_updateBanner->setVisible(true);
        }
    }

    void handleInstallerUpdateCheck(const QString& latest, const QString& url, bool force,
                                    const QString& sha256, const QString& packageKind,
                                    const QString& releaseID, const QString& notes = QString()) {
        if (latest.isEmpty()) return;
        if (CompareVersion(GetClientVersion(), latest) >= 0) return;
        m_lastUpdateStatus = QString::fromUtf8(_S("发现新版本 v%1")).arg(latest);
        m_pendingUpdateInstaller = true;
        m_pendingUpdatePackageKind = packageKind;
        m_pendingUpdateReleaseID = releaseID;
        if (force) {
            applyForceInstallerUpdateBlock(latest, url, sha256, packageKind, releaseID);
        } else if (!m_updateDismissed) {
            m_pendingUpdateVersion = latest;
            m_pendingUpdateUrl = url;
            m_pendingUpdateSha256 = sha256;
            m_updateLabel->setText(QString::fromUtf8(_S("<b>新版本可用:</b> v%1 已发布%2"))
                .arg(latest, notes.isEmpty() ? QString() : QString::fromUtf8(_S("<br>%1")).arg(notes.toHtmlEscaped())));
            m_updateNowBtn->setVisible(!url.isEmpty());
            m_remindLaterBtn->setVisible(true);
            m_updateBanner->setVisible(true);
        }
    }

    QFrame* sep() { QFrame* s = new QFrame(); s->setProperty("role", "sep"); return s; }

    QFrame* card() { QFrame* c = new QFrame(); c->setProperty("role", "card"); return c; }

    QLabel* heading(const QString& t) { QLabel* l = new QLabel(t); l->setProperty("role", "heading"); return l; }

    void buildUI() {
        QWidget* cw = new QWidget();
        cw->setObjectName("central");
        cw->setStyleSheet(QString("#central { background: transparent; border-radius: 16px; }"));
        setCentralWidget(cw);

        QVBoxLayout* root = new QVBoxLayout(cw);
        root->setContentsMargins(0, 0, 0, 0);
        root->setSpacing(0);

        // Title bar
        m_titleBar = new TitleBar();
        connect(m_titleBar, &TitleBar::closeClicked, this, &QMainWindow::close);
        connect(m_titleBar, &TitleBar::minClicked, this, [this]() { showMinimized(); });
        root->addWidget(m_titleBar);
        root->addWidget(sep());

        QHBoxLayout* nav = new QHBoxLayout();
        nav->setContentsMargins(24, 12, 24, 0);
        nav->setSpacing(8);
        m_homeNavBtn = new AnimatedButton(QString::fromUtf8(_S("首页")), AnimatedButton::GhostStyle);
        m_homeNavBtn->setFixedSize(74, 30);
        m_communityNavBtn = new AnimatedButton(QString::fromUtf8(_S("社区")), AnimatedButton::GhostStyle);
        m_communityNavBtn->setFixedSize(74, 30);
        connect(m_homeNavBtn, &QPushButton::clicked, this, [this]() { showHomePage(); });
        connect(m_communityNavBtn, &QPushButton::clicked, this, [this]() { showCommunityPage(); });
        nav->addWidget(m_homeNavBtn);
        nav->addWidget(m_communityNavBtn);
        nav->addStretch();
        root->addLayout(nav);

        m_pageStack = new QStackedWidget();
        m_pageStack->setStyleSheet("QStackedWidget { background: transparent; border: none; }");
        m_pageStack->addWidget(buildHomePage());
        m_pageStack->addWidget(buildCommunityPage());
        root->addWidget(m_pageStack, 1);

        setUiLocked(true);
        updatePageNav();
    }

    QWidget* buildHomePage() {
        QScrollArea* scroll = new QScrollArea();
        scroll->setWidgetResizable(true);
        scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
        scroll->setStyleSheet("QScrollArea { background: transparent; border: none; }");

        QWidget* body = new QWidget();
        body->setStyleSheet("background: transparent;");
        QVBoxLayout* bl = new QVBoxLayout(body);
        bl->setContentsMargins(24, 18, 24, 20);
        bl->setSpacing(16);

        bl->addWidget(buildCardSection());
        bl->addWidget(buildSessionStatusSection());

        bl->addWidget(buildTargetSection());
        bl->addWidget(buildApiKeySection());

        buildAnnouncement();
        bl->addWidget(m_announcement);

        buildUpdateBanner();
        bl->addWidget(m_updateBanner);

        bl->addSpacing(8);
        bl->addLayout(buildActionRow());

        // Injection history label
        m_historyLabel = new QLabel();
        m_historyLabel->setStyleSheet("font-size: 11px; color: #7ec8e3; background: transparent;");
        m_historyLabel->setOpenExternalLinks(false);
        m_historyLabel->setVisible(false);
        connect(m_historyLabel, &QLabel::linkActivated, this, [this](const QString& link) {
            if (QFileInfo::exists(link)) {
                m_targetInput->setText(link);
                QSettings(_S("LingQiao"), _S("Injector")).setValue("targetPath", link);
            }
        });
        bl->addWidget(m_historyLabel);

        bl->addStretch();

        scroll->setWidget(body);
        return scroll;
    }

    QWidget* buildCommunityPage() {
        QWidget* body = new QWidget();
        body->setStyleSheet("background: transparent;");
        QVBoxLayout* bl = new QVBoxLayout(body);
        bl->setContentsMargins(24, 12, 24, 16);
        bl->setSpacing(10);

        // Header Card (sleek, transparent standard card)
        QFrame* headerCard = card();
        QHBoxLayout* hl = new QHBoxLayout(headerCard);
        hl->setContentsMargins(16, 8, 16, 8);
        hl->setSpacing(10);

        // Status indicator
        m_communityState = new QLabel();
        m_communityState->setWordWrap(false);
        m_communityState->setStyleSheet("font-size: 12px; font-weight: 600; color: #475569;");
        hl->addWidget(m_communityState, 1);

        // Nickname label
        QLabel* nickLabel = new QLabel(QString::fromUtf8(_S("昵称:")));
        nickLabel->setObjectName("chatNicknameLabel");
        nickLabel->setStyleSheet("font-size: 11px; color: #64748b; font-weight: 500;");
        hl->addWidget(nickLabel);

        m_chatNicknameInput = new QLineEdit();
        m_chatNicknameInput->setPlaceholderText(QString::fromUtf8(_S("设置昵称")));
        m_chatNicknameInput->setMaxLength(16);
        m_chatNicknameInput->setFixedSize(100, 26);
        m_chatNicknameInput->setEnabled(false);
        m_chatNicknameInput->setStyleSheet(
            "QLineEdit { font-size: 11px; padding: 2px 6px; border-radius: 6px; "
            "background: rgba(255, 255, 255, 0.45); border: 1px solid rgba(200, 200, 210, 0.45); color: #0f172a; } "
            "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }");
        installThemedLineEditMenu(m_chatNicknameInput);
        connect(m_chatNicknameInput, &QLineEdit::returnPressed, this, &MainWindow::saveChatProfile);
        hl->addWidget(m_chatNicknameInput);

        m_chatNicknameSaveBtn = new AnimatedButton(QString::fromUtf8(_S("保存")), AnimatedButton::GhostStyle);
        m_chatNicknameSaveBtn->setFixedSize(48, 26);
        m_chatNicknameSaveBtn->setEnabled(false);
        m_chatNicknameSaveBtn->setStyleSheet(
            "QPushButton { font-size: 11px; padding: 2px 6px; border-radius: 6px; "
            "background: rgba(255, 255, 255, 0.60); border: 1px solid rgba(200, 200, 210, 0.50); color: #334155; font-weight: 600; } "
            "QPushButton:hover { background: rgba(240, 240, 245, 0.85); border-color: rgba(180, 180, 190, 0.60); }");
        connect(m_chatNicknameSaveBtn, &QPushButton::clicked, this, &MainWindow::saveChatProfile);
        hl->addWidget(m_chatNicknameSaveBtn);

        bl->addWidget(headerCard);

        // Chat section card (stretches to fill remaining space)
        QWidget* chatSection = buildChatSection();
        chatSection->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Expanding);
        bl->addWidget(chatSection, 1);

        return body;
    }

    void showHomePage() {
        if (m_pageStack) m_pageStack->setCurrentIndex(0);
        updatePageNav();
    }

    void showCommunityPage() {
        if (m_pageStack) m_pageStack->setCurrentIndex(1);
        m_chatUnreadCount = 0;
        if (m_chatHeading) m_chatHeading->setText(QString::fromUtf8(_S("公共聊天")));
        updatePageNav();
    }

    void updatePageNav() {
        const bool community = m_pageStack && m_pageStack->currentIndex() == 1;
        const QString communityText = m_chatUnreadCount > 0
            ? QString::fromUtf8(_S("社区(%1)")).arg(m_chatUnreadCount)
            : QString::fromUtf8(_S("社区"));
        if (m_communityNavBtn) m_communityNavBtn->setText(communityText);
        if (m_homeNavBtn) m_homeNavBtn->setStyleSheet(buttonStyle(community ? ButtonNeutral : ButtonAccent, spx(11), spx(6), 10, 4));
        if (m_communityNavBtn) m_communityNavBtn->setStyleSheet(buttonStyle(community ? ButtonAccent : ButtonNeutral, spx(11), spx(6), 10, 4));
    }

    void updateCommunityStateText() {
        if (!m_communityState) return;
        if (!m_activated) {
            m_communityState->setText(QString::fromUtf8(_S("<span style='color: #ef4444;'>●</span> 未激活")));
            return;
        }
        
        QString dotColor = "#22c55e"; // Success green
        if (m_lastChatStatus != QString::fromUtf8(_S("正常")) && m_lastChatStatus != QString::fromUtf8(_S("已连接")) && m_lastChatStatus != QString::fromUtf8(_S("消息已发送"))) {
            dotColor = "#f59e0b"; // Warning amber
        }
        
        QString online = m_chatOnlineCount > 0
            ? QString::fromUtf8(_S("最近活跃 %1 人")).arg(m_chatOnlineCount)
            : QString::fromUtf8(_S("同步中..."));
            
        m_communityState->setText(QString::fromUtf8(_S("<span style='color: %1;'>●</span> 连接: %2 · %3"))
            .arg(dotColor, m_lastChatStatus, online));
    }

    QWidget* buildCardSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("卡密激活"))));

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(10);

        m_cardInput = new QLineEdit();
        m_cardInput->setPlaceholderText(QString::fromUtf8(_S("输入卡密，如 XXXX-XXXX-XXXX")));
        m_cardInput->setFixedHeight(38);
        m_cardInput->setStyleSheet(
            "QLineEdit { font-size: 13px; font-family: \"Cascadia Code\", \"Consolas\", monospace; "
            "letter-spacing: 0.5px; border-radius: 8px; }");
        installThemedLineEditMenu(m_cardInput);
        row->addWidget(m_cardInput, 1);

        m_activateBtn = new AnimatedButton(QString::fromUtf8(_S("激活")), AnimatedButton::PrimaryStyle);
        m_activateBtn->setFixedSize(72, 38);
        connect(m_activateBtn, &QPushButton::clicked, this, &MainWindow::onActivate);
        row->addWidget(m_activateBtn);

        o->addLayout(row);
        return c;
    }

    QWidget* buildSessionStatusSection() {
        QFrame* s = card();
        s->setObjectName("sessionStatus");
        QVBoxLayout* o = new QVBoxLayout(s);
        o->setContentsMargins(18, 12, 18, 12);
        o->setSpacing(0);

        m_cardExpiry = new QLabel();
        m_cardExpiry->setProperty("role", "warning");
        m_cardExpiry->setStyleSheet("font-size: 11px; background: transparent; color: #f59e0b; font-weight: 600;");
        m_cardExpiry->setVisible(false);

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(10);
        m_status = new InlineStatus();
        row->addWidget(m_status, 1);

        m_diagnosticsBtn = new AnimatedButton(QString::fromUtf8(_S("诊断")), AnimatedButton::GhostStyle);
        m_diagnosticsBtn->setFixedSize(64, 28);
        m_diagnosticsBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(6), 8, 4));
        connect(m_diagnosticsBtn, &QPushButton::clicked, this, &MainWindow::showDiagnostics);
        row->addWidget(m_diagnosticsBtn);
        o->addLayout(row);
        return s;
    }

    QWidget* buildTargetSection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("目标程序"))));

        QHBoxLayout* row = new QHBoxLayout();
        row->setContentsMargins(0, 0, 0, 0);
        row->setSpacing(10);

        m_targetInput = new QLineEdit();
        m_targetInput->setPlaceholderText(QString::fromUtf8(_S("选择或拖拽 .exe 文件")));
        m_targetInput->setFixedHeight(38);
        m_targetInput->setReadOnly(true);
        m_targetInput->setAcceptDrops(true);
        installThemedLineEditMenu(m_targetInput);
        row->addWidget(m_targetInput, 1);

        m_browseBtn = new AnimatedButton(QString::fromUtf8(_S("浏览")), AnimatedButton::GhostStyle);
        m_browseBtn->setFixedSize(64, 38);
        m_browseBtn->setEnabled(false);
        connect(m_browseBtn, &QPushButton::clicked, this, &MainWindow::onBrowse);
        row->addWidget(m_browseBtn);

        o->addLayout(row);
        return c;
    }

    QWidget* buildApiKeySection() {
        QFrame* c = card();
        QVBoxLayout* o = new QVBoxLayout(c);
        o->setContentsMargins(18, 16, 18, 16);
        o->setSpacing(12);
        o->addWidget(heading(QString::fromUtf8(_S("API 密钥"))));

        m_apiKeyInput = new QLineEdit();
        m_apiKeyInput->setPlaceholderText(QString::fromUtf8(_S("输入 DeepSeek API Key")));
        m_apiKeyInput->setFixedHeight(38);
        m_apiKeyInput->setEchoMode(QLineEdit::Password);
        m_apiKeyInput->setEnabled(false);
        installThemedLineEditMenu(m_apiKeyInput);
        o->addWidget(m_apiKeyInput);

        m_balanceLabel = new QLabel();
        m_balanceLabel->setStyleSheet(
            "font-size: 11px; padding: 4px 2px; background: transparent; color: #7ec8e3; font-weight: 500;");
        m_balanceLabel->setVisible(false);
        o->addWidget(m_balanceLabel);

        return c;
    }

    void buildAnnouncement() {
        m_announcement = new QWidget();
        m_announcement->setVisible(false);
        m_announcement->setStyleSheet(
            "#banner { background: rgba(251,191,58,0.12); "
            "border: 1px solid rgba(251,191,58,0.25); border-radius: 10px; }");
        m_announcement->setObjectName("banner");

        QHBoxLayout* lay = new QHBoxLayout(m_announcement);
        lay->setContentsMargins(12, 7, 12, 7);
        lay->setSpacing(8);
        m_announceLabel = new QLabel();
        m_announceLabel->setWordWrap(true);
        m_announceLabel->setStyleSheet("font-size: 11px; color: #fbbf3a; background: transparent; font-weight: 500;");
        lay->addWidget(m_announceLabel, 1);
    }

    void buildUpdateBanner() {
        m_updateBanner = new QWidget();
        m_updateBanner->setVisible(false);
        m_updateBanner->setStyleSheet(
            "#updateBanner { background: rgba(74,158,255,0.12); "
            "border: 1px solid rgba(74,158,255,0.25); border-radius: 10px; }");
        m_updateBanner->setObjectName("updateBanner");

        QHBoxLayout* lay = new QHBoxLayout(m_updateBanner);
        lay->setContentsMargins(12, 7, 12, 7);
        lay->setSpacing(8);
        m_updateLabel = new QLabel();
        m_updateLabel->setWordWrap(true);
        m_updateLabel->setStyleSheet("font-size: 11px; color: #4a9eff; background: transparent; font-weight: 500;");
        lay->addWidget(m_updateLabel, 1);

        // Auto-update button
        m_updateNowBtn = new AnimatedButton(QString::fromUtf8(_S("立即更新")), AnimatedButton::PrimaryStyle);
        m_updateNowBtn->setStyleSheet(buttonStyle(ButtonAccent, spx(11), spx(4)));
        m_updateNowBtn->setVisible(false);
        connect(m_updateNowBtn, &QPushButton::clicked, this, [this]() {
            if (m_pendingUpdateUrl.isEmpty()) return;
            QMessageBox* dlg = createThemedProgressBox(QString::fromUtf8(_S("正在更新")),
                m_pendingUpdateInstaller
                    ? QString::fromUtf8(_S("正在下载安装包 v%1...\n请稍候。")).arg(m_pendingUpdateVersion)
                    : QString::fromUtf8(_S("正在下载安装包 v%1...\n请稍候。")).arg(m_pendingUpdateVersion));
            dlg->show();
            QApplication::processEvents();
            if (m_pendingUpdateInstaller)
                ApplyInstallerUpdate(m_pendingUpdateVersion, m_pendingUpdateUrl,
                                     m_pendingUpdateSha256, m_pendingUpdatePackageKind,
                                     m_pendingUpdateReleaseID, dlg);
            else
                ApplyInstallerUpdate(m_pendingUpdateVersion, m_pendingUpdateUrl,
                                     m_pendingUpdateSha256, m_pendingUpdatePackageKind,
                                     QString(), dlg);
        });
        lay->addWidget(m_updateNowBtn);

        m_remindLaterBtn = new AnimatedButton(QString::fromUtf8(_S("稍后提醒")), AnimatedButton::GhostStyle);
        m_remindLaterBtn->setStyleSheet(buttonStyle(ButtonNeutral, spx(11), spx(4)));
        connect(m_remindLaterBtn, &QPushButton::clicked, this, [this]() {
            m_updateDismissed = true;
            m_updateBanner->setVisible(false);
        });
        lay->addWidget(m_remindLaterBtn);
    }

    void applyForceUpdateBlock(const QString& latest, const QString& url, const QString& sha256 = QString()) {
        m_forceUpdateBlocked = true;
        m_injectBtn->setEnabled(false);
        m_remindLaterBtn->setVisible(false);

        // Auto-download installer. Do not replace the running executable in-place.
        if (!url.isEmpty()) {
            QMessageBox* dlg = createThemedProgressBox(QString::fromUtf8(_S("需要更新")),
                QString::fromUtf8(_S("发现新版本 v%1，正在下载安装包...\n下载完成后请确认安装。")).arg(latest));
            dlg->show();
            QApplication::processEvents();
            QString packageKind = url.endsWith(".msi", Qt::CaseInsensitive) ? "msi" : "bundle";
            ApplyInstallerUpdate(latest, url, sha256, packageKind, QString(), dlg);
        } else {
            QMessageBox dlg(this);
            dlg.setWindowTitle(QString::fromUtf8(_S("需要更新")));
            dlg.setIcon(QMessageBox::Warning);
            dlg.setText(QString::fromUtf8(_S("当前版本过低，必须更新到 v%1 才能继续使用。\n请联系管理员上传新版本。")).arg(latest));
            dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
            dlg.setStyleSheet(POPUP_CSS);
            dlg.exec();
            close();
        }
    }

    void applyForceInstallerUpdateBlock(const QString& latest, const QString& url,
                                        const QString& sha256, const QString& packageKind,
                                        const QString& releaseID) {
        m_forceUpdateBlocked = true;
        m_injectBtn->setEnabled(false);
        m_remindLaterBtn->setVisible(false);
        if (url.isEmpty()) {
            setStatus("error", QString::fromUtf8(_S("当前版本过低，但服务器未提供安装包")));
            return;
        }
        QMessageBox* dlg = createThemedProgressBox(QString::fromUtf8(_S("需要更新")),
            QString::fromUtf8(_S("发现新版本 v%1，正在下载安装包...\n下载完成后请确认安装。")).arg(latest));
        dlg->show();
        QApplication::processEvents();
        ApplyInstallerUpdate(latest, url, sha256, packageKind, releaseID, dlg);
    }
