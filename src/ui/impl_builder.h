    void applyShadow() {
        HWND hwnd = (HWND)winId();
        MARGINS m = {0, 0, 0, 1};
        DwmExtendFrameIntoClientArea(hwnd, &m);
        BOOL dark = TRUE;
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));
    }

    void applyAcrylic() {
        HWND hwnd = (HWND)winId();
        HMODULE hUser = GetModuleHandleW(L"user32.dll");
        if (hUser) {
            auto pSWCA = (pfnSetWindowCompositionAttribute)
                GetProcAddress(hUser, "SetWindowCompositionAttribute");
            if (pSWCA) {
                // AccentState=4 (ACRYLIC), GradientColor=0xA0FFFFFF (white tint, ABGR format)
                ACCENT_POLICY policy = {4, 0, 0xA0FFFFFF, 0};
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
        if (force) {
            applyForceUpdateBlock(latest, url);
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

        // Body
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
        bl->addWidget(buildExpiryLabel());
        bl->addSpacing(4);

        m_status = new InlineStatus();
        bl->addWidget(m_status);
        bl->addWidget(sep());

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
        root->addWidget(scroll, 1);

        setUiLocked(true);
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
        row->addWidget(m_cardInput, 1);

        m_activateBtn = new AnimatedButton(QString::fromUtf8(_S("激活")), AnimatedButton::PrimaryStyle);
        m_activateBtn->setFixedSize(72, 38);
        connect(m_activateBtn, &QPushButton::clicked, this, &MainWindow::onActivate);
        row->addWidget(m_activateBtn);

        o->addLayout(row);
        return c;
    }

    QWidget* buildExpiryLabel() {
        m_cardExpiry = new QLabel();
        m_cardExpiry->setProperty("role", "warning");
        m_cardExpiry->setStyleSheet("font-size: 11px; padding-left: 2px; background: transparent; color: #fbbf3a;");
        m_cardExpiry->setVisible(false);
        return m_cardExpiry;
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
        m_updateNowBtn->setStyleSheet(
            "QPushButton { font-size: 11px; color: #d8e4f8; background: rgba(40,60,100,0.45); "
            "border: 1px solid rgba(74,158,255,0.25); border-radius: 4px; padding: 3px 12px; font-weight: 600; }"
            "QPushButton:hover { background: rgba(50,75,120,0.55); "
            "border-color: rgba(74,158,255,0.40); }");
        m_updateNowBtn->setVisible(false);
        connect(m_updateNowBtn, &QPushButton::clicked, this, [this]() {
            if (!m_pendingUpdateUrl.isEmpty())
                ApplyUpdate(m_pendingUpdateVersion, m_pendingUpdateUrl, m_pendingUpdateSha256);
        });
        lay->addWidget(m_updateNowBtn);

        m_remindLaterBtn = new AnimatedButton(QString::fromUtf8(_S("稍后提醒")), AnimatedButton::GhostStyle);
        m_remindLaterBtn->setStyleSheet(
            "QPushButton { font-size: 11px; color: #a0aec5; background: transparent; "
            "border: 1px solid rgba(80, 100, 140, 0.30); border-radius: 4px; padding: 2px 8px; }"
            "QPushButton:hover { color: #f0f4fa; border-color: rgba(100, 120, 160, 0.50); }");
        connect(m_remindLaterBtn, &QPushButton::clicked, this, [this]() {
            m_updateDismissed = true;
            m_updateBanner->setVisible(false);
        });
        lay->addWidget(m_remindLaterBtn);
    }

    void applyForceUpdateBlock(const QString& latest, const QString& url) {
        m_forceUpdateBlocked = true;
        m_injectBtn->setEnabled(false);
        m_remindLaterBtn->setVisible(false);

        // Auto-download and replace
        if (!url.isEmpty()) {
            QMessageBox* dlg = new QMessageBox(this);
            dlg->setWindowTitle(QString::fromUtf8(_S("需要更新")));
            dlg->setIcon(QMessageBox::Information);
            dlg->setText(QString::fromUtf8(_S("发现新版本 v%1，正在自动下载更新...\n下载完成后将自动替换并重启。")).arg(latest));
            dlg->setStandardButtons(QMessageBox::NoButton);
            dlg->setAttribute(Qt::WA_DeleteOnClose);
            dlg->show();
            QApplication::processEvents();
            ApplyUpdate(latest, url, QString(), dlg);
        } else {
            QMessageBox dlg(this);
            dlg.setWindowTitle(QString::fromUtf8(_S("需要更新")));
            dlg.setIcon(QMessageBox::Warning);
            dlg.setText(QString::fromUtf8(_S("当前版本过低，必须更新到 v%1 才能继续使用。\n请联系管理员上传新版本。")).arg(latest));
            dlg.setWindowFlags(dlg.windowFlags() & ~Qt::WindowContextHelpButtonHint);
            dlg.exec();
            close();
        }
    }

