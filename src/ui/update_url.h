#pragma once

#include <windows.h>
#include <winhttp.h>

#include <QString>
#include <QUrl>
#include <string>

#include "strcrypt.h"

struct UpdateDownloadTarget {
    std::wstring host;
    int port = INTERNET_DEFAULT_HTTPS_PORT;
    std::wstring path;
    bool secure = true;
    QString error;
};

static UpdateDownloadTarget ResolveUpdateDownloadTarget(const QString& url, const QString& label,
                                                        const wchar_t* defaultHost, int defaultPort) {
    UpdateDownloadTarget target;
    target.host = std::wstring(defaultHost);
    target.port = defaultPort;
    target.secure = true;

    if (url.startsWith("http://", Qt::CaseInsensitive) ||
        url.startsWith("https://", Qt::CaseInsensitive)) {
        QUrl qurl(url);
        if (!qurl.isValid() || qurl.host().isEmpty()) {
            target.error = QString::fromUtf8(_S("%1地址无效")).arg(label);
            return target;
        }
        target.secure = qurl.scheme().compare("https", Qt::CaseInsensitive) == 0;
        target.host = qurl.host().toStdWString();
        int explicitPort = qurl.port();
        if (explicitPort > 0) {
            target.port = explicitPort;
        } else {
            target.port = target.secure ? INTERNET_DEFAULT_HTTPS_PORT : INTERNET_DEFAULT_HTTP_PORT;
        }
        target.path = qurl.path(QUrl::FullyEncoded).toStdWString();
        QString query = qurl.query(QUrl::FullyEncoded);
        if (!query.isEmpty()) target.path += L"?" + query.toStdWString();
    } else {
        target.path = url.toStdWString();
    }

    if (target.path.empty()) {
        target.error = QString::fromUtf8(_S("%1地址无效")).arg(label);
    }
    return target;
}
