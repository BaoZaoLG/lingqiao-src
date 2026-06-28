#pragma once
// ============================================================================
// AnimatedButton — QPushButton with natural click feedback
//
//   1. Ripple wave   — soft expanding circle from click point
//   2. Hover/pressed tint from stylesheet
// ============================================================================

#include <QPushButton>
#include <QPainter>
#include <QMouseEvent>
#include <QVector>
#include <QColor>

class AnimatedButton : public QPushButton {
    Q_OBJECT
public:
    enum Style { DefaultStyle, PrimaryStyle, GhostStyle };

    explicit AnimatedButton(const QString& text, Style style = DefaultStyle, QWidget* parent = nullptr)
        : QPushButton(text, parent), m_style(style)
    {
        setCursor(Qt::PointingHandCursor);
        setAttribute(Qt::WA_Hover, true);
        initStyle();
    }

protected:
    void mousePressEvent(QMouseEvent* e) override {
        if (e->button() == Qt::LeftButton && isEnabled()) {
            addRipple(e->pos());
        }
        QPushButton::mousePressEvent(e);
    }

    void mouseReleaseEvent(QMouseEvent* e) override {
        QPushButton::mouseReleaseEvent(e);
    }

    void paintEvent(QPaintEvent* e) override {
        QPushButton::paintEvent(e);

        if (m_ripples.isEmpty()) return;

        QPainter p(this);
        p.setRenderHint(QPainter::Antialiasing);
        p.setPen(Qt::NoPen);

        QColor base = (m_style == PrimaryStyle) ? QColor(255, 255, 255)
                    : QColor(180, 200, 230);

        for (auto it = m_ripples.begin(); it != m_ripples.end(); ) {
            if (it->progress >= 1.0f) {
                it = m_ripples.erase(it);
                continue;
            }
            float fade = 1.0f - it->progress;
            base.setAlpha((int)(80 * fade));
            float radius = qMax(width(), height()) * 0.7f * it->progress;
            p.setBrush(base);
            p.drawEllipse(it->center, radius, radius);
            ++it;
        }
    }

    void timerEvent(QTimerEvent* e) override {
        if (e->timerId() == m_rippleTimer) {
            bool alive = false;
            for (auto& r : m_ripples) {
                r.progress += 0.04f;
                if (r.progress < 1.0f) alive = true;
            }
            update();
            if (!alive) {
                killTimer(m_rippleTimer);
                m_rippleTimer = 0;
            }
        }
        QPushButton::timerEvent(e);
    }

private:
    struct Ripple { QPointF center; float progress = 0.0f; };

    Style m_style;
    QVector<Ripple> m_ripples;
    int m_rippleTimer = 0;

    void initStyle() {
        switch (m_style) {
        case PrimaryStyle:
            setStyleSheet(
                "QPushButton { background: rgba(74,158,255,0.85); "
                "border: 1px solid rgba(74,158,255,0.60); color: #ffffff; "
                "font-weight: 600; letter-spacing: 0.5px; border-radius: 8px; padding: 9px 18px; }"
                "QPushButton:hover { background: rgba(109,179,255,0.90); "
                "border-color: rgba(74,158,255,0.80); }"
                "QPushButton:pressed { background: rgba(59,130,246,0.90); "
                "border-color: rgba(74,158,255,0.70); }"
                "QPushButton:disabled { background: rgba(74,158,255,0.40); "
                "border-color: rgba(74,158,255,0.30); color: rgba(255,255,255,0.55); }");
            break;
        case GhostStyle:
            setStyleSheet(
                "QPushButton { background: transparent; "
                "border: 1px solid rgba(200,200,210,0.50); border-radius: 8px; padding: 9px 18px; color: #1a1a2e; }"
                "QPushButton:hover { background: rgba(240,240,245,0.60); "
                "border-color: rgba(180,180,190,0.60); }"
                "QPushButton:pressed { background: rgba(230,230,240,0.90); }"
                "QPushButton:disabled { color: #94a3b8; border-color: rgba(220,220,230,0.30); }");
            break;
        default:
            break;
        }
    }

    void addRipple(const QPoint& pos) {
        m_ripples.append({QPointF(pos), 0.0f});
        if (m_rippleTimer == 0)
            m_rippleTimer = startTimer(16);
    }

};
