package com.lingqiao.server.authorizations;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardStatuses;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AuthorizationControlServiceTest {
    private static final Instant NOW = Instant.parse("2026-07-02T12:00:00Z");

    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AuthorizationControlService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY,
              machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL,
              activated_at DATETIME,
              expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active',
              note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        jdbc.execute("""
            CREATE TABLE sessions (
              token TEXT PRIMARY KEY,
              card_code TEXT NOT NULL,
              machine_id TEXT NOT NULL,
              fingerprint TEXT NOT NULL DEFAULT '',
              client_version TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL,
              last_seen DATETIME NOT NULL,
              expires_at DATETIME NOT NULL,
              remote_addr TEXT NOT NULL DEFAULT ''
            )
            """);
        jdbc.execute("""
            CREATE TABLE audit_log (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              time DATETIME NOT NULL,
              action TEXT NOT NULL,
              card TEXT NOT NULL DEFAULT '',
              detail TEXT NOT NULL DEFAULT ''
            )
            """);
        service = new AuthorizationControlService(jdbc, () -> NOW);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void listComputesAllLifecycleStatesAndHidesArchived() {
        insertCard("ISSUED", "", null, NOW.plus(Duration.ofHours(24)), "active", "", 1);
        insertCard("BOUND", "M1", NOW.minus(Duration.ofHours(2)), NOW.plus(Duration.ofHours(24)), "active", "", 1);
        insertCard("INUSE", "M2", NOW.minus(Duration.ofHours(2)), NOW.plus(Duration.ofHours(24)), "active", "", 1);
        insertCard("EXPIRED", "", null, NOW.minusSeconds(1), "active", "", 1);
        insertCard("FROZEN", "", null, NOW.plus(Duration.ofHours(24)), "disabled", "", 1);
        insertCard("ARCHIVED", "", null, NOW.plus(Duration.ofHours(24)), "archived", "", 1);
        insertSession("T1", "INUSE", "M2", NOW.minusSeconds(60), NOW.plusSeconds(3600));

        List<AuthorizationControlService.AuthorizationSummary> result =
            service.list(emptyFilter());

        assertThat(result).extracting(AuthorizationControlService.AuthorizationSummary::code)
            .doesNotContain("ARCHIVED");
        assertThat(status(result, "ISSUED")).isEqualTo("issued");
        assertThat(status(result, "BOUND")).isEqualTo("activated");
        assertThat(status(result, "INUSE")).isEqualTo("in_use");
        assertThat(status(result, "EXPIRED")).isEqualTo("expired");
        assertThat(status(result, "FROZEN")).isEqualTo("frozen");
        assertThat(result.stream().filter(item -> item.code().equals("INUSE")).findFirst().orElseThrow()
            .lastSeen()).isEqualTo(NOW.minusSeconds(60));
    }

    @Test
    void listAppliesGoFilterBoundariesAndCanIncludeArchivedInternally() {
        insertCard("MATCH", "M1", null, NOW.plus(Duration.ofHours(2)), "active", "needle", 2);
        insertCard("UNBOUND", "", null, NOW.plus(Duration.ofHours(2)), "active", "needle", 2);
        insertCard("TOO-LATE", "M2", null, NOW.plus(Duration.ofHours(48)), "active", "needle", 2);
        insertCard("ARCHIVED", "M3", null, NOW.plus(Duration.ofHours(2)), "archived", "needle", 2);

        AuthorizationControlService.ListFilter filter =
            new AuthorizationControlService.ListFilter(
                "needle", "activated", "", "", "true",
                NOW.plus(Duration.ofHours(2)), NOW.plus(Duration.ofHours(24)), 2, false);

        assertThat(service.list(filter))
            .extracting(AuthorizationControlService.AuthorizationSummary::code)
            .containsExactly("MATCH");
        assertThat(service.list(new AuthorizationControlService.ListFilter(
            "", "", "", "", "", null, null, 0, true)))
            .extracting(AuthorizationControlService.AuthorizationSummary::code)
            .contains("ARCHIVED");
    }

    @Test
    void detailIsFlattenedAndIncludesOnlyActiveSessions() throws Exception {
        insertCard("DETAIL", "M1", null, NOW.plus(Duration.ofHours(24)), "active", "", 2);
        insertSession("ACTIVE", "DETAIL", "M1", NOW.minusSeconds(30), NOW.plusSeconds(3600));
        insertSession("OLD", "DETAIL", "M1", NOW.minusSeconds(90), NOW.minusSeconds(1));

        AuthorizationControlService.AuthorizationDetail detail = service.detail("DETAIL");
        ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());
        String json = mapper.writeValueAsString(detail);

        assertThat(detail.sessions()).extracting(session -> session.token())
            .containsExactly("ACTIVE");
        assertThat(json).contains("\"code\":\"DETAIL\"", "\"active_sessions\":1");
        assertThat(json).doesNotContain("\"summary\"");
        assertThatThrownBy(() -> service.detail("MISSING"))
            .isInstanceOf(SidecarException.class)
            .hasMessage("authorization not found");
    }

    @Test
    void previewCountsSessionsSortsMachinesAndPreservesArchiveWarning() {
        insertCard("A", "Z-MACHINE", null, NOW.plus(Duration.ofHours(24)), "active", "", 1);
        insertCard("B", "A-MACHINE", null, NOW.plus(Duration.ofHours(24)), "active", "", 1);
        insertSession("TA", "A", "Z-MACHINE", NOW.minusSeconds(30), NOW.plusSeconds(10));

        AuthorizationControlService.ImpactPreview preview =
            service.preview(List.of("A", "B"), "archive_delete", 0);

        assertThat(preview.targetCount()).isEqualTo(2);
        assertThat(preview.activeSessions()).isEqualTo(1);
        assertThat(preview.machines()).containsExactly("A-MACHINE", "Z-MACHINE");
        assertThat(preview.reversible()).isTrue();
        assertThat(preview.warnings()).containsExactly(
            "授权将从默认列表隐藏，客户端不可继续使用，但历史记录会保留");
    }

    @Test
    void executeMatchesPartialSuccessDefaultsAndLifecycleMutations() {
        insertCard("EXTEND", "", null, NOW.plus(Duration.ofHours(1)), "expired", "", 1);
        insertCard("BOUND", "M1", NOW.minus(Duration.ofHours(2)), NOW.plus(Duration.ofHours(3)), "active", "", 1);
        insertCard("ACTIVE", "", null, NOW.plus(Duration.ofHours(1)), "active", "", 1);

        AuthorizationControlService.ActionResult extend =
            service.execute(List.of("EXTEND", "MISSING"), "extend", 0);
        assertThat(extend.updated()).isEqualTo(1);
        assertThat(extend.failed()).isEqualTo(1);
        assertThat(extend.codes()).containsExactly("EXTEND");
        Card extended = card("EXTEND");
        assertThat(extended.status()).isEqualTo(CardStatuses.ACTIVE);
        assertThat(extended.expiresAt()).isEqualTo(NOW.plus(Duration.ofHours(721)));

        service.execute(List.of("BOUND"), "unbind_machine", 0);
        Card unbound = card("BOUND");
        assertThat(unbound.machineId()).isEmpty();
        assertThat(unbound.activatedAt()).isNull();
        assertThat(unbound.expiresAt()).isEqualTo(
            NOW.minus(Duration.ofHours(24)).plus(Duration.ofHours(5)));

        assertThat(service.execute(List.of("ACTIVE"), "freeze", 0).updated()).isEqualTo(1);
        assertThat(service.execute(List.of("ACTIVE"), "freeze", 0).skipped()).isEqualTo(1);
        assertThat(service.execute(List.of("ACTIVE"), "restore", 0).updated()).isEqualTo(1);
        assertThat(service.execute(List.of("ACTIVE"), "archive_delete", 0).updated()).isEqualTo(1);
        assertThat(card("ACTIVE").status()).isEqualTo(CardStatuses.ARCHIVED);

        AuthorizationControlService.ActionResult unknown =
            service.execute(List.of("ACTIVE"), "unknown", 0);
        assertThat(unknown.failed()).isEqualTo(1);
        assertThat(unknown.codes()).isNull();
    }

    @Test
    void authorizationAuditFailureIsBestEffortLikeGo() {
        jdbc.execute("DROP TABLE audit_log");
        service.recordAudit("authorization_frozen", "A", "Action: freeze");
    }

    private AuthorizationControlService.ListFilter emptyFilter() {
        return new AuthorizationControlService.ListFilter(
            "", "", "", "", "", null, null, 0, false);
    }

    private static String status(
        List<AuthorizationControlService.AuthorizationSummary> items, String code
    ) {
        return items.stream().filter(item -> item.code().equals(code))
            .findFirst().orElseThrow().effectiveStatus();
    }

    private void insertCard(
        String code,
        String machineId,
        Instant activatedAt,
        Instant expiresAt,
        String status,
        String note,
        int maxSessions
    ) {
        jdbc.update("""
            INSERT INTO cards (
              code, machine_id, agent_id, created_at, activated_at,
              expires_at, status, note, max_sessions
            ) VALUES (?, ?, '', ?, ?, ?, ?, ?, ?)
            """,
            code,
            machineId,
            NOW.minus(Duration.ofHours(24)).toString(),
            activatedAt == null ? null : activatedAt.toString(),
            expiresAt.toString(),
            status,
            note,
            maxSessions);
    }

    private void insertSession(
        String token, String code, String machineId, Instant lastSeen, Instant expiresAt
    ) {
        jdbc.update("""
            INSERT INTO sessions (
              token, card_code, machine_id, fingerprint, client_version,
              created_at, last_seen, expires_at, remote_addr
            ) VALUES (?, ?, ?, '', '', ?, ?, ?, '')
            """,
            token, code, machineId, NOW.minus(Duration.ofHours(1)).toString(),
            lastSeen.toString(), expiresAt.toString());
    }

    private Card card(String code) {
        return jdbc.queryForObject("""
            SELECT code, machine_id, agent_id, created_at, activated_at,
                   expires_at, status, note, max_sessions
            FROM cards WHERE code = ?
            """, (rs, rowNum) -> new Card(
                rs.getString("code"),
                rs.getString("machine_id"),
                rs.getString("agent_id"),
                Instant.parse(rs.getString("created_at")),
                rs.getString("activated_at") == null
                    ? null : Instant.parse(rs.getString("activated_at")),
                Instant.parse(rs.getString("expires_at")),
                rs.getString("status"),
                rs.getString("note"),
                rs.getInt("max_sessions")
            ), code);
    }
}
