#pragma once
// ============================================================================
// TitleBar — frameless window title bar with brand, connection dot, controls
// ============================================================================
#include <QWidget>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QPainter>
#include <QPainterPath>
#include <QLinearGradient>
#include <QPaintEvent>
#include <QTimer>
#include "theme.h"
#include "config.h"
#include <QVariant>

// ============================================================================
// GradientLabel — animated gradient text using QPainter
// ============================================================================
class GradientLabel : public QLabel {
    Q_OBJECT
public:
    explicit GradientLabel(QWidget* parent = nullptr) : QLabel(parent) {
        setAttribute(Qt::WA_TranslucentBackground);
        m_animTimer = new QTimer(this);
        connect(m_animTimer, &QTimer::timeout, this, [this]() {
            if (!isVisible()) return;
            m_offset += 0.5f;
            if (m_offset >= 200.0f) m_offset -= 200.0f;
            update();
        });
        m_animTimer->start(60);
    }

protected:
    void paintEvent(QPaintEvent*) override {
        QPainter p(this);
        p.setRenderHint(QPainter::TextAntialiasing);
        p.setFont(font());

        QFontMetrics fm(font());
        QRect cr = contentsRect();
        QRect br = fm.boundingRect(cr, alignment() | Qt::TextSingleLine, text());
        int x = cr.left() + (cr.width() - br.width()) / 2;
        int y = cr.top() + (cr.height() + fm.ascent() - fm.descent()) / 2;

        QPainterPath path;
        path.addText(x, y, font(), text());

        QLinearGradient grad(m_offset, 0, m_offset + 200, 0);
        grad.setSpread(QGradient::RepeatSpread);
        grad.setColorAt(0.00f, QColor(0x3A, 0xB0, 0xFF));
        grad.setColorAt(0.25f, QColor(0x4A, 0x9E, 0xFF));
        grad.setColorAt(0.50f, QColor(0x7B, 0x6E, 0xF0));
        grad.setColorAt(0.75f, QColor(0xA8, 0x5C, 0xF0));
        grad.setColorAt(1.00f, QColor(0x3A, 0xB0, 0xFF));

        p.fillPath(path, QBrush(grad));
    }

private:
    float m_offset = 0.0f;
    QTimer* m_animTimer;
};

// ============================================================================
// MinButton / CloseButton — custom-painted title bar buttons
// ============================================================================
static constexpr int WINDOW_CORNER_RADIUS = 16;

class MinButton : public QPushButton {
    Q_OBJECT
public:
    explicit MinButton(const QString& text, QWidget* parent = nullptr)
        : QPushButton(text, parent) {
        setCursor(Qt::PointingHandCursor);
        setStyleSheet("QPushButton { background: transparent; border: none; }");
    }
protected:
    void paintEvent(QPaintEvent*) override {
        QPainter p(this);
        p.setRenderHint(QPainter::Antialiasing);
        if (isDown())
            p.fillRect(rect(), QColor(200, 215, 235, 50));
        else if (underMouse())
            p.fillRect(rect(), QColor(200, 215, 235, 35));
        p.setFont(font());
        p.setPen(isDown() || underMouse() ? QColor(0xF0, 0xF4, 0xFA)
                                          : QColor(0xA0, 0xAE, 0xC5));
        p.drawText(rect(), Qt::AlignCenter, text());
    }
};

class CloseButton : public QPushButton {
    Q_OBJECT
public:
    explicit CloseButton(const QString& text, QWidget* parent = nullptr)
        : QPushButton(text, parent) {
        setCursor(Qt::PointingHandCursor);
        setStyleSheet("QPushButton { background: transparent; border: none; }");
    }
protected:
    void paintEvent(QPaintEvent*) override {
        QPainter p(this);
        p.setRenderHint(QPainter::Antialiasing);
        QColor bg;
        if (isDown())      bg = QColor(0xC9, 0x3C, 0x41);
        else if (underMouse()) bg = QColor(0xE5, 0x48, 0x4D);
        if (bg.isValid()) {
            qreal r = WINDOW_CORNER_RADIUS;
            qreal w = width(), h = height();
            QPainterPath clip;
            clip.moveTo(0, 0);
            clip.lineTo(w - r, 0);
            clip.arcTo(w - r, 0, r * 2, r * 2, 90, -90);
            clip.lineTo(w, h);
            clip.lineTo(0, h);
            clip.closeSubpath();
            p.setClipPath(clip);
            p.fillRect(0, 0, w, h, bg);
            p.setClipping(false);
        }
        p.setFont(font());
        p.setPen(isDown() || underMouse() ? QColor(0xFF, 0xFF, 0xFF)
                                          : QColor(0xA0, 0xAE, 0xC5));
        p.drawText(rect(), Qt::AlignCenter, text());
    }
};

class TitleBar : public QWidget {
    Q_OBJECT
public:
    explicit TitleBar(QWidget* parent = nullptr) : QWidget(parent) {
        setFixedHeight(TITLE_BAR_H);
        setCursor(Qt::ArrowCursor);
        setStyleSheet("background: transparent;");

        QHBoxLayout* lay = new QHBoxLayout(this);
        lay->setContentsMargins(16, 0, 0, 0);
        lay->setSpacing(0);

        GradientLabel* brand = new GradientLabel();
        brand->setText(QString::fromUtf8("\xe7\x81\xb5\xe6\xa1\xa5"));
        brand->setStyleSheet(
            "font-size: 15px; font-weight: 600; background: transparent; "
            "font-family: \"Microsoft YaHei UI\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif; "
            "letter-spacing: 1px;");
        lay->addWidget(brand);

        QLabel* ver = new QLabel(QLatin1Char('v') + GetClientVersion());
        ver->setStyleSheet(
            "font-size: 10px; font-weight: 600; color: #6b7a95; background: transparent;");
        lay->addWidget(ver);
        lay->addSpacing(8);

        m_dot = new QLabel();
        m_dot->setFixedSize(7, 7);
        m_dot->setStyleSheet("background: #6b7a95; border-radius: 3px;");
        lay->addWidget(m_dot);
        lay->addStretch();

        m_minBtn = new MinButton(QString::fromUtf8("\xe2\x80\x94"));
        m_minBtn->setFixedSize(46, TITLE_BAR_H);
        connect(m_minBtn, &QPushButton::clicked, this, &TitleBar::minClicked);
        lay->addWidget(m_minBtn);

        m_closeBtn = new CloseButton(QString::fromUtf8("\xe2\x9c\x95"));
        m_closeBtn->setFixedSize(46, TITLE_BAR_H);
        connect(m_closeBtn, &QPushButton::clicked, this, &TitleBar::closeClicked);
        lay->addWidget(m_closeBtn);
    }

    void setDot(const QString& state) {
        QString c = "#6b7a95";
        if (state == "ok")    c = "#34d27b";
        if (state == "error") c = "#f05c5c";
        m_dot->setStyleSheet(QString("background: %1; border-radius: 3px;").arg(c));
    }

signals:
    void closeClicked();
    void minClicked();

protected:
    void paintEvent(QPaintEvent*) override {
        QPainter p(this);
        p.fillRect(rect(), QColor(255, 255, 255, 20));
    }

private:
    QLabel*        m_dot;
    MinButton*     m_minBtn;
    CloseButton*   m_closeBtn;
};
