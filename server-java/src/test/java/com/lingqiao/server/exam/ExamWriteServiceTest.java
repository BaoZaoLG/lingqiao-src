package com.lingqiao.server.exam;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ExamWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ExamWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE exam_records (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              card_code TEXT NOT NULL, machine_id TEXT NOT NULL,
              exam_id TEXT NOT NULL, course_id TEXT NOT NULL, class_id TEXT NOT NULL,
              question_id TEXT NOT NULL UNIQUE, question_text TEXT NOT NULL, question_type TEXT NOT NULL,
              options_json TEXT NOT NULL, provided_answer TEXT NOT NULL, correct_answer TEXT NOT NULL,
              model_name TEXT NOT NULL, confidence REAL NOT NULL, answer_time_ms INTEGER NOT NULL,
              fill_ok INTEGER NOT NULL, created_at DATETIME NOT NULL
            )
            """);
        service = new ExamWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void insertBatchPersistsAllRecords() {
        int inserted = service.insertBatch(List.of(record("q1"), record("q2")));
        assertThat(inserted).isEqualTo(2);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM exam_records", Integer.class)).isEqualTo(2);
    }

    @Test
    void insertBatchIsAtomicOnFailure() {
        assertThatThrownBy(() -> service.insertBatch(List.of(record("dup"), record("dup"))))
            .isInstanceOf(RuntimeException.class);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM exam_records", Integer.class)).isZero();
    }

    @Test
    void emptyBatchIsNoop() {
        assertThat(service.insertBatch(List.of())).isZero();
    }

    private static ExamRecord record(String questionId) {
        return new ExamRecord(
            0, "C1", "M1", "E1", "course", "class", questionId, "text", "single",
            "[]", "A", "A", "model", 0.9, 120, true, Instant.parse("2026-07-01T00:00:00Z")
        );
    }
}
