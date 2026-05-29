#pragma once
// ============================================================================
// AnimatedButton — QPushButton with natural click feedback
//
//   1. Press shrink  — button geometry gently contracts on press
//   2. Release bounce — overshoots then settles back via spring curve
//   3. Ripple wave   — soft expanding circle from click point
//   4. Hover tint    — subtle background brightness shift
// ============================================================================

#include <QPushButton>
#include <QPropertyAnimation>
#include <QSequentialAnimationGroup>
#include <QPainter>
#include <QMouseEvent>
#include <QEasingCurve>
#include <QVector>
#include <QColor>
#include <QRect>

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
            startShrink();
        }
        QPushButton::mousePressEvent(e);
    }

    void mouseReleaseEvent(QMouseEvent* e) override {
        if (e->button() == Qt::LeftButton && isEnabled()) {
            startBounce();
        }
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
    bool m_animating = false;

    void initStyle() {
        switch (m_style) {
        case PrimaryStyle:
            setStyleSheet(
                "QPushButton { background: rgba(40,60,100,0.45); "
                "border: 1px solid rgba(74,158,255,0.25); color: #d8e4f8; "
                "font-weight: 600; letter-spacing: 0.5px; border-radius: 8px; padding: 9px 18px; }"
                "QPushButton:hover { background: rgba(50,75,120,0.55); "
                "border-color: rgba(74,158,255,0.40); }"
                "QPushButton:pressed { background: rgba(30,45,80,0.55); "
                "border-color: rgba(74,158,255,0.20); }"
                "QPushButton:disabled { background: rgba(30,40,65,0.30); "
                "border-color: rgba(60,80,110,0.15); color: rgba(216,228,248,0.35); }");
            break;
        case GhostStyle:
            setStyleSheet(
                "QPushButton { background: transparent; "
                "border: 1px solid rgba(80,100,140,0.30); border-radius: 8px; padding: 9px 18px; }"
                "QPushButton:hover { background: rgba(30,40,60,0.45); "
                "border-color: rgba(100,120,160,0.45); }"
                "QPushButton:pressed { background: rgba(18,24,36,0.55); }"
                "QPushButton:disabled { color: #6b7a95; border-color: rgba(60,80,110,0.20); }");
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

    // --- geometry-based press/bounce (no pixmap, no effects) ---

    QRect baseRect() const {
        // The "intended" rect when not animating
        QWidget* p = parentWidget();
        if (!p) return geometry();
        // We store base rect in a property on first shrink
        return property("_ab_base").toRect();
    }

    void storeBase() {
        if (property("_ab_base").isNull())
            setProperty("_ab_base", geometry());
    }

    void startShrink() {
        if (m_animating) return;
        m_animating = true;
        storeBase();

        QRect r = property("_ab_base").toRect();
        int dx = (int)(r.width()  * 0.03);
        int dy = (int)(r.height() * 0.03);
        QRect shrunk(r.x() + dx, r.y() + dy, r.width() - dx * 2, r.height() - dy * 2);

        auto* anim = new QPropertyAnimation(this, "geometry", this);
        anim->setDuration(80);
        anim->setStartValue(r);
        anim->setEndValue(shrunk);
        anim->setEasingCurve(QEasingCurve::InQuad);
        anim->start(QAbstractAnimation::DeleteWhenStopped);
    }

    void startBounce() {
        QRect base = property("_ab_base").toRect();
        QRect cur  = geometry();
        m_animating = false;

        int dx = (int)(base.width()  * 0.015);
        int dy = (int)(base.height() * 0.015);
        QRect expanded(base.x() - dx, base.y() - dy,
                       base.width() + dx * 2, base.height() + dy * 2);

        auto* seq = new QSequentialAnimationGroup(this);

        auto* grow = new QPropertyAnimation(this, "geometry", seq);
        grow->setDuration(120);
        grow->setStartValue(cur);
        grow->setEndValue(expanded);
        grow->setEasingCurve(QEasingCurve::OutBack);

        auto* settle = new QPropertyAnimation(this, "geometry", seq);
        settle->setDuration(100);
        settle->setStartValue(expanded);
        settle->setEndValue(base);
        settle->setEasingCurve(QEasingCurve::OutQuad);

        seq->addAnimation(grow);
        seq->addAnimation(settle);
        connect(seq, &QSequentialAnimationGroup::finished, this, [this]() {
            m_animating = false;
        });
        seq->start(QAbstractAnimation::DeleteWhenStopped);
    }
};
