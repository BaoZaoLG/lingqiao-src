package com.lingqiao.server.common.errors;

public final class SidecarException extends RuntimeException {
    private final ErrorCode code;

    public SidecarException(ErrorCode code, String message) {
        super(message);
        this.code = code;
    }

    public ErrorCode code() {
        return code;
    }
}
