#pragma once
// ============================================================================
// InlineStatus — status indicator with colored dot and text
// ============================================================================
#include <QWidget>
#include <QHBoxLayout>
#include <QLabel>
#include <QStyle>
#include "theme.h"
#include <QVariant>

class InlineStatus : public QWidget {
    Q_OBJECT
public:
    explicit InlineStatus(QWidget* parent = nullptr) : QWidget(parent) {
        setFixedHeight(26);
        setStyleSheet("background: transparent;");
        QHBoxLayout* lay = new QHBoxLayout(this);
        lay->setContentsMargins(0, 0, 0, 0);
        lay->setSpacing(8);
        lay->addStretch();

        m_expiry = new QLabel();
        m_expiry->setProperty("role", "caption");
        m_expiry->setStyleSheet("background: transparent;");
        m_expiry->setAlignment(Qt::AlignCenter);
        m_expiry->setVisible(false);
        lay->addWidget(m_expiry);

        m_dot = new QLabel();
        m_dot->setFixedSize(6, 6);
        m_dot->setStyleSheet("background: #6b7a95; border-radius: 3px;");
        lay->addWidget(m_dot);

        m_text = new QLabel(QString::fromUtf8("\xe8\xaf\xb7\xe8\xbe\x93\xe5\x85\xa5\xe5\x8d\xa1\xe5\xaf\x86\xe5\xb9\xb6\xe7\x82\xb9\xe5\x87\xbb\xe6\xbf\x80\xe6\xb4\xbb"));
        m_text->setProperty("role", "caption");
        m_text->setStyleSheet("background: transparent;");
        m_text->setAlignment(Qt::AlignCenter);
        lay->addWidget(m_text);
        lay->addStretch();
    }

    void setState(const QString& state, const QString& msg) {
        m_text->setText(msg);
        QString dc = "#6b7a95", role = "caption";
        if (state == "ok")    { dc = "#34d27b"; role = "success"; }
        if (state == "error") { dc = "#f05c5c"; role = "danger"; }
        if (state == "warn")  { dc = "#fbbf3a"; role = "warning"; }
        if (state == "idle")  { dc = "#7ec8e3"; role = "caption"; }
        if (state == "info")  { dc = "#7ec8e3"; role = "caption"; }
        m_dot->setStyleSheet(QString("background: %1; border-radius: 3px;").arg(dc));
        m_text->setProperty("role", role);
        m_text->style()->unpolish(m_text);
        m_text->style()->polish(m_text);
    }

    void setExpiryText(const QString& text, const QString& tone = QStringLiteral("normal")) {
        m_expiry->setText(text);
        m_expiry->setVisible(!text.isEmpty());
        QString color = "#64748b";
        if (tone == "warn") color = "#f59e0b";
        if (tone == "danger") color = "#ef4444";
        m_expiry->setStyleSheet(QString(
            "background: transparent; color: %1; font-weight: 600;").arg(color));
    }

    void clearExpiryText() {
        setExpiryText(QString());
    }

private:
    QLabel* m_expiry;
    QLabel* m_dot;
    QLabel* m_text;
};
