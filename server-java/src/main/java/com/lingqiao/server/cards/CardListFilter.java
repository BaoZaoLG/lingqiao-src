package com.lingqiao.server.cards;

import java.time.Instant;

/**
 * Admin card list filter mirroring Go cards.HandleListCards query parameters.
 *
 * <p>{@code status} is applied against the effective status (expiry wins over the
 * stored column), so {@code active}/{@code expired} are filtered in memory exactly
 * like Go rather than pushed into SQL.
 */
public record CardListFilter(
    String query,
    String status,
    String machineId,
    String agentId,
    Boolean bound,
    Instant expiresBefore,
    Instant expiresAfter,
    Integer maxSessions
) {
    public CardListFilter(String query, String status, String machineId, String agentId) {
        this(query, status, machineId, agentId, null, null, null, null);
    }

    /** Go pushes only non-lifecycle statuses into the SQL filter. */
    public String sqlStatus() {
        if (status == null || status.isBlank()) {
            return null;
        }
        if (CardStatuses.ACTIVE.equals(status) || CardStatuses.EXPIRED.equals(status)) {
            return null;
        }
        return status;
    }
}
