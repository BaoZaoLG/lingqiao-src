#pragma once
// Chat-area control styles applied when UI scale changes.

#include <QString>
#include <QTextBrowser>
#include <QWidget>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>

#include "layout_scale.h"

inline QString ChatViewCss(double scale) {
    return QString(
        "QTextBrowser { background: rgba(255, 255, 255, 0.35); border: 1px solid rgba(200, 200, 210, 0.40); "
        "border-radius: %1px; padding: %2px; color: #1a1a2e; font-size: %3px; }")
        .arg(SpX(scale, 8)).arg(SpX(scale, 10)).arg(SpX(scale, 12));
}

inline QString ReplyBannerCss(double scale) {
    return QString(
        "QFrame#replyBanner { background: rgba(74, 158, 255, 0.12); border: 1px solid rgba(74, 158, 255, 0.25); "
        "border-radius: %1px; padding: %2px %3px; }")
        .arg(SpX(scale, 6)).arg(SpX(scale, 4)).arg(SpX(scale, 8));
}

inline QString ChatInputCss(double scale) {
    return QString(
        "QLineEdit { font-size: %1px; border-radius: %2px; padding: %3px %4px; "
        "background: rgba(255, 255, 255, 0.50); border: 1px solid rgba(200, 200, 210, 0.45); color: #0f172a; } "
        "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }")
        .arg(SpX(scale, 12)).arg(SpX(scale, 8)).arg(SpX(scale, 6)).arg(SpX(scale, 10));
}

inline QString ChatSendCss(double scale) {
    return QString(
        "QPushButton { font-size: %1px; border-radius: %2px; "
        "background: rgba(74, 158, 255, 0.85); border: 1px solid rgba(74, 158, 255, 0.60); "
        "color: #ffffff; font-weight: 600; } "
        "QPushButton:hover { background: rgba(109, 179, 255, 0.90); } "
        "QPushButton:disabled { background: rgba(74, 158, 255, 0.40); color: rgba(255,255,255,0.5); }")
        .arg(SpX(scale, 12)).arg(SpX(scale, 8));
}

inline QString NicknameInputCss(double scale) {
    return QString(
        "QLineEdit { font-size: %1px; border-radius: %2px; padding: %3px %4px; "
        "background: rgba(255, 255, 255, 0.45); border: 1px solid rgba(200, 200, 210, 0.45); color: #0f172a; } "
        "QLineEdit:focus { border-color: #4a9eff; background: #ffffff; }")
        .arg(SpX(scale, 11)).arg(SpX(scale, 6)).arg(SpX(scale, 2)).arg(SpX(scale, 4));
}

struct ChatWidgets {
    QTextBrowser* chatView = nullptr;
    QWidget* replyBanner = nullptr;
    QLabel* replyLabel = nullptr;
    QLineEdit* chatInput = nullptr;
    QLabel* chatHeading = nullptr;
    QPushButton* chatSendBtn = nullptr;
    QLineEdit* nicknameInput = nullptr;
};

// Applies DPI-scaled chat styles. Caller should refresh chat HTML after.
inline void ApplyChatScaleStyles(double scale, const ChatWidgets& w) {
    if (w.chatView) {
        w.chatView->setStyleSheet(ChatViewCss(scale));
    }
    if (w.replyBanner) {
        w.replyBanner->setStyleSheet(ReplyBannerCss(scale));
        if (w.replyLabel) {
            w.replyLabel->setStyleSheet(QString("font-size: %1px; color: #1e40af; font-weight: 500;")
                .arg(SpX(scale, 11)));
        }
        if (QPushButton* cancelBtn = w.replyBanner->findChild<QPushButton*>()) {
            cancelBtn->setFixedSize(SpX(scale, 24), SpX(scale, 24));
            cancelBtn->setStyleSheet(QString(
                "QPushButton { border: none; background: transparent; color: #1e40af; font-weight: bold; "
                "font-size: %1px; margin: 0; padding: 0; } "
                "QPushButton:hover { color: #ef4444; }").arg(SpX(scale, 16)));
        }
    }
    if (w.chatInput) {
        w.chatInput->setFixedHeight(SpX(scale, 34));
        w.chatInput->setStyleSheet(ChatInputCss(scale));
    }
    if (w.chatHeading) {
        w.chatHeading->setStyleSheet(QString("font-size: %1px; font-weight: 700; color: #1e293b;")
            .arg(SpX(scale, 12)));
    }
    if (w.chatSendBtn) {
        w.chatSendBtn->setFixedSize(SpX(scale, 64), SpX(scale, 34));
        w.chatSendBtn->setStyleSheet(ChatSendCss(scale));
    }
    if (w.nicknameInput) {
        w.nicknameInput->setFixedSize(SpX(scale, 100), SpX(scale, 26));
        w.nicknameInput->setStyleSheet(NicknameInputCss(scale));
    }
}
