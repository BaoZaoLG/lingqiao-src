package com.lingqiao.server.exam;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ExamReportIngestionTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ExamSessionLookup sessions;

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
        sessions = new ExamSessionLookup(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void resolvesActiveSessionCardAndMachine() {
        insertSession("tok-1", "CARD-1", "M1", Instant.now().plusSeconds(3600));

        ExamSessionLookup.ExamSession session = sessions.requireActive("tok-1");

        assertThat(session.cardCode()).isEqualTo("CARD-1");
        assertThat(session.machineId()).isEqualTo("M1");
    }

    @Test
    void rejectsMissingBlankAndExpiredSessions() {
        insertSession("expired", "CARD-2", "M2", Instant.now().minusSeconds(60));

        assertThatThrownBy(() -> sessions.requireActive(null))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("session_token required");
        assertThatThrownBy(() -> sessions.requireActive("unknown"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid session");
        assertThatThrownBy(() -> sessions.requireActive(" "))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid session");
        assertThatThrownBy(() -> sessions.requireActive("expired"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid session");
    }

    @Test
    void recordsInheritSessionIdentityLikeGo() {
        ExamReportRequest payload = new ExamReportRequest(
            "tok-1", "exam-1", "course-1", "class-1",
            List.of(new ExamReportRequest.Question(
                "q1", "text", "single", "[]", "A", "A", "model", 0.9, 120, true))
        );
        ExamSessionLookup.ExamSession session = new ExamSessionLookup.ExamSession("CARD-9", "M9");
        Instant now = Instant.parse("2026-07-01T00:00:00Z");

        List<ExamRecord> records = ExamReportController.toRecords(payload, session, now);

        assertThat(records).hasSize(1);
        ExamRecord record = records.get(0);
        assertThat(record.cardCode()).isEqualTo("CARD-9");
        assertThat(record.machineId()).isEqualTo("M9");
        assertThat(record.examId()).isEqualTo("exam-1");
        assertThat(record.questionId()).isEqualTo("q1");
        assertThat(record.fillOk()).isTrue();
        assertThat(record.createdAt()).isEqualTo(now);
    }

    private void insertSession(String token, String cardCode, String machineId, Instant expiresAt) {
        jdbc.update("INSERT INTO sessions (token, card_code, machine_id, expires_at) VALUES (?, ?, ?, ?)",
            token, cardCode, machineId, Timestamp.from(expiresAt));
    }
}
