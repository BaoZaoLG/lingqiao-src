package com.lingqiao.server.cards;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.annotation.Transactional;

/**
 * Java write path for cards. Mirrors Go cards.Service semantics.
 * Enabled only when lingqiao.sidecar.cards-write-enabled=true.
 */
public class CardsWriteService {
    private final JdbcTemplate jdbc;
    private final CardsService reader;

    public CardsWriteService(JdbcTemplate jdbc, CardsService reader) {
        this.jdbc = jdbc;
        this.reader = reader;
    }

    @Transactional
    public Card create(Duration duration, String note, int maxSessions, String agentId) {
        validateCreate(duration, maxSessions);
        Instant now = Instant.now();
        String code = CardCodes.format(generateCode());
        insert(code, agentId, note, maxSessions, now, now.plus(duration));
        return reader.get(code);
    }

    @Transactional
    public Card createWithCode(String code, Duration duration, String note, int maxSessions, String agentId) {
        validateCreate(duration, maxSessions);
        String normalized = CardCodes.validateImport(code)
            .map(CardCodes::format)
            .orElseThrow(() -> new SidecarException(ErrorCode.BAD_REQUEST, importValidationMessage(code)));
        Instant now = Instant.now();
        insert(normalized, agentId, note, maxSessions, now, now.plus(duration));
        return reader.get(normalized);
    }

    @Transactional
    public List<Card> batchCreate(int count, Duration duration, String note, int maxSessions, String agentId) {
        if (count <= 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "count must be positive");
        }
        validateCreate(duration, maxSessions);
        Instant now = Instant.now();
        List<Card> created = new ArrayList<>(count);
        for (int i = 0; i < count; i++) {
            String code = CardCodes.format(generateCode());
            insert(code, agentId, note, maxSessions, now, now.plus(duration));
            created.add(reader.get(code));
        }
        return created;
    }

    @Transactional
    public void updateStatus(String code, String status) {
        if (!CardStatuses.isValid(status)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid card status");
        }
        requireUpdated(jdbc.update("UPDATE cards SET status = ? WHERE code = ?", status, normalized(code)));
    }

    @Transactional
    public void extend(String code, int extendHours) {
        if (extendHours <= 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "extendHours must be positive");
        }
        Card card = reader.get(normalized(code));
        Instant expiresAt = card.expiresAt().plus(Duration.ofHours(extendHours));
        String status = CardStatuses.EXPIRED.equals(card.status()) ? CardStatuses.ACTIVE : card.status();
        requireUpdated(jdbc.update(
            "UPDATE cards SET expires_at = ?, status = ? WHERE code = ?",
            TimestampWriter.format(expiresAt), status, card.code()
        ));
    }

    @Transactional
    public void unbind(String code) {
        Card card = reader.get(normalized(code));
        Instant expiresAt = card.expiresAt();
        if (card.activatedAt() != null) {
            Duration originalDuration = Duration.between(card.activatedAt(), card.expiresAt());
            expiresAt = card.createdAt().plus(originalDuration);
        }
        requireUpdated(jdbc.update(
            "UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?",
            TimestampWriter.format(expiresAt), card.code()
        ));
    }

    @Transactional
    public void updateDetails(String code, String note, Integer maxSessions) {
        Card card = reader.get(normalized(code));
        String nextNote = note == null ? card.note() : note;
        int nextMaxSessions = card.maxSessions();
        if (maxSessions != null) {
            if (maxSessions < 1) {
                throw new SidecarException(ErrorCode.BAD_REQUEST, "max sessions must be at least 1");
            }
            nextMaxSessions = maxSessions;
        }
        requireUpdated(jdbc.update(
            "UPDATE cards SET note = ?, max_sessions = ? WHERE code = ?",
            nextNote, nextMaxSessions, card.code()
        ));
    }

    @Transactional
    public void delete(String code) {
        requireUpdated(jdbc.update("DELETE FROM cards WHERE code = ?", normalized(code)));
    }

    /**
     * Mirrors Go's per-card bulk result semantics, including unchanged and
     * missing cards being reported as skipped rather than failing the request.
     */
    public BulkResult bulkUpdateDetailed(List<String> codes, String action, int extendHours, boolean dryRun) {
        if (!BulkAction.isValid(action)) {
            List<BulkItem> items = new ArrayList<>(codes.size());
            for (String code : codes) {
                items.add(new BulkItem(code, "failed", "unknown action: " + action));
            }
            throw new SidecarException(ErrorCode.BAD_REQUEST, "unknown action: " + action);
        }

        List<BulkItem> items = new ArrayList<>(codes.size());
        int updated = 0;
        int skipped = 0;
        int failed = 0;
        int effectiveExtendHours = extendHours <= 0 ? 720 : extendHours;

        for (String code : codes) {
            Card card;
            try {
                card = reader.get(code);
            } catch (SidecarException ex) {
                if (ex.code() == ErrorCode.CARD_NOT_FOUND) {
                    items.add(new BulkItem(code, "skipped", "card not found"));
                    skipped++;
                    continue;
                }
                items.add(new BulkItem(code, "failed", ex.getMessage()));
                failed++;
                continue;
            }

            boolean changed = switch (action) {
                case "disable" -> !CardStatuses.DISABLED.equals(card.status());
                case "enable" -> !CardStatuses.ACTIVE.equals(card.status());
                case "expire" -> !CardStatuses.EXPIRED.equals(card.status());
                case "extend", "delete" -> true;
                case "unbind" -> card.machineId() != null && !card.machineId().isEmpty();
                default -> false;
            };
            if (!changed) {
                items.add(new BulkItem(code, "skipped", ""));
                skipped++;
                continue;
            }

            if (!dryRun) {
                try {
                    switch (action) {
                        case "disable" -> updateStatus(card.code(), CardStatuses.DISABLED);
                        case "enable" -> updateStatus(card.code(), CardStatuses.ACTIVE);
                        case "expire" -> updateStatus(card.code(), CardStatuses.EXPIRED);
                        case "extend" -> extend(card.code(), effectiveExtendHours);
                        case "unbind" -> unbind(card.code());
                        case "delete" -> delete(card.code());
                        default -> throw new IllegalStateException("validated action was not handled");
                    }
                } catch (RuntimeException ex) {
                    items.add(new BulkItem(code, "failed", ex.getMessage()));
                    failed++;
                    continue;
                }
            }
            items.add(new BulkItem(code, "updated", ""));
            updated++;
        }
        return new BulkResult(updated, skipped, failed, items);
    }

    private void insert(String code, String agentId, String note, int maxSessions, Instant createdAt, Instant expiresAt) {
        jdbc.update("""
            INSERT INTO cards (code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions)
            VALUES (?, '', ?, ?, NULL, ?, ?, ?, ?)
            """,
            code,
            agentId == null ? "" : agentId,
            TimestampWriter.format(createdAt),
            TimestampWriter.format(expiresAt),
            CardStatuses.ACTIVE,
            note == null ? "" : note,
            maxSessions
        );
    }

    private static void validateCreate(Duration duration, int maxSessions) {
        if (duration == null || duration.isZero() || duration.isNegative()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "duration must be positive");
        }
        if (maxSessions < 1) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "max sessions must be at least 1");
        }
    }

    private static void requireUpdated(int rows) {
        if (rows == 0) {
            throw new SidecarException(ErrorCode.CARD_NOT_FOUND, "card not found");
        }
    }

    private static String normalized(String code) {
        String normalized = CardCodes.normalize(code);
        if (normalized.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "code is required");
        }
        return CardCodes.format(normalized);
    }

    private static String generateCode() {
        java.security.SecureRandom random = new java.security.SecureRandom();
        StringBuilder sb = new StringBuilder(CardCodes.NORMALIZED_LENGTH);
        for (int i = 0; i < CardCodes.NORMALIZED_LENGTH; i++) {
            sb.append(CardCodes.ALPHABET.charAt(random.nextInt(CardCodes.ALPHABET.length())));
        }
        return sb.toString();
    }

    private static String importValidationMessage(String code) {
        String normalized = CardCodes.normalize(code);
        if (normalized.length() != CardCodes.NORMALIZED_LENGTH) {
            return "card code must contain 18 base32 characters";
        }
        for (int i = 0; i < normalized.length(); i++) {
            if (CardCodes.ALPHABET.indexOf(normalized.charAt(i)) < 0) {
                return "card code contains invalid character '" + normalized.charAt(i) + "'";
            }
        }
        return "invalid card code";
    }

    private static final class BulkAction {
        private BulkAction() {
        }

        static boolean isValid(String action) {
            return "disable".equals(action)
                || "enable".equals(action)
                || "expire".equals(action)
                || "extend".equals(action)
                || "unbind".equals(action)
                || "delete".equals(action);
        }
    }

    public record BulkItem(
        String code,
        String status,
        @com.fasterxml.jackson.annotation.JsonInclude(
            com.fasterxml.jackson.annotation.JsonInclude.Include.NON_EMPTY)
        String message
    ) {
    }

    public record BulkResult(int updated, int skipped, int failed, List<BulkItem> items) {
    }
}
