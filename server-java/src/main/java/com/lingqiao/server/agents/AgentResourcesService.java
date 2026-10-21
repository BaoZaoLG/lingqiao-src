package com.lingqiao.server.agents;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardsService;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampReader;
import java.time.Instant;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;

/** Machine and ownership queries exposed beside the admin agent module in Go. */
public class AgentResourcesService {
    private final JdbcTemplate jdbc;
    private final CardsService cards;

    public AgentResourcesService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.cards = new CardsService(jdbc);
    }

    public List<MachineInfo> machines() {
        try {
            Set<String> blacklisted = jdbc.query(
                "SELECT value FROM blacklist WHERE type = 'machine'",
                (rs, rowNum) -> rs.getString("value")
            ).stream().collect(Collectors.toSet());
            return jdbc.query("""
                SELECT c.machine_id, COUNT(c.code) AS card_count, MAX(s.last_seen) AS last_seen
                FROM cards c
                LEFT JOIN sessions s ON c.machine_id = s.machine_id
                WHERE c.machine_id != ''
                GROUP BY c.machine_id
                """, (rs, rowNum) -> new MachineInfo(
                    rs.getString("machine_id"),
                    rs.getInt("card_count"),
                    TimestampReader.read(rs, "last_seen"),
                    blacklisted.contains(rs.getString("machine_id"))
                ));
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to list machines");
        }
    }

    public List<Card> machineCards(String machineId) {
        if (machineId == null || machineId.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "machine id is required");
        }
        try {
            return cards.listStoredByMachine(machineId);
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to get machine cards");
        }
    }

    public List<AgentCard> agentCards(String agentId) {
        if (agentId == null || agentId.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "agent_id is required");
        }
        try {
            return cards.listStoredByAgent(agentId).stream().map(AgentCard::from).toList();
        } catch (DataAccessException ex) {
            // Go's adapter returns an empty slice when the underlying list fails.
            return List.of();
        }
    }

    public record MachineInfo(
        @JsonProperty("machine_id") String machineId,
        @JsonProperty("card_count") int cardCount,
        @JsonInclude(JsonInclude.Include.NON_NULL)
        @JsonProperty("last_seen") Instant lastSeen,
        @JsonProperty("is_blacklisted") boolean blacklisted
    ) {
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
                card.activatedAt(), card.expiresAt(), card.status(), card.note(),
                card.maxSessions());
        }
    }
}
