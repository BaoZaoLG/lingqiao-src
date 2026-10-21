package com.lingqiao.server.client;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ClientSessionServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ClientSessionService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY,
              machine_id TEXT NOT NULL DEFAULT '',
              status TEXT NOT NULL DEFAULT 'active',
              max_sessions INTEGER NOT NULL DEFAULT 1,
              created_at DATETIME NOT NULL,
              activated_at DATETIME,
              expires_at DATETIME NOT NULL
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
        jdbc.execute("CREATE TABLE blacklist (value TEXT PRIMARY KEY, type TEXT, reason TEXT, created_at DATETIME)");
        service = new ClientSessionService(jdbc, Duration.ofHours(4));
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void firstActivationBindsMachineAndRebasesExpiry() {
        Instant created = Instant.now().minus(Duration.ofDays(2));
        insertCard("ABCDEF-GHJKMN-PQRSTV", "", "active", 1, created, null, created.plus(Duration.ofDays(10)));

        ClientSessionService.ActivationResult result = service.activate(
            new ClientSessionService.ActivationInput("abcdef-ghjkmn-pqrstv", "M1", "fp", "2.4.0", "127.0.0.1"));

        assertThat(result.sessionToken()).hasSize(32);
        assertThat(jdbc.queryForObject(
            "SELECT machine_id FROM cards WHERE code='ABCDEF-GHJKMN-PQRSTV'", String.class)).isEqualTo("M1");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM sessions", Integer.class)).isEqualTo(1);
        assertThat(result.cardExpiresAt()).isAfter(Instant.now().plus(Duration.ofDays(9)));
    }

    @Test
    void secondMachineIsRejectedForSingleSessionCard() {
        Instant now = Instant.now();
        insertCard("CARD-A", "M1", "active", 1, now, now, now.plus(Duration.ofDays(1)));

        assertThatThrownBy(() -> service.activate(
            new ClientSessionService.ActivationInput("CARD-A", "M2", "", "", "")))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("bound to another machine");
    }

    @Test
    void maxSessionsLimitIsEnforced() {
        Instant now = Instant.now();
        insertCard("CARD-B", "M1", "active", 2, now, now, now.plus(Duration.ofDays(1)));
        insertSession("tok-1", "CARD-B", "M1", now.plus(Duration.ofHours(1)));
        insertSession("tok-2", "CARD-B", "M1", now.plus(Duration.ofHours(1)));

        assertThatThrownBy(() -> service.activate(
            new ClientSessionService.ActivationInput("CARD-B", "M1", "", "", "")))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("max sessions reached");
    }

    @Test
    void disabledExpiredAndBlacklistedCardsAreRejected() {
        Instant now = Instant.now();
        insertCard("CARD-D", "", "disabled", 1, now, null, now.plus(Duration.ofDays(1)));
        insertCard("CARD-E", "", "active", 1, now.minus(Duration.ofDays(5)), null, now.minus(Duration.ofDays(1)));
        insertCard("CARD-F", "", "active", 1, now, null, now.plus(Duration.ofDays(1)));
        jdbc.update("INSERT INTO blacklist (value, type, reason, created_at) VALUES ('CARD-F','card','abuse',?)",
            TimestampWriter.format(now));

        assertThatThrownBy(() -> service.activate(new ClientSessionService.ActivationInput("CARD-D", "M1", "", "", "")))
            .hasMessageContaining("card is disabled");
        assertThatThrownBy(() -> service.activate(new ClientSessionService.ActivationInput("CARD-E", "M1", "", "", "")))
            .hasMessageContaining("card is expired");
        assertThatThrownBy(() -> service.activate(new ClientSessionService.ActivationInput("CARD-F", "M1", "", "", "")))
            .hasMessageContaining("card is blacklisted");
    }

    @Test
    void heartbeatExtendsSessionAndValidatesMachine() {
        Instant now = Instant.now();
        insertCard("CARD-G", "M1", "active", 1, now, now, now.plus(Duration.ofDays(1)));
        insertSession("tok-live", "CARD-G", "M1", now.plus(Duration.ofMinutes(5)));

        ClientSessionService.HeartbeatResult result = service.heartbeat(
            new ClientSessionService.HeartbeatInput("tok-live", "M1", "2.4.1"));

        assertThat(result.sessionExpiresAt()).isAfter(now.plus(Duration.ofHours(3)));
        assertThat(jdbc.queryForObject(
            "SELECT client_version FROM sessions WHERE token='tok-live'", String.class)).isEqualTo("2.4.1");

        assertThatThrownBy(() -> service.heartbeat(
            new ClientSessionService.HeartbeatInput("tok-live", "OTHER", "")))
            .hasMessageContaining("machine mismatch");
        assertThatThrownBy(() -> service.heartbeat(
            new ClientSessionService.HeartbeatInput("missing", "M1", "")))
            .hasMessageContaining("session not found");
    }

    @Test
    void expiredSessionHeartbeatIsRejectedAndDeactivateRemovesSession() {
        Instant now = Instant.now();
        insertCard("CARD-H", "M1", "active", 1, now, now, now.plus(Duration.ofDays(1)));
        insertSession("tok-stale", "CARD-H", "M1", now.minus(Duration.ofMinutes(1)));
        insertSession("tok-ok", "CARD-H", "M1", now.plus(Duration.ofHours(1)));

        assertThatThrownBy(() -> service.heartbeat(
            new ClientSessionService.HeartbeatInput("tok-stale", "M1", "")))
            .hasMessageContaining("session expired");

        service.deactivate("tok-ok", "M1");
        assertThat(jdbc.queryForObject(
            "SELECT COUNT(*) FROM sessions WHERE token='tok-ok'", Integer.class)).isZero();
    }

    private void insertCard(String code, String machineId, String status, int maxSessions,
                            Instant createdAt, Instant activatedAt, Instant expiresAt) {
        jdbc.update("""
            INSERT INTO cards (code, machine_id, status, max_sessions, created_at, activated_at, expires_at)
            VALUES (?,?,?,?,?,?,?)
            """, code, machineId, status, maxSessions,
            TimestampWriter.format(createdAt),
            activatedAt == null ? null : TimestampWriter.format(activatedAt),
            TimestampWriter.format(expiresAt));
    }

    private void insertSession(String token, String cardCode, String machineId, Instant expiresAt) {
        Instant now = Instant.now();
        jdbc.update("""
            INSERT INTO sessions (token, card_code, machine_id, created_at, last_seen, expires_at)
            VALUES (?,?,?,?,?,?)
            """, token, cardCode, machineId, TimestampWriter.format(now), TimestampWriter.format(now), TimestampWriter.format(expiresAt));
    }
}
