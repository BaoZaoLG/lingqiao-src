package com.lingqiao.server.common.errors;

import java.util.Locale;

public final class ProblemDetailsFactory {
    private static final String TYPE_PREFIX = "https://lingqiao.local/problems/";

    private ProblemDetailsFactory() {
    }

    public static ProblemDetails from(ErrorCode code, String instance, String detail) {
        String slug = code.name().toLowerCase(Locale.ROOT).replace('_', '-');
        return new ProblemDetails(
            TYPE_PREFIX + slug,
            code.title(),
            code.status(),
            detail,
            instance,
            code.name()
        );
    }
}
