package com.lingqiao.server.common.errors;

public enum ErrorCode {
    INVALID_JSON(400, "Bad Request"),
    METHOD_NOT_ALLOWED(405, "Method Not Allowed"),
    HMAC_MISSING(401, "Unauthorized"),
    HMAC_INVALID(401, "Unauthorized"),
    BAD_REQUEST(400, "Bad Request"),
    UNAUTHORIZED(401, "Unauthorized"),
    FORBIDDEN(403, "Forbidden"),
    NOT_FOUND(404, "Not Found"),
    TOO_MANY_REQUESTS(429, "Too Many Requests"),
    CONFLICT(409, "Conflict"),
    INTERNAL_ERROR(500, "Internal Server Error"),
    FEATURE_DISABLED(503, "Service Unavailable"),
    CARD_NOT_FOUND(404, "Card Not Found"),
    SESSION_NOT_FOUND(404, "Session Not Found"),
    AGENT_NOT_FOUND(404, "Agent Not Found"),
    RELEASE_NOT_FOUND(404, "Release Not Found"),
    PAYLOAD_NOT_FOUND(404, "Payload Not Found");

    private final int status;
    private final String title;

    ErrorCode(int status, String title) {
        this.status = status;
        this.title = title;
    }

    public int status() {
        return status;
    }

    public String title() {
        return title;
    }
}
