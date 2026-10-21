package com.lingqiao.server.releases;

import java.util.Set;

public final class ReleaseStatuses {
    public static final String DRAFT = "draft";
    public static final String PUBLISHED = "published";
    public static final String PAUSED = "paused";
    public static final String ROLLED_BACK = "rolled_back";

    private static final Set<String> VALID = Set.of(DRAFT, PUBLISHED, PAUSED, ROLLED_BACK);

    private ReleaseStatuses() {
    }

    public static boolean isValid(String status) {
        return status != null && VALID.contains(status);
    }
}
