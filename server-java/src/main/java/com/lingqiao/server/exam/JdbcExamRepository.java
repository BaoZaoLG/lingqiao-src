package com.lingqiao.server.exam;

import com.lingqiao.server.common.jdbc.TimestampReader;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

public final class JdbcExamRepository implements ExamRepository {
    private static final RowMapper<ExamRecord> ROW_MAPPER = JdbcExamRepository::mapRecord;
    private final JdbcTemplate jdbc;

    public JdbcExamRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public ExamStats stats() {
        return jdbc.queryForObject("""
            SELECT COUNT(*), COALESCE(SUM(fill_ok), 0),
                   COALESCE(SUM(CASE WHEN correct_answer != '' AND correct_answer = provided_answer THEN 1 ELSE 0 END), 0)
            FROM exam_records
            """, (rs, rowNum) -> new ExamStats(rs.getLong(1), rs.getLong(2), rs.getLong(3)));
    }

    @Override
    public ExamPage list(ExamFilter filter) {
        List<Object> args = new ArrayList<>();
        String where = buildWhere(filter, args);
        Long total = jdbc.queryForObject("SELECT COUNT(*) FROM exam_records " + where, Long.class, args.toArray());
        List<Object> queryArgs = new ArrayList<>(args);
        queryArgs.add(filter.pageSize());
        queryArgs.add((filter.page() - 1) * filter.pageSize());
        List<ExamRecord> records = jdbc.query("""
            SELECT id, card_code, machine_id, exam_id, course_id, class_id,
                   question_id, question_text, question_type, options_json,
                   provided_answer, correct_answer, model_name, confidence,
                   answer_time_ms, fill_ok, created_at
            FROM exam_records %s ORDER BY created_at DESC LIMIT ? OFFSET ?
            """.formatted(where), ROW_MAPPER, queryArgs.toArray());
        return new ExamPage(records, total == null ? 0 : total);
    }

    @Override
    public List<ExamRecord> listAll(ExamFilter filter) {
        List<Object> args = new ArrayList<>();
        String where = buildWhere(filter, args);
        return jdbc.query("""
            SELECT id, card_code, machine_id, exam_id, course_id, class_id,
                   question_id, question_text, question_type, options_json,
                   provided_answer, correct_answer, model_name, confidence,
                   answer_time_ms, fill_ok, created_at
            FROM exam_records %s ORDER BY created_at DESC
            """.formatted(where), ROW_MAPPER, args.toArray());
    }

    private static String buildWhere(ExamFilter filter, List<Object> args) {
        List<String> clauses = new ArrayList<>();
        if (hasText(filter.cardCode())) {
            clauses.add("card_code = ?");
            args.add(filter.cardCode());
        }
        if (hasText(filter.examId())) {
            clauses.add("exam_id = ?");
            args.add(filter.examId());
        }
        if (hasText(filter.question())) {
            clauses.add("(question_text LIKE ? OR question_id = ?)");
            args.add("%" + filter.question() + "%");
            args.add(filter.question());
        }
        return clauses.isEmpty() ? "" : "WHERE " + String.join(" AND ", clauses);
    }

    private static boolean hasText(String value) {
        return value != null && !value.isBlank();
    }

    private static ExamRecord mapRecord(ResultSet rs, int rowNum) throws SQLException {
        Instant createdAt = TimestampReader.readOrEpoch(rs, "created_at");
        return new ExamRecord(
            rs.getLong("id"), rs.getString("card_code"), rs.getString("machine_id"), rs.getString("exam_id"),
            rs.getString("course_id"), rs.getString("class_id"), rs.getString("question_id"),
            rs.getString("question_text"), rs.getString("question_type"), rs.getString("options_json"),
            rs.getString("provided_answer"), rs.getString("correct_answer"), rs.getString("model_name"),
            rs.getDouble("confidence"), rs.getLong("answer_time_ms"), rs.getInt("fill_ok") != 0, createdAt
        );
    }
}
