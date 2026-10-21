package com.lingqiao.server.exam;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.DriverManager;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class JdbcExamRepositoryTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private JdbcExamRepository repository;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE exam_records (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              card_code TEXT NOT NULL, machine_id TEXT NOT NULL,
              exam_id TEXT NOT NULL, course_id TEXT NOT NULL, class_id TEXT NOT NULL,
              question_id TEXT NOT NULL, question_text TEXT NOT NULL, question_type TEXT NOT NULL,
              options_json TEXT NOT NULL, provided_answer TEXT NOT NULL, correct_answer TEXT NOT NULL,
              model_name TEXT NOT NULL, confidence REAL NOT NULL, answer_time_ms INTEGER NOT NULL,
              fill_ok INTEGER NOT NULL, created_at DATETIME NOT NULL
            )
            """);
        repository = new JdbcExamRepository(jdbc);
    }

    @AfterEach
    void tearDown() {
        dataSource.destroy();
    }

    @Test
    void listAllUsesAllGoFiltersAndNewestFirstWithoutPagination() {
        insert("C1", "E1", "q1", "needle old", "2026-07-01 00:00:00");
        insert("C1", "E1", "needle", "other new", "2026-07-02 00:00:00");
        insert("C2", "E1", "q3", "needle excluded", "2026-07-03 00:00:00");

        var records = repository.listAll(new ExamFilter("C1", "E1", "needle", 1, 50));

        assertThat(records).extracting(ExamRecord::questionId).containsExactly("needle", "q1");
    }

    private void insert(String card, String exam, String questionId, String text, String createdAt) {
        jdbc.update("""
            INSERT INTO exam_records
              (card_code, machine_id, exam_id, course_id, class_id, question_id, question_text,
               question_type, options_json, provided_answer, correct_answer, model_name,
               confidence, answer_time_ms, fill_ok, created_at)
            VALUES (?, 'M1', ?, 'course', 'class', ?, ?, 'single', '[]', 'A', 'A', 'model',
                    0.9, 120, 1, ?)
            """, card, exam, questionId, text, createdAt);
    }
}
