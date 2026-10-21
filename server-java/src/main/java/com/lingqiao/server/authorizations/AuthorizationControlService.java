package com.lingqiao.server.authorizations;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonUnwrapped;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardStatuses;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.sessions.Session;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.function.Supplier;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

/** Go-compatible authorization lifecycle control plane backed by cards and sessions. */
public class AuthorizationControlService {
    static final String ISSUED = "issued";
    static final String ACTIVATED = "activated";
    static final String IN_USE = "in_use";
    static final String EXPIRED = "expired";
    static final String FROZEN = "frozen";
    static final String ARCHIVED = "archived";

    private static final RowMapper<Card> CARD_MAPPER =
        (rs, rowNum) -> mapCard(rs);
    private static final RowMapper<Session> SESSION_MAPPER =
        (rs, rowNum) -> mapSession(rs);

    private final JdbcTemplate jdbc;
    private final Supplier<Instant> clock;

    public AuthorizationControlService(JdbcTemplate jdbc) {
        this(jdbc, Instant::now);
    }

    AuthorizationControlService(JdbcTemplate jdbc, Supplier<Instant> clock) {
        this.jdbc = jdbc;
        this.clock = clock;
    }

    public List<AuthorizationSummary> list(ListFilter filter) {
        try {
            List<Object> args = new ArrayList<>();
            StringBuilder sql = new StringBuilder("""
                SELECT code, machine_id, agent_id, created_at, activated_at,
                       expires_at, status, note, max_sessions
                FROM cards WHERE 1=1
                """);
            if (hasText(filter.query())) {
                sql.append(" AND (code LIKE ? OR note LIKE ?)");
                String like = "%" + filter.query() + "%";
                args.add(like);
                args.add(like);
            }
            if (hasText(filter.agentId())) {
                sql.append(" AND agent_id = ?");
                args.add(filter.agentId());
            }
            if (hasText(filter.machineId())) {
                sql.append(" AND machine_id = ?");
                args.add(filter.machineId());
            }
            sql.append(" ORDER BY created_at DESC");

            Instant now = clock.get();
            List<AuthorizationSummary> result = new ArrayList<>();
            for (Card card : jdbc.query(sql.toString(), CARD_MAPPER, args.toArray())) {
                List<Session> sessions = activeSessions(card.code(), now);
                AuthorizationSummary summary = summarize(card, sessions, now);
                if (ARCHIVED.equals(summary.effectiveStatus()) && !filter.includeArchived()) {
                    continue;
                }
                if (!matches(card, filter)) {
                    continue;
                }
                if (hasText(filter.effectiveStatus())
                    && !filter.effectiveStatus().equals(summary.effectiveStatus())) {
                    continue;
                }
                result.add(summary);
            }
            return result;
        } catch (SidecarException ex) {
            throw ex;
        } catch (DataAccessException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to list authorizations");
        }
    }

    public AuthorizationDetail detail(String code) {
        Card card = getCard(code);
        Instant now = clock.get();
        List<Session> sessions = activeSessions(card.code(), now);
        return new AuthorizationDetail(
            summarize(card, sessions, now), sessions.isEmpty() ? null : sessions);
    }

    public ImpactPreview preview(List<String> codes, String action, int extendHours) {
        int targets = 0;
        int activeCount = 0;
        Set<String> machineSet = new LinkedHashSet<>();
        Instant now = clock.get();
        for (String code : safeCodes(codes)) {
            Card card = getCard(code);
            targets++;
            if (hasText(card.machineId())) {
                machineSet.add(card.machineId());
            }
            activeCount += activeSessions(card.code(), now).size();
        }
        List<String> machines = machineSet.isEmpty()
            ? null
            : machineSet.stream().sorted().toList();
        List<String> warnings = null;
        if ("archive_delete".equals(action)) {
            warnings = List.of("授权将从默认列表隐藏，客户端不可继续使用，但历史记录会保留");
        }
        // Go currently reports every supported action, including archive_delete,
        // as reversible. Preserve that externally visible contract.
        return new ImpactPreview(action, targets, activeCount, machines, true, warnings);
    }

    public ActionResult execute(List<String> codes, String action, int extendHours) {
        int updated = 0;
        int skipped = 0;
        int failed = 0;
        List<String> changedCodes = new ArrayList<>();
        int effectiveExtendHours = extendHours <= 0 ? 720 : extendHours;

        for (String code : safeCodes(codes)) {
            Card card;
            try {
                card = findCard(code);
            } catch (DataAccessException ex) {
                failed++;
                continue;
            }
            if (card == null) {
                failed++;
                continue;
            }

            boolean changed;
            Instant expiresAt = card.expiresAt();
            String status = card.status();
            String machineId = card.machineId();
            Instant activatedAt = card.activatedAt();
            switch (nullToEmpty(action)) {
                case "extend" -> {
                    expiresAt = expiresAt.plus(Duration.ofHours(effectiveExtendHours));
                    if (CardStatuses.EXPIRED.equals(status)) {
                        status = CardStatuses.ACTIVE;
                    }
                    changed = true;
                }
                case "unbind_machine" -> {
                    changed = hasText(machineId) || activatedAt != null;
                    if (changed && activatedAt != null) {
                        Duration originalDuration = Duration.between(activatedAt, expiresAt);
                        expiresAt = card.createdAt().plus(originalDuration);
                    }
                    machineId = "";
                    activatedAt = null;
                }
                case "freeze" -> {
                    changed = !CardStatuses.DISABLED.equals(status);
                    status = CardStatuses.DISABLED;
                }
                case "restore" -> {
                    changed = CardStatuses.DISABLED.equals(status)
                        || CardStatuses.ARCHIVED.equals(status);
                    if (changed) {
                        status = CardStatuses.ACTIVE;
                    }
                }
                case "archive_delete" -> {
                    changed = !CardStatuses.ARCHIVED.equals(status);
                    if (changed) {
                        status = CardStatuses.ARCHIVED;
                        machineId = "";
                        activatedAt = null;
                    }
                }
                default -> {
                    failed++;
                    continue;
                }
            }
            if (!changed) {
                skipped++;
                continue;
            }

            try {
                int rows = jdbc.update("""
                    UPDATE cards
                    SET machine_id = ?, activated_at = ?, expires_at = ?, status = ?
                    WHERE code = ?
                    """,
                    machineId,
                    activatedAt == null ? null : TimestampWriter.format(activatedAt),
                    TimestampWriter.format(expiresAt),
                    status,
                    card.code()
                );
                if (rows == 0) {
                    failed++;
                    continue;
                }
            } catch (DataAccessException ex) {
                failed++;
                continue;
            }
            updated++;
            changedCodes.add(code);
        }
        return new ActionResult(
            updated, skipped, failed, changedCodes.isEmpty() ? null : changedCodes);
    }

    public void recordAudit(String action, String code, String detail) {
        try {
            jdbc.update(
                "INSERT INTO audit_log (time, action, card, detail) VALUES (?, ?, ?, ?)",
                TimestampWriter.format(clock.get()), action, nullToEmpty(code), detail);
        } catch (DataAccessException ignored) {
            // Go logs and ignores authorization audit failures after a successful action.
        }
    }

    private Card getCard(String code) {
        try {
            Card card = findCard(code);
            if (card == null) {
                throw new SidecarException(
                    ErrorCode.CARD_NOT_FOUND, "authorization not found");
            }
            return card;
        } catch (SidecarException ex) {
            throw ex;
        } catch (DataAccessException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to load authorization");
        }
    }

    private Card findCard(String code) {
        List<Card> rows = jdbc.query("""
            SELECT code, machine_id, agent_id, created_at, activated_at,
                   expires_at, status, note, max_sessions
            FROM cards WHERE code = ?
            """, CARD_MAPPER, nullToEmpty(code));
        return rows.isEmpty() ? null : rows.get(0);
    }

    private List<Session> activeSessions(String code, Instant now) {
        return jdbc.query("""
            SELECT token, card_code, machine_id, fingerprint, client_version,
                   created_at, last_seen, expires_at, remote_addr
            FROM sessions
            WHERE card_code = ? AND expires_at > ?
            ORDER BY last_seen DESC
            """, SESSION_MAPPER, code, TimestampWriter.format(now));
    }

    static AuthorizationSummary summarize(Card card, List<Session> sessions, Instant now) {
        Instant lastSeen = sessions.stream()
            .map(Session::lastSeen)
            .max(Comparator.naturalOrder())
            .orElse(null);
        return new AuthorizationSummary(
            card.code(), card.agentId(), card.machineId(), card.createdAt(),
            card.activatedAt(), card.expiresAt(),
            effectiveStatus(card, sessions, now), card.status(), card.note(),
            card.maxSessions(), sessions.size(), lastSeen);
    }

    static String effectiveStatus(Card card, List<Session> activeSessions, Instant now) {
        if (CardStatuses.ARCHIVED.equals(card.status())) {
            return ARCHIVED;
        }
        if (CardStatuses.DISABLED.equals(card.status())) {
            return FROZEN;
        }
        if (CardStatuses.EXPIRED.equals(card.status())
            || (card.expiresAt() != null && !Instant.EPOCH.equals(card.expiresAt())
                && !now.isBefore(card.expiresAt()))) {
            return EXPIRED;
        }
        if (!activeSessions.isEmpty()) {
            return IN_USE;
        }
        if (hasText(card.machineId()) || card.activatedAt() != null) {
            return ACTIVATED;
        }
        return ISSUED;
    }

    private static boolean matches(Card card, ListFilter filter) {
        if (("true".equals(filter.bound()) || "1".equals(filter.bound()))
            && !hasText(card.machineId())) {
            return false;
        }
        if (("false".equals(filter.bound()) || "0".equals(filter.bound()))
            && hasText(card.machineId())) {
            return false;
        }
        if (filter.expiresAfter() != null && card.expiresAt().isBefore(filter.expiresAfter())) {
            return false;
        }
        if (filter.expiresBefore() != null && card.expiresAt().isAfter(filter.expiresBefore())) {
            return false;
        }
        return filter.maxSessions() <= 0 || card.maxSessions() == filter.maxSessions();
    }

    private static Card mapCard(ResultSet rs) throws SQLException {
        return new Card(
            nullToEmpty(rs.getString("code")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("agent_id")),
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.read(rs, "activated_at"),
            TimestampReader.readOrEpoch(rs, "expires_at"),
            nullToEmpty(rs.getString("status")),
            nullToEmpty(rs.getString("note")),
            rs.getInt("max_sessions")
        );
    }

    private static Session mapSession(ResultSet rs) throws SQLException {
        return new Session(
            nullToEmpty(rs.getString("token")),
            nullToEmpty(rs.getString("card_code")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("fingerprint")),
            nullToEmpty(rs.getString("client_version")),
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.readOrEpoch(rs, "last_seen"),
            TimestampReader.readOrEpoch(rs, "expires_at"),
            nullToEmpty(rs.getString("remote_addr"))
        );
    }

    private static List<String> safeCodes(List<String> codes) {
        return codes == null ? List.of() : codes;
    }

    private static boolean hasText(String value) {
        return value != null && !value.isEmpty();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    public record ListFilter(
        String query,
        String effectiveStatus,
        String agentId,
        String machineId,
        String bound,
        Instant expiresAfter,
        Instant expiresBefore,
        int maxSessions,
        boolean includeArchived
    ) {
    }

    public record AuthorizationSummary(
        String code,
        @JsonInclude(JsonInclude.Include.NON_EMPTY)
        @JsonProperty("agent_id") String agentId,
        @JsonInclude(JsonInclude.Include.NON_EMPTY)
        @JsonProperty("machine_id") String machineId,
        @JsonProperty("created_at") Instant createdAt,
        @JsonInclude(JsonInclude.Include.NON_NULL)
        @JsonProperty("activated_at") Instant activatedAt,
        @JsonProperty("expires_at") Instant expiresAt,
        @JsonProperty("effective_status") String effectiveStatus,
        @JsonProperty("stored_status") String storedStatus,
        @JsonInclude(JsonInclude.Include.NON_EMPTY) String note,
        @JsonProperty("max_sessions") int maxSessions,
        @JsonProperty("active_sessions") int activeSessions,
        @JsonInclude(JsonInclude.Include.NON_NULL)
        @JsonProperty("last_seen") Instant lastSeen
    ) {
    }

    public record AuthorizationDetail(
        @JsonUnwrapped AuthorizationSummary summary,
        List<Session> sessions
    ) {
    }

    public record ImpactPreview(
        String action,
        @JsonProperty("target_count") int targetCount,
        @JsonProperty("active_sessions") int activeSessions,
        List<String> machines,
        boolean reversible,
        List<String> warnings
    ) {
    }

    public record ActionResult(
        int updated,
        int skipped,
        int failed,
        List<String> codes
    ) {
    }
}
