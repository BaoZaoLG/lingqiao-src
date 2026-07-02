#include <QCoreApplication>
#include <QJsonObject>
#include <QJsonDocument>
#include <iostream>
#include <cassert>

#include "services/api_client.h"

#define ASSERT_EQ(a, b) if((a) != (b)) { std::cerr << "Assertion failed: " << #a << " == " << #b << std::endl; return 1; }
#define ASSERT_TRUE(a) if(!(a)) { std::cerr << "Assertion failed: " << #a << " is false" << std::endl; return 1; }

int test_ApiErrorNetworkParsing() {
    ApiClient client(L"localhost", 8080);
    
    ApiResult rawResp;
    rawResp.statusCode = 0;
    rawResp.error = "Connection refused";
    
    ApiError err = client.parseError(rawResp);
    ASSERT_EQ((int)err.category, (int)ApiErrorCategory::Network);
    ASSERT_EQ(err.status, 0);
    ASSERT_EQ(err.detail, QString("Connection refused"));
    return 0;
}

int test_ApiErrorLegacyParsing() {
    ApiClient client(L"localhost", 8080);
    
    ApiResult rawResp;
    rawResp.statusCode = 403;
    QJsonObject obj;
    obj["message"] = "Invalid card";
    rawResp.body = QJsonDocument(obj).toJson(QJsonDocument::Compact);
    
    ApiError err = client.parseError(rawResp);
    ASSERT_EQ((int)err.category, (int)ApiErrorCategory::Business);
    ASSERT_EQ(err.status, 403);
    ASSERT_EQ(err.detail, QString("Invalid card"));
    return 0;
}

int test_ApiErrorRFC9457Parsing() {
    ApiClient client(L"localhost", 8080);
    
    ApiResult rawResp;
    rawResp.statusCode = 400;
    QJsonObject obj;
    obj["code"] = "invalid_parameter";
    obj["detail"] = "Parameter X is missing";
    rawResp.body = QJsonDocument(obj).toJson(QJsonDocument::Compact);
    
    ApiError err = client.parseError(rawResp);
    ASSERT_EQ((int)err.category, (int)ApiErrorCategory::Http);
    ASSERT_EQ(err.status, 400);
    ASSERT_EQ(err.code, QString("invalid_parameter"));
    ASSERT_EQ(err.detail, QString("Parameter X is missing"));
    return 0;
}

int main(int argc, char *argv[]) {
    QCoreApplication a(argc, argv);
    std::cout << "Running tests..." << std::endl;
    
    if (test_ApiErrorNetworkParsing() != 0) return 1;
    if (test_ApiErrorLegacyParsing() != 0) return 1;
    if (test_ApiErrorRFC9457Parsing() != 0) return 1;
    
    std::cout << "Tests passed." << std::endl;
    return 0;
}
