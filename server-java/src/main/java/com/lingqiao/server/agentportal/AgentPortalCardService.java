package com.lingqiao.server.agentportal;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardsService;
import com.lingqiao.server.cards.CardsWriteService;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.time.Duration;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.annotation.Transactional;

/**
 * Agent-owned card operations. This keeps the agent limit checks next to the
 * write transaction instead of reusing the less restrictive admin card path.
 */
public class AgentPortalCardService {
    private final JdbcTemplate jdbc;
    private final CardsService reader;
    private final CardsWriteService writer;

    public AgentPortalCardService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.reader = new CardsService(jdbc);
        this.writer = new CardsWriteService(jdbc, reader);
    }

    public List<AgentCard> list(String agentId) {
        return reader.listStoredByAgent(agentId).stream()
            .map(AgentCard::from)
            .toList();
    }

    @Transactional
    public AgentCard generate(String agentId, int durationHours, String note, int maxSessions) {
        requireWithinLimits(agentId, 1, durationHours, maxSessions);
        return AgentCard.from(writer.create(
            Duration.ofHours(durationHours), note, maxSessions, agentId));
    }

    @Transactional
    public List<AgentCard> batchGenerate(
        String agentId,
        int count,
        int durationHours,
        String note,
        int maxSessions
    ) {
        requireWithinLimits(agentId, count, durationHours, maxSessions);
        return writer.batchCreate(
                count, Duration.ofHours(durationHours), note, maxSessions, agentId)
            .stream()
            .map(AgentCard::from)
            .toList();
    }

    private void requireWithinLimits(String agentId, int count, int durationHours, int maxSessions) {
        List<Map<String, Object>> rows = jdbc.queryForList("""
            SELECT disabled, max_session_duration_minutes, max_daily_sessions,
                   max_concurrent_sessions
            FROM agents WHERE id = ?
            """, agentId);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "agent not found");
        }
        Map<String, Object> agent = rows.get(0);
        if (number(agent.get("disabled")) != 0) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "agent is disabled");
        }

        int maxDurationMinutes = number(agent.get("max_session_duration_minutes"));
        if (maxDurationMinutes > 0 && Duration.ofHours(durationHours).toMinutes() > maxDurationMinutes) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR,
                "duration exceeds maximum allowed (" + maxDurationMinutes + " minutes)"
            );
        }
        int maxConcurrent = number(agent.get("max_concurrent_sessions"));
        if (maxConcurrent > 0 && maxSessions > maxConcurrent) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR,
                "sessions per card exceeds maximum allowed (" + maxConcurrent + ")"
            );
        }
        int maxDaily = number(agent.get("max_daily_sessions"));
        if (maxDaily > 0) {
            Instant startOfUtcDay = Instant.now().truncatedTo(ChronoUnit.DAYS);
            Integer issuedToday = jdbc.queryForObject("""
                SELECT COALESCE(SUM(max_sessions), 0)
                FROM cards WHERE agent_id = ? AND created_at >= ?
                """, Integer.class, agentId, startOfUtcDay.toString());
            if ((issuedToday == null ? 0 : issuedToday) + count * maxSessions > maxDaily) {
                throw new SidecarException(
                    ErrorCode.INTERNAL_ERROR,
                    "exceeds maximum daily sessions limit (" + maxDaily + ")"
                );
            }
        }
    }

    private static int number(Object value) {
        return value instanceof Number number ? number.intValue() : 0;
    }

    public record AgentCard(
        String code,
        @JsonInclude(JsonInclude.Include.NON_EMPTY)
        @JsonProperty("machine_id") String machineId,
        @JsonInclude(JsonInclude.Include.NON_EMPTY)
        @JsonProperty("agent_id") String agentId,
        @JsonProperty("created_at") Instant createdAt,
        @JsonInclude(JsonInclude.Include.NON_NULL)
        @JsonProperty("activated_at") Instant activatedAt,
        @JsonProperty("expires_at") Instant expiresAt,
        String status,
        @JsonInclude(JsonInclude.Include.NON_EMPTY) String note,
        @JsonProperty("max_sessions") int maxSessions
    ) {
        static AgentCard from(Card card) {
            return new AgentCard(
                card.code(), card.machineId(), card.agentId(), card.createdAt(),
                card.activatedAt(), card.expiresAt(), card.status(), card.note(), card.maxSessions()
            );
        }
    }
}
