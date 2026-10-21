package com.lingqiao.server.agentportal;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.security.SecureRandom;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Admin invite management compatible with Go agents.InviteService. */
public class InviteAdminService {
    private static final Instant GO_ZERO_TIME = Instant.parse("0001-01-01T00:00:00Z");
    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;
    private final SecureRandom random = new SecureRandom();

    public InviteAdminService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public List<InviteView> list() {
        return jdbc.query("""
            SELECT code, created_at, created_by, used_by, used_at, max_uses, use_count
            FROM invites ORDER BY created_at DESC
            """, (rs, rowNum) -> view(rs));
    }

    public List<InviteView> create(int count, int maxUses) {
        return transactions.execute(status -> {
            List<InviteView> created = new ArrayList<>(count);
            for (int i = 0; i < count; i++) {
                String code = generateCode();
                Instant now = Instant.now();
                jdbc.update("""
                    INSERT INTO invites (
                      code, created_at, created_by, used_by, used_at, max_uses, use_count
                    ) VALUES (?, ?, 'admin', '', NULL, ?, 0)
                    """, code, TimestampWriter.format(now), maxUses);
                created.add(new InviteView(code, now, "admin", "", null, maxUses, 0));
            }
            appendAudit(
                "invite_created",
                "codes=" + String.join(",", created.stream().map(InviteView::code).toList())
                    + " count=" + created.size() + " max_uses=" + maxUses
            );
            return created;
        });
    }

    public void delete(String code) {
        transactions.executeWithoutResult(status -> {
            int affected = jdbc.update(
                "DELETE FROM invites WHERE code = ?", code == null ? "" : code);
            if (affected == 0) {
                throw new SidecarException(ErrorCode.NOT_FOUND, "邀请码不存在");
            }
            appendAudit("invite_deleted", "codes=" + code);
        });
    }

    private void appendAudit(String action, String detail) {
        jdbc.update("""
            INSERT INTO audit_log (time, action, detail) VALUES (?, ?, ?)
            """, TimestampWriter.format(Instant.now()), action, detail);
    }

    private String generateCode() {
        byte[] bytes = new byte[8];
        random.nextBytes(bytes);
        return "INV-" + HexFormat.of().withUpperCase().formatHex(bytes);
    }

    private static InviteView view(ResultSet rs) throws SQLException {
        return new InviteView(
            rs.getString("code"),
            TimestampReader.readOrEpoch(rs, "created_at"),
            nullToEmpty(rs.getString("created_by")),
            nullToEmpty(rs.getString("used_by")),
            firstNonNull(TimestampReader.read(rs, "used_at"), GO_ZERO_TIME),
            rs.getInt("max_uses"),
            rs.getInt("use_count")
        );
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    private static <T> T firstNonNull(T value, T fallback) {
        return value == null ? fallback : value;
    }

    public record InviteView(
        String code,
        @JsonProperty("created_at") Instant createdAt,
        @JsonProperty("created_by") String createdBy,
        @JsonInclude(JsonInclude.Include.NON_EMPTY)
        @JsonProperty("used_by") String usedBy,
        @JsonProperty("used_at") Instant usedAt,
        @JsonProperty("max_uses") int maxUses,
        @JsonProperty("use_count") int useCount
    ) {
    }
}
