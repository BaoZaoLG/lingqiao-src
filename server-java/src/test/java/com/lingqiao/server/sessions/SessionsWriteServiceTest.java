package com.lingqiao.server.sessions;

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

class SessionsWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private SessionsWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE sessions (
              token TEXT PRIMARY KEY,
              card_code TEXT NOT NULL,
              machine_id TEXT NOT NULL,
              expires_at DATETIME NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY,
              machine_id TEXT NOT NULL DEFAULT '',
              activated_at DATETIME,
              expires_at DATETIME NOT NULL
            )
            """);
        service = new SessionsWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void forceLogoutDeletesSessionWithoutTouchingCard() {
        insertSession("tok-1", "CARD-1", "M1");
        insertCard("CARD-1", "M1", Instant.now().minusSeconds(3600), Instant.now().plusSeconds(3600));

        service.forceLogout("tok-1");

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM sessions", Integer.class)).isZero();
        assertThat(jdbc.queryForObject("SELECT machine_id FROM cards WHERE code = 'CARD-1'", String.class))
            .isEqualTo("M1");
    }

    @Test
    void deactivateWithUnbindPreservesRemainingDuration() {
        Instant activatedAt = Instant.parse("2026-07-01T00:00:00Z");
        Instant expiresAt = activatedAt.plus(Duration.ofHours(10));
        insertSession("tok-2", "CARD-2", "M2");
        insertCard("CARD-2", "M2", activatedAt, expiresAt);

        Instant before = Instant.now();
        service.deactivate(new SessionsWriteService.DeactivateInput("tok-2", "M2", "CARD-2", true, false));

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM sessions", Integer.class)).isZero();
        assertThat(jdbc.queryForObject("SELECT machine_id FROM cards WHERE code = 'CARD-2'", String.class)).isEmpty();
        assertThat(jdbc.queryForObject("SELECT activated_at FROM cards WHERE code = 'CARD-2'", Object.class)).isNull();

        Instant newExpiry = SessionsWriteService.toInstant(
            jdbc.queryForObject("SELECT expires_at FROM cards WHERE code = 'CARD-2'", Object.class));
        assertThat(newExpiry).isNotNull();
        assertThat(Duration.between(before.plus(Duration.ofHours(10)), newExpiry).abs())
            .isLessThan(Duration.ofMinutes(1));
    }

    @Test
    void ownershipMismatchIsRejectedAndSessionKept() {
        insertSession("tok-3", "CARD-3", "M3");

        assertThatThrownBy(() -> service.deactivate(
            new SessionsWriteService.DeactivateInput("tok-3", "OTHER", null, false, false)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("machine mismatch");
        assertThatThrownBy(() -> service.deactivate(
            new SessionsWriteService.DeactivateInput("tok-3", null, "OTHER-CARD", false, false)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("session card mismatch");

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM sessions", Integer.class)).isEqualTo(1);
    }

    @Test
    void missingTokenAndSessionAreRejected() {
        assertThatThrownBy(() -> service.forceLogout(" "))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("session_token is required");
        assertThatThrownBy(() -> service.forceLogout("unknown"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("session not found");
    }

    private void insertSession(String token, String cardCode, String machineId) {
        jdbc.update("INSERT INTO sessions (token, card_code, machine_id, expires_at) VALUES (?, ?, ?, ?)",
            token, cardCode, machineId, Timestamp.from(Instant.now().plusSeconds(3600)));
    }

    private void insertCard(String code, String machineId, Instant activatedAt, Instant expiresAt) {
        jdbc.update("INSERT INTO cards (code, machine_id, activated_at, expires_at) VALUES (?, ?, ?, ?)",
            code, machineId, Timestamp.from(activatedAt), Timestamp.from(expiresAt));
    }
}
