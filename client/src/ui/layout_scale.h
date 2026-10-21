#pragma once
// UI scale math and global stylesheet. Widget-level sizing stays in MainWindow.

#include <QtGlobal>
#include <QString>

inline double ComputeUiScale(int width, int height, int baseW, int baseH) {
    double sx = (double)width / (double)baseW;
    double sy = (double)height / (double)baseH;
    return qBound(1.0, qMin(sx, sy), 1.8);
}

inline int SpX(double scale, double base) {
    double s = scale > 0.0 ? scale : 1.0;
    return qMax(1, (int)qRound(base * s));
}

// Global stylesheet layered on top of theme.h CSS.
inline QString BuildScaledMainWindowCssheet(double scale) {
    auto spx = [scale](double base) { return SpX(scale, base); };
    return QString(
        "#MainWindow { background: transparent; border: 1px solid rgba(200, 200, 210, 0.40); border-radius: %1px; }"
        "* { font-size: %2px; }"
        "QLabel[role=\"heading\"] { font-size: %3px; font-weight: 600; color: #334155; letter-spacing: 0.6px; }"
        "QLabel[role=\"caption\"], QLabel[role=\"success\"], QLabel[role=\"danger\"], QLabel[role=\"warning\"] { font-size: %3px; }"
        "QLineEdit { font-size: %4px; border-radius: %5px; padding: %6px %7px; }"
        "QComboBox { font-size: %4px; border-radius: %5px; padding: %9px %10px %9px %7px; }"
        "QComboBox::drop-down { width: %11px; border-top-right-radius: %5px; border-bottom-right-radius: %5px; }"
        "QComboBox QAbstractItemView::item { min-height: %12px; padding: %13px %14px; }"
        "QPushButton { font-size: %2px; border-radius: %5px; padding: %6px %8px; }")
        .arg(spx(16)).arg(spx(12)).arg(spx(11)).arg(spx(13))
        .arg(spx(8)).arg(spx(9)).arg(spx(12)).arg(spx(18))
        .arg(spx(8)).arg(spx(34)).arg(spx(30)).arg(spx(26))
        .arg(spx(5)).arg(spx(10));
}
