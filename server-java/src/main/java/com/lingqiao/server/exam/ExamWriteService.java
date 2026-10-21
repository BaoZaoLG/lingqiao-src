package com.lingqiao.server.exam;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.sql.PreparedStatement;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.util.List;
import java.util.Objects;
import org.springframework.jdbc.core.BatchPreparedStatementSetter;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Java write path for exam records. Mirrors Go exam.Repository.InsertBatch atomicity.
 */
public class ExamWriteService {
    private static final String INSERT_SQL = """
        INSERT INTO exam_records
            (card_code, machine_id, exam_id, course_id, class_id,
             question_id, question_text, question_type, options_json,
             provided_answer, correct_answer, model_name, confidence,
             answer_time_ms, fill_ok, created_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        """;

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;

    public ExamWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public int insertBatch(List<ExamRecord> records) {
        if (records == null || records.isEmpty()) {
            return 0;
        }
        return Objects.requireNonNull(transactions.execute(status -> insertBatchInTransaction(records)));
    }

    private int insertBatchInTransaction(List<ExamRecord> records) {
        jdbc.batchUpdate(INSERT_SQL, new BatchPreparedStatementSetter() {
            @Override
            public void setValues(PreparedStatement ps, int i) throws SQLException {
                ExamRecord record = records.get(i);
                ps.setString(1, nullToEmpty(record.cardCode()));
                ps.setString(2, nullToEmpty(record.machineId()));
                ps.setString(3, nullToEmpty(record.examId()));
                ps.setString(4, nullToEmpty(record.courseId()));
                ps.setString(5, nullToEmpty(record.classId()));
                ps.setString(6, nullToEmpty(record.questionId()));
                ps.setString(7, nullToEmpty(record.questionText()));
                ps.setString(8, nullToEmpty(record.questionType()));
                ps.setString(9, nullToEmpty(record.optionsJson()));
                ps.setString(10, nullToEmpty(record.providedAnswer()));
                ps.setString(11, nullToEmpty(record.correctAnswer()));
                ps.setString(12, nullToEmpty(record.modelName()));
                ps.setDouble(13, record.confidence());
                ps.setLong(14, record.answerTimeMs());
                ps.setInt(15, record.fillOk() ? 1 : 0);
                ps.setString(16, TimestampWriter.format(record.createdAt()));
            }

            @Override
            public int getBatchSize() {
                return records.size();
            }
        });
        return records.size();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
