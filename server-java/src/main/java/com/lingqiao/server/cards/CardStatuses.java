package com.lingqiao.server.cards;

import java.time.Instant;
import java.util.Set;

/** Card lifecycle statuses mirroring Go cards.Status plus Go's effective-status rule. */
public final class CardStatuses {
    public static final String ACTIVE = "active";
    public static final String DISABLED = "disabled";
    public static final String EXPIRED = "expired";
    public static final String ARCHIVED = "archived";
    public static final String UNUSED = "unused";

    private static final Set<String> VALID = Set.of(ACTIVE, DISABLED, EXPIRED, ARCHIVED, UNUSED);

    private CardStatuses() {
    }

    public static boolean isValid(String status) {
        return status != null && VALID.contains(status);
    }

    /**
     * Mirrors Go cards.effectiveStatus: a card whose expiry has passed reports
     * "expired" regardless of the stored status column.
     */
    public static String effective(String storedStatus, Instant expiresAt, Instant now) {
        if (expiresAt != null && !Instant.EPOCH.equals(expiresAt) && !now.isBefore(expiresAt)) {
            return EXPIRED;
        }
        return storedStatus == null ? "" : storedStatus;
    }
}
