#pragma once
// Frameless window hit-testing helpers (WM_NCHITTEST).

#include <functional>

#include <QWidget>
#include <QPushButton>
#include <QLineEdit>
#include <QScrollBar>
#include <QComboBox>
#include <QMenu>
#include <QLabel>
#include <QAbstractScrollArea>

#include <windows.h>

inline bool IsInteractiveChromeWidget(QWidget* w, const QWidget* root) {
    if (!w) return false;
    while (w && w != root) {
        if (qobject_cast<QPushButton*>(w) ||
            qobject_cast<QLineEdit*>(w) ||
            qobject_cast<QScrollBar*>(w) ||
            qobject_cast<QComboBox*>(w) ||
            qobject_cast<QMenu*>(w)) {
            return true;
        }
        if (qobject_cast<QAbstractScrollArea*>(w)) return true;
        if (auto* lbl = qobject_cast<QLabel*>(w)) {
            if (lbl->openExternalLinks() || lbl->textFormat() == Qt::RichText) return true;
        }
        w = w->parentWidget();
    }
    return false;
}

// Returns HT* code when the point should resize/drag; 0 = defer to Qt.
inline LRESULT FramelessHitTest(
    const QPoint& localPos,
    const QRect& rect,
    bool maximized,
    int titleBarH,
    int resizeBorder,
    std::function<QWidget*(const QPoint&)> childAt,
    const QWidget* root) {
    if (!rect.contains(localPos)) return 0;

    const bool left = localPos.x() <= resizeBorder;
    const bool right = localPos.x() >= rect.width() - resizeBorder;
    const bool top = localPos.y() <= resizeBorder;
    const bool bottom = localPos.y() >= rect.height() - resizeBorder;

    if (!maximized) {
        if (top && left) return HTTOPLEFT;
        if (top && right) return HTTOPRIGHT;
        if (bottom && left) return HTBOTTOMLEFT;
        if (bottom && right) return HTBOTTOMRIGHT;
        if (left) return HTLEFT;
        if (right) return HTRIGHT;
        if (top) return HTTOP;
        if (bottom) return HTBOTTOM;
    }

    if (localPos.y() < titleBarH) {
        QWidget* hit = childAt ? childAt(localPos) : nullptr;
        if (!IsInteractiveChromeWidget(hit, root)) return HTCAPTION;
    }
    return 0;
}
