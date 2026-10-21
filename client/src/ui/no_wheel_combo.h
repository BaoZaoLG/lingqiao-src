#pragma once
// Combo box that ignores mouse-wheel value changes.

#include <QComboBox>
#include <QWheelEvent>

class NoWheelComboBox : public QComboBox {
public:
    explicit NoWheelComboBox(QWidget* parent = nullptr) : QComboBox(parent) {}

protected:
    void wheelEvent(QWheelEvent* e) override {
        e->ignore();
    }
};
