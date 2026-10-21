#pragma once
// Themed popup menus / combo dropdowns.

#include <QAction>
#include <QApplication>
#include <QClipboard>
#include <QLineEdit>
#include <QComboBox>
#include <QListView>
#include <QMenu>
#include <QObject>
#include <QPoint>
#include <QTextBrowser>
#include <QTextEdit>

#include "theme.h"

inline void InstallThemedLineEditMenu(QObject* context, QLineEdit* edit) {
    if (!edit) return;
    edit->setContextMenuPolicy(Qt::CustomContextMenu);
    QObject::connect(edit, &QLineEdit::customContextMenuRequested, context,
        [edit](const QPoint& pos) {
            QMenu* menu = edit->createStandardContextMenu();
            if (!menu) return;
            menu->setAttribute(Qt::WA_DeleteOnClose);
            menu->setStyleSheet(POPUP_CSS);
            menu->popup(edit->mapToGlobal(pos));
        });
}

/* Right-click on combo body / its internal line edit. */
inline void InstallThemedComboBoxMenu(QObject* context, QComboBox* combo) {
    if (!combo) return;
    combo->setContextMenuPolicy(Qt::CustomContextMenu);
    QObject::connect(combo, &QComboBox::customContextMenuRequested, context,
        [combo](const QPoint& pos) {
            QMenu* menu = nullptr;
            if (combo->lineEdit()) {
                menu = combo->lineEdit()->createStandardContextMenu();
            } else {
                menu = new QMenu(combo);
                QAction* copy = menu->addAction(QObject::tr("Copy"));
                QObject::connect(copy, &QAction::triggered, combo, [combo]() {
                    if (auto* le = combo->lineEdit()) {
                        le->copy();
                    } else if (!combo->currentText().isEmpty()) {
                        QApplication::clipboard()->setText(combo->currentText());
                    }
                });
            }
            if (!menu) return;
            menu->setAttribute(Qt::WA_DeleteOnClose);
            menu->setStyleSheet(POPUP_CSS);
            menu->popup(combo->mapToGlobal(pos));
        });
    if (combo->lineEdit()) {
        InstallThemedLineEditMenu(context, combo->lineEdit());
    }
}

/* Right-click in chat message browser (copy / select-all). */
inline void InstallThemedTextBrowserMenu(QObject* context, QTextBrowser* browser) {
    if (!browser) return;
    browser->setContextMenuPolicy(Qt::CustomContextMenu);
    QObject::connect(browser, &QTextBrowser::customContextMenuRequested, context,
        [browser](const QPoint& pos) {
            QMenu* menu = browser->createStandardContextMenu();
            if (!menu) return;
            menu->setAttribute(Qt::WA_DeleteOnClose);
            menu->setStyleSheet(POPUP_CSS);
            menu->popup(browser->viewport()->mapToGlobal(pos));
        });
}

inline void InstallThemedComboBox(QComboBox* combo) {
    if (!combo) return;
    combo->setView(new QListView(combo));
    combo->view()->setStyleSheet(
        "QListView {"
        " background: rgba(255, 255, 255, 0.98);"
        " border: 1px solid rgba(200, 200, 210, 0.60);"
        " color: #1a1a2e;"
        " outline: none;"
        " padding: 4px;"
        " selection-background-color: #4a9eff;"
        " selection-color: #ffffff;"
        "}"
        "QListView::item {"
        " min-height: 26px;"
        " padding: 5px 10px;"
        " color: #1a1a2e;"
        " background: rgba(255, 255, 255, 0.98);"
        "}"
        "QListView::item:hover {"
        " background: rgba(240, 240, 245, 0.95);"
        " color: #0f172a;"
        "}"
        "QListView::item:selected {"
        " background: #4a9eff;"
        " color: #ffffff;"
        "}"
    );
}
