#pragma once
// Button stylesheet builders. Padding values should already be DPI-scaled.

#include <QString>

enum class UiButtonTone {
    Neutral,
    Accent,
    Success
};

inline QString BuildButtonStyle(
    UiButtonTone tone,
    int fontPx,
    int radiusPx,
    int padY,
    int padX) {
    if (tone == UiButtonTone::Accent) {
        return QString(
            "QPushButton { background: rgba(74,158,255,0.85); "
            "border: 1px solid rgba(74,158,255,0.60); color: #ffffff; "
            "font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
            "border-radius: %2px; padding: %3px %4px; }"
            "QPushButton:hover { background: rgba(109,179,255,0.90); "
            "border-color: rgba(74,158,255,0.80); }"
            "QPushButton:pressed { background: rgba(59,130,246,0.90); "
            "border-color: rgba(74,158,255,0.70); }"
            "QPushButton:disabled { background: rgba(74,158,255,0.40); "
            "border-color: rgba(74,158,255,0.30); color: rgba(255,255,255,0.55); }")
            .arg(fontPx).arg(radiusPx).arg(padY).arg(padX);
    }
    if (tone == UiButtonTone::Success) {
        return QString(
            "QPushButton { background: #34d27b; border: 1px solid #34d27b; "
            "color: #ffffff; font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
            "border-radius: %2px; padding: %3px %4px; }"
            "QPushButton:hover { background: #3ae085; border-color: #3ae085; }"
            "QPushButton:pressed { background: #22c55e; border-color: #22c55e; }"
            "QPushButton:disabled { background: rgba(52,210,123,0.40); "
            "border-color: rgba(52,210,123,0.30); color: rgba(255,255,255,0.55); }")
            .arg(fontPx).arg(radiusPx).arg(padY).arg(padX);
    }
    return QString(
        "QPushButton { background: transparent; "
        "border: 1px solid rgba(200,200,210,0.50); color: #1a1a2e; "
        "font-size: %1px; font-weight: 600; letter-spacing: 0.5px; "
        "border-radius: %2px; padding: %3px %4px; }"
        "QPushButton:hover { background: rgba(255,255,255,0.38); "
        "border-color: rgba(180,180,190,0.65); }"
        "QPushButton:pressed { background: rgba(235,235,242,0.42); }"
        "QPushButton:disabled { background: rgba(255,255,255,0.14); "
        "border-color: rgba(220,220,230,0.26); color: #94a3b8; }")
        .arg(fontPx).arg(radiusPx).arg(padY).arg(padX);
}
