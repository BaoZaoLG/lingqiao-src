#include "manual_injector_core.h"

#include <QtCore/QDir>
#include <QtCore/QFileInfo>
#include <QtWidgets/QApplication>
#include <QtWidgets/QFileDialog>
#include <QtWidgets/QFormLayout>
#include <QtWidgets/QHBoxLayout>
#include <QtWidgets/QHeaderView>
#include <QtWidgets/QLabel>
#include <QtWidgets/QLineEdit>
#include <QtWidgets/QMainWindow>
#include <QtWidgets/QPlainTextEdit>
#include <QtWidgets/QPushButton>
#include <QtWidgets/QTableWidget>
#include <QtWidgets/QTableWidgetItem>
#include <QtWidgets/QVBoxLayout>
#include <QtWidgets/QWidget>

static QString ToQString(const std::wstring& text) {
    return QString::fromWCharArray(text.c_str());
}

static const wchar_t* WidePtr(const QString& text) {
    return reinterpret_cast<const wchar_t*>(text.utf16());
}

class ManualInjectorWindow : public QMainWindow {
public:
    ManualInjectorWindow() {
        setWindowTitle(QStringLiteral("Manual DLL Injector"));
        resize(720, 500);

        auto* root = new QWidget(this);
        auto* layout = new QVBoxLayout(root);
        auto* form = new QFormLayout();

        m_exeEdit = new QLineEdit(root);
        m_exeEdit->setPlaceholderText(QStringLiteral("选择要启动的目标程序 .exe"));
        auto* exeBrowseButton = new QPushButton(QStringLiteral("浏览..."), root);
        form->addRow(QStringLiteral("目标程序"), MakeBrowseRow(m_exeEdit, exeBrowseButton, root));

        m_argsEdit = new QLineEdit(root);
        m_argsEdit->setPlaceholderText(QStringLiteral("可选，例如 --flag value"));
        form->addRow(QStringLiteral("启动参数"), m_argsEdit);

        m_workDirEdit = new QLineEdit(root);
        m_workDirEdit->setPlaceholderText(QStringLiteral("默认使用目标程序所在目录"));
        auto* workDirBrowseButton = new QPushButton(QStringLiteral("浏览..."), root);
        form->addRow(QStringLiteral("工作目录"), MakeBrowseRow(m_workDirEdit, workDirBrowseButton, root));

        m_dllEdit = new QLineEdit(root);
        m_dllEdit->setPlaceholderText(QStringLiteral("选择要注入的 DLL，例如 CefHook.dll"));
        auto* dllBrowseButton = new QPushButton(QStringLiteral("浏览..."), root);
        form->addRow(QStringLiteral("DLL 路径"), MakeBrowseRow(m_dllEdit, dllBrowseButton, root));

        layout->addLayout(form);

        auto* hint = new QLabel(
            QStringLiteral("仅用于本机调试：本工具只启动你选择的 EXE，并向该新进程注入你选择的本地 DLL。不会自动提权、隐藏模块或绕过保护。"),
            root);
        hint->setWordWrap(true);
        layout->addWidget(hint);

        auto* launchButton = new QPushButton(QStringLiteral("启动并注入"), root);
        layout->addWidget(launchButton);

        auto* processRow = new QWidget(root);
        auto* processRowLayout = new QHBoxLayout(processRow);
        processRowLayout->setContentsMargins(0, 0, 0, 0);
        m_processFilterEdit = new QLineEdit(processRow);
        m_processFilterEdit->setPlaceholderText(QStringLiteral("筛选运行中进程，例如 CXexam 或 renderer"));
        auto* refreshProcessButton = new QPushButton(QStringLiteral("刷新进程"), processRow);
        auto* injectSelectedButton = new QPushButton(QStringLiteral("注入选中进程"), processRow);
        processRowLayout->addWidget(m_processFilterEdit, 1);
        processRowLayout->addWidget(refreshProcessButton);
        processRowLayout->addWidget(injectSelectedButton);
        layout->addWidget(processRow);

        m_processTable = new QTableWidget(root);
        m_processTable->setColumnCount(4);
        m_processTable->setHorizontalHeaderLabels(QStringList()
            << QStringLiteral("PID")
            << QStringLiteral("父 PID")
            << QStringLiteral("进程名")
            << QStringLiteral("路径"));
        m_processTable->horizontalHeader()->setSectionResizeMode(0, QHeaderView::ResizeToContents);
        m_processTable->horizontalHeader()->setSectionResizeMode(1, QHeaderView::ResizeToContents);
        m_processTable->horizontalHeader()->setSectionResizeMode(2, QHeaderView::ResizeToContents);
        m_processTable->horizontalHeader()->setSectionResizeMode(3, QHeaderView::Stretch);
        m_processTable->setEditTriggers(QAbstractItemView::NoEditTriggers);
        m_processTable->setSelectionBehavior(QAbstractItemView::SelectRows);
        m_processTable->setSelectionMode(QAbstractItemView::SingleSelection);
        layout->addWidget(m_processTable, 1);

        m_log = new QPlainTextEdit(root);
        m_log->setReadOnly(true);
        m_log->setPlaceholderText(QStringLiteral("日志输出"));
        layout->addWidget(m_log, 1);

        setCentralWidget(root);

        QObject::connect(exeBrowseButton, &QPushButton::clicked, [this]() {
            QString file = QFileDialog::getOpenFileName(
                this,
                QStringLiteral("选择目标程序"),
                QString(),
                QStringLiteral("Executable files (*.exe);;All files (*.*)"));
            if (!file.isEmpty()) {
                file = QDir::toNativeSeparators(file);
                m_exeEdit->setText(file);
                if (m_workDirEdit->text().trimmed().isEmpty()) {
                    m_workDirEdit->setText(QDir::toNativeSeparators(QFileInfo(file).absolutePath()));
                }
            }
        });

        QObject::connect(workDirBrowseButton, &QPushButton::clicked, [this]() {
            QString dir = QFileDialog::getExistingDirectory(this, QStringLiteral("选择工作目录"));
            if (!dir.isEmpty()) {
                m_workDirEdit->setText(QDir::toNativeSeparators(dir));
            }
        });

        QObject::connect(dllBrowseButton, &QPushButton::clicked, [this]() {
            QString file = QFileDialog::getOpenFileName(
                this,
                QStringLiteral("选择 DLL"),
                QString(),
                QStringLiteral("DLL files (*.dll);;All files (*.*)"));
            if (!file.isEmpty()) {
                m_dllEdit->setText(QDir::toNativeSeparators(file));
            }
        });

        QObject::connect(launchButton, &QPushButton::clicked, [this]() {
            RunLaunchAndInjection();
        });
        QObject::connect(refreshProcessButton, &QPushButton::clicked, [this]() {
            RefreshProcessList();
        });
        QObject::connect(injectSelectedButton, &QPushButton::clicked, [this]() {
            InjectSelectedProcess();
        });
        QObject::connect(m_processFilterEdit, &QLineEdit::textChanged, [this]() {
            PopulateProcessTable();
        });

        RefreshProcessList();
    }

private:
    QWidget* MakeBrowseRow(QLineEdit* edit, QPushButton* button, QWidget* parent) {
        auto* row = new QWidget(parent);
        auto* rowLayout = new QHBoxLayout(row);
        rowLayout->setContentsMargins(0, 0, 0, 0);
        rowLayout->addWidget(edit, 1);
        rowLayout->addWidget(button);
        return row;
    }

    void AppendLog(const QString& line) {
        m_log->appendPlainText(line);
    }

    void RunLaunchAndInjection() {
        const QString exePath = m_exeEdit->text().trimmed();
        const QString args = m_argsEdit->text().trimmed();
        QString workDir = m_workDirEdit->text().trimmed();
        const QString dllPath = m_dllEdit->text().trimmed();

        AppendLog(QStringLiteral("----"));
        AppendLog(QStringLiteral("开始校验参数"));

        if (!ManualInjectorIsAbsolutePath(WidePtr(exePath))) {
            AppendLog(QStringLiteral("错误：目标程序必须是绝对路径"));
            return;
        }
        if (!ManualInjectorFileExists(WidePtr(exePath))) {
            AppendLog(QStringLiteral("错误：目标程序不存在"));
            return;
        }
        if (workDir.isEmpty()) {
            workDir = ToQString(ManualInjectorDefaultWorkingDirectory(WidePtr(exePath)));
            m_workDirEdit->setText(workDir);
        }
        if (!workDir.isEmpty() && !QDir(workDir).exists()) {
            AppendLog(QStringLiteral("错误：工作目录不存在"));
            return;
        }
        if (!ManualInjectorIsAbsolutePath(WidePtr(dllPath))) {
            AppendLog(QStringLiteral("错误：DLL 路径必须是绝对路径"));
            return;
        }
        if (!ManualInjectorFileExists(WidePtr(dllPath))) {
            AppendLog(QStringLiteral("错误：DLL 文件不存在"));
            return;
        }

        AppendLog(QStringLiteral("目标程序: %1").arg(exePath));
        AppendLog(QStringLiteral("启动参数: %1").arg(args.isEmpty() ? QStringLiteral("(无)") : args));
        AppendLog(QStringLiteral("工作目录: %1").arg(workDir.isEmpty() ? QStringLiteral("(默认)") : workDir));
        AppendLog(QStringLiteral("DLL: %1").arg(dllPath));
        AppendLog(QStringLiteral("启动目标程序并等待初始化"));

        DWORD launchedPid = 0;
        ManualInjectResult result = ManualLaunchAndInject(
            WidePtr(exePath),
            WidePtr(args),
            workDir.isEmpty() ? L"" : WidePtr(workDir),
            WidePtr(dllPath),
            800,
            &launchedPid);

        if (launchedPid) {
            AppendLog(QStringLiteral("新进程 PID: %1").arg(launchedPid));
        }
        AppendLog(ToQString(ManualInjectorFormatResult(result)));
        if (result.code == ManualInjectResultCode::Success) {
            AppendLog(QStringLiteral("启动并注入完成"));
        } else {
            AppendLog(QStringLiteral("启动或注入失败，检查路径、权限、目标/DLL 架构是否一致"));
        }
    }

    void RefreshProcessList() {
        m_processes = ManualInjectorListProcesses();
        PopulateProcessTable();
        AppendLog(QStringLiteral("已刷新进程列表: %1 个").arg(static_cast<int>(m_processes.size())));
    }

    void PopulateProcessTable() {
        const QString filter = m_processFilterEdit->text().trimmed();
        m_processTable->setRowCount(0);

        for (const ManualProcessInfo& process : m_processes) {
            const QString pid = QString::number(process.pid);
            const QString parentPid = QString::number(process.parentPid);
            const QString name = ToQString(process.exeName);
            const QString path = ToQString(process.exePath);
            if (!filter.isEmpty() &&
                !pid.contains(filter, Qt::CaseInsensitive) &&
                !parentPid.contains(filter, Qt::CaseInsensitive) &&
                !name.contains(filter, Qt::CaseInsensitive) &&
                !path.contains(filter, Qt::CaseInsensitive)) {
                continue;
            }

            const int row = m_processTable->rowCount();
            m_processTable->insertRow(row);

            auto* pidItem = new QTableWidgetItem(pid);
            pidItem->setData(Qt::UserRole, static_cast<unsigned int>(process.pid));
            m_processTable->setItem(row, 0, pidItem);
            m_processTable->setItem(row, 1, new QTableWidgetItem(parentPid));
            m_processTable->setItem(row, 2, new QTableWidgetItem(name));
            m_processTable->setItem(row, 3, new QTableWidgetItem(path.isEmpty() ? QStringLiteral("(无权限读取)") : path));
        }
    }

    void InjectSelectedProcess() {
        const QList<QTableWidgetItem*> selected = m_processTable->selectedItems();
        if (selected.isEmpty()) {
            AppendLog(QStringLiteral("错误：请先选择一个运行中的进程"));
            return;
        }

        const int row = selected.first()->row();
        QTableWidgetItem* pidItem = m_processTable->item(row, 0);
        if (!pidItem) {
            AppendLog(QStringLiteral("错误：无法读取选中进程 PID"));
            return;
        }

        const DWORD pid = pidItem->data(Qt::UserRole).toUInt();
        const QString dllPath = m_dllEdit->text().trimmed();
        if (!ManualInjectorIsAbsolutePath(WidePtr(dllPath))) {
            AppendLog(QStringLiteral("错误：DLL 路径必须是绝对路径"));
            return;
        }
        if (!ManualInjectorFileExists(WidePtr(dllPath))) {
            AppendLog(QStringLiteral("错误：DLL 文件不存在"));
            return;
        }

        AppendLog(QStringLiteral("----"));
        AppendLog(QStringLiteral("向运行中进程注入: PID=%1 name=%2").arg(pid).arg(m_processTable->item(row, 2)->text()));
        AppendLog(QStringLiteral("DLL: %1").arg(dllPath));
        ManualInjectResult result = ManualInjectDll(pid, WidePtr(dllPath));
        AppendLog(ToQString(ManualInjectorFormatResult(result)));
        if (result.code == ManualInjectResultCode::Success) {
            AppendLog(QStringLiteral("运行中进程注入完成"));
        } else {
            AppendLog(QStringLiteral("注入失败，检查权限、目标/DLL 架构是否一致，并确认选中的是 Renderer 子进程"));
        }
    }

    QLineEdit* m_exeEdit = nullptr;
    QLineEdit* m_argsEdit = nullptr;
    QLineEdit* m_workDirEdit = nullptr;
    QLineEdit* m_dllEdit = nullptr;
    QLineEdit* m_processFilterEdit = nullptr;
    QTableWidget* m_processTable = nullptr;
    QPlainTextEdit* m_log = nullptr;
    std::vector<ManualProcessInfo> m_processes;
};

int main(int argc, char** argv) {
    QApplication app(argc, argv);
    ManualInjectorWindow window;
    window.show();
    return app.exec();
}
