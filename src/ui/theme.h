#pragma once
// ============================================================================
// UI Theme — pure white transparent style
// ============================================================================

namespace Color {
    inline constexpr const char* BG          = "rgba(255, 255, 255, 0.85)";
    inline constexpr const char* SURFACE     = "rgba(255, 255, 255, 0.72)";
    inline constexpr const char* INPUT_BG    = "rgba(255, 255, 255, 0.60)";
    inline constexpr const char* HOVER       = "rgba(240, 240, 245, 0.80)";
    inline constexpr const char* BORDER      = "rgba(200, 200, 210, 0.50)";
    inline constexpr const char* BORDER_FOCUS = "#4a9eff";
    inline constexpr const char* FAINT       = "rgba(220, 220, 230, 0.40)";
    inline constexpr const char* ACCENT      = "#4a9eff";
    inline constexpr const char* ACCENT_HOVER = "#6db3ff";
    inline constexpr const char* SUCCESS     = "#22c55e";
    inline constexpr const char* WARNING     = "#f59e0b";
    inline constexpr const char* DANGER      = "#ef4444";
    inline constexpr const char* TEXT        = "#1a1a2e";
    inline constexpr const char* TEXT_DIM    = "#64748b";
    inline constexpr const char* TEXT_MUTED  = "#94a3b8";
    inline constexpr const char* TEXT_BRIGHT = "#0f172a";
}

static const char* CSS = R"(
* {
    font-family: "Microsoft YaHei UI", "Microsoft YaHei", "Segoe UI", "PingFang SC", "Helvetica Neue", sans-serif;
    font-size: 12px;
    color: #1a1a2e;
    outline: none;
    border: none;
    letter-spacing: 0.3px;
}
QWidget { background: transparent; }

/* Scrollbar */
QScrollBar:vertical { background: transparent; width: 6px; margin: 0; }
QScrollBar::handle:vertical { background: rgba(180, 180, 190, 0.40); border-radius: 3px; min-height: 24px; }
QScrollBar::handle:vertical:hover { background: rgba(160, 160, 170, 0.55); }
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }
QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }

/* Labels */
QLabel[role="heading"]  { font-size: 11px; font-weight: 600; color: #334155; letter-spacing: 0.6px; }
QLabel[role="caption"]  { font-size: 11px; color: #64748b; }
QLabel[role="success"]  { font-size: 11px; color: #22c55e; font-weight: 600; }
QLabel[role="danger"]   { font-size: 11px; color: #ef4444; font-weight: 600; }
QLabel[role="warning"]  { font-size: 11px; color: #f59e0b; font-weight: 600; }

/* QLineEdit */
QLineEdit {
    background: rgba(255, 255, 255, 0.60);
    border: 1px solid rgba(200, 200, 210, 0.50);
    border-radius: 8px;
    padding: 9px 12px;
    font-size: 13px;
    font-weight: 500;
    color: #0f172a;
    selection-background-color: #4a9eff;
    selection-color: #ffffff;
}
QLineEdit:focus { border-color: #4a9eff; }
QLineEdit:disabled { background: rgba(240, 240, 245, 0.50); color: #94a3b8; border-color: rgba(220, 220, 230, 0.40); }
QLineEdit[readOnly="true"] { background: rgba(245, 245, 250, 0.55); color: #64748b; }

/* QComboBox */
QComboBox {
    background: rgba(255, 255, 255, 0.60);
    border: 1px solid rgba(200, 200, 210, 0.50);
    border-radius: 8px;
    padding: 8px 34px 8px 12px;
    font-size: 13px;
    font-weight: 500;
    color: #0f172a;
    selection-background-color: #4a9eff;
    selection-color: #ffffff;
}
QComboBox:hover { background: rgba(255, 255, 255, 0.72); border-color: rgba(180, 180, 190, 0.65); }
QComboBox:focus { border-color: #4a9eff; }
QComboBox:disabled { background: rgba(240, 240, 245, 0.50); color: #94a3b8; border-color: rgba(220, 220, 230, 0.40); }
QComboBox::drop-down {
    subcontrol-origin: padding;
    subcontrol-position: top right;
    width: 30px;
    border-left: 1px solid rgba(200, 200, 210, 0.35);
    border-top-right-radius: 8px;
    border-bottom-right-radius: 8px;
    background: rgba(255, 255, 255, 0.18);
}
QComboBox::down-arrow {
    image: url(:/ui/icons/chevron-down.svg);
    width: 12px;
    height: 12px;
    margin: 0;
}
QComboBox QAbstractItemView {
    background: rgba(255, 255, 255, 0.98);
    border: 1px solid rgba(200, 200, 210, 0.60);
    color: #1a1a2e;
    outline: none;
    padding: 4px;
    selection-background-color: #4a9eff;
    selection-color: #ffffff;
}
QComboBox QAbstractItemView::item {
    min-height: 26px;
    padding: 5px 10px;
    color: #1a1a2e;
    background: transparent;
}
QComboBox QAbstractItemView::item:hover {
    background: rgba(240, 240, 245, 0.95);
    color: #0f172a;
}
QComboBox QAbstractItemView::item:selected {
    background: #4a9eff;
    color: #ffffff;
}

/* QPushButton */
QPushButton {
    background: rgba(255, 255, 255, 0.70);
    border: 1px solid rgba(200, 200, 210, 0.50);
    border-radius: 8px;
    padding: 9px 18px;
    font-size: 12px;
    font-weight: 600;
    color: #1a1a2e;
    letter-spacing: 0.3px;
}
QPushButton:hover { background: rgba(240, 240, 245, 0.85); border-color: rgba(180, 180, 190, 0.60); }
QPushButton:pressed { background: rgba(230, 230, 240, 0.90); }
QPushButton:disabled { background: rgba(240, 240, 245, 0.40); color: #94a3b8; border-color: rgba(220, 220, 230, 0.30); }

QPushButton[role="primary"] {
    background: rgba(74, 158, 255, 0.85);
    border: 1px solid rgba(74, 158, 255, 0.60);
    color: #ffffff;
    font-weight: 600;
    letter-spacing: 0.5px;
}
QPushButton[role="primary"]:hover { background: rgba(109, 179, 255, 0.90); border-color: rgba(74, 158, 255, 0.80); }
QPushButton[role="primary"]:pressed { background: rgba(59, 130, 246, 0.90); }
QPushButton[role="primary"]:disabled { background: rgba(74, 158, 255, 0.40); border-color: rgba(74, 158, 255, 0.30); color: rgba(255, 255, 255, 0.50); }

QPushButton[role="ghost"] { background: transparent; border: 1px solid rgba(200, 200, 210, 0.50); }
QPushButton[role="ghost"]:hover { background: rgba(240, 240, 245, 0.60); border-color: rgba(180, 180, 190, 0.60); }

/* Cards */
QFrame[role="card"] { background: rgba(255, 255, 255, 0.72); border: 1px solid rgba(200, 200, 210, 0.40); border-radius: 12px; }
QFrame[role="sep"]  { background: rgba(200, 200, 210, 0.35); max-height: 1px; min-height: 1px; }

/* Tooltip */
QToolTip {
    background: rgba(255, 255, 255, 0.95); color: #1a1a2e; border: 1px solid rgba(200, 200, 210, 0.50);
    padding: 5px 10px; border-radius: 6px; font-size: 11px;
}
)";

static const char* POPUP_CSS = R"(
QMenu {
    background: rgba(255, 255, 255, 0.96);
    border: 1px solid rgba(200, 200, 210, 0.55);
    border-radius: 0;
    padding: 4px;
}
QMenu::item {
    background: transparent;
    color: #1a1a2e;
    padding: 8px 24px 8px 12px;
    border-radius: 0;
    font-size: 12px;
}
QMenu::item:selected {
    background: rgba(240, 240, 245, 0.90);
    color: #0f172a;
}
QMenu::separator {
    height: 1px;
    background: rgba(200, 200, 210, 0.45);
    margin: 5px 6px;
}
QMessageBox {
    background: rgba(255, 255, 255, 0.96);
}
QMessageBox QLabel {
    background: transparent;
    color: #1a1a2e;
    font-size: 12px;
}
QMessageBox QPushButton {
    background: rgba(255, 255, 255, 0.76);
    border: 1px solid rgba(200, 200, 210, 0.55);
    border-radius: 8px;
    padding: 8px 18px;
    color: #1a1a2e;
    font-weight: 600;
    min-width: 72px;
}
QMessageBox QPushButton:hover {
    background: rgba(240, 240, 245, 0.90);
    border-color: rgba(180, 180, 190, 0.65);
}
QMessageBox QPushButton:pressed {
    background: rgba(230, 230, 240, 0.95);
}
)";
