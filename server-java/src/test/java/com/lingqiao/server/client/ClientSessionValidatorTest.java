package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ClientSessionValidatorTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ClientSessionValidator validator;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE sessions (
              token TEXT PRIMARY KEY, card_code TEXT NOT NULL,
              machine_id TEXT NOT NULL, expires_at DATETIME NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY, status TEXT NOT NULL, expires_at DATETIME NOT NULL
            )
            """);
        validator = new ClientSessionValidator(jdbc);
    }

    @AfterEach
    void tearDown() {
        dataSource.destroy();
    }

    @Test
    void validatesSessionBindingAndCurrentCardLifecycle() {
        insert("active", Instant.now().plusSeconds(3600));

        var session = validator.validateBound("tok", "M1", "card-1");

        assertThat(session.cardCode()).isEqualTo("CARD-1");
        assertThat(session.machineId()).isEqualTo("M1");
    }

    @Test
    void tokenIsValidatedBeforeRequiredMachineLikeGo() {
        assertThatThrownBy(() -> validator.validateBound("", "", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessage("session_token is required");
        assertThatThrownBy(() -> validator.validateBound("tok", "", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessage("machine_id is required");
    }

    @Test
    void rejectsCardsDisabledAfterSessionCreation() {
        insert("disabled", Instant.now().plusSeconds(3600));

        assertThatThrownBy(() -> validator.validateBound("tok", "M1", "CARD-1"))
            .isInstanceOf(SidecarException.class)
            .hasMessage("card disabled");
    }

    private void insert(String status, Instant cardExpiry) {
        jdbc.update("INSERT INTO cards (code, status, expires_at) VALUES ('CARD-1', ?, ?)",
            status, cardExpiry.toString());
        jdbc.update("""
            INSERT INTO sessions (token, card_code, machine_id, expires_at)
            VALUES ('tok', 'CARD-1', 'M1', ?)
            """, Instant.now().plusSeconds(3600).toString());
    }
}
