#pragma once
// Home-page control styles applied when UI scale changes.

#include <QString>

#include "layout_scale.h"

inline QString CardInputMonospaceCss(double scale) {
    return QString(
        "QLineEdit { font-size: %1px; font-family: \"Cascadia Code\", \"Consolas\", monospace; "
        "letter-spacing: 0.5px; border-radius: %2px; }")
        .arg(SpX(scale, 13)).arg(SpX(scale, 8));
}

inline QString HistoryLabelCss(double scale) {
    return QString(
        "font-size: %1px; color: #7ec8e3; background: transparent;")
        .arg(SpX(scale, 11));
}

inline QString AnnounceLabelCss(double scale) {
    return QString(
        "font-size: %1px; color: #fbbf3a; background: transparent; font-weight: 500;")
        .arg(SpX(scale, 11));
}

inline QString UpdateLabelCss(double scale) {
    return QString(
        "font-size: %1px; color: #4a9eff; background: transparent; font-weight: 500;")
        .arg(SpX(scale, 11));
}

inline QString CommunityStateCss(double scale) {
    return QString(
        "font-size: %1px; color: #475569; font-weight: 600;")
        .arg(SpX(scale, 12));
}
