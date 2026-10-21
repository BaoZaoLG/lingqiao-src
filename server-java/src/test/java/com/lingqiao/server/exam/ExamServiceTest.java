package com.lingqiao.server.exam;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;

class ExamServiceTest {
    @Test
    void statsDelegatesToReadOnlyRepositoryForLowRiskSample() {
        FakeExamRepository repository = new FakeExamRepository();
        repository.stats = new ExamStats(3, 2, 1);
        ExamService service = new ExamService(repository);

        ExamStats stats = service.stats();

        assertThat(stats.total()).isEqualTo(3);
        assertThat(stats.filled()).isEqualTo(2);
        assertThat(stats.correct()).isEqualTo(1);
        assertThat(repository.inserted).isEmpty();
    }

    @Test
    void reportInsertsRecordsWithSessionCardAndMachine() {
        FakeExamRepository repository = new FakeExamRepository();
        ExamService service = new ExamService(repository);
        ExamQuestionInput question = new ExamQuestionInput(
            "q1", "text", "single", "[]", "A", "A", "model", 0.9, 1200, true);

        int inserted = service.report(new ExamReportInput(
            "CARD-1", "MACHINE-1", "exam", "course", "class", List.of(question), Instant.parse("2026-07-14T00:00:00Z")));

        assertThat(inserted).isEqualTo(1);
        assertThat(repository.inserted).hasSize(1);
        ExamRecord record = repository.inserted.get(0);
        assertThat(record.cardCode()).isEqualTo("CARD-1");
        assertThat(record.machineId()).isEqualTo("MACHINE-1");
        assertThat(record.questionId()).isEqualTo("q1");
        assertThat(record.fillOk()).isTrue();
    }

    private static final class FakeExamRepository implements ExamRepository {
        private final List<ExamRecord> inserted = new ArrayList<>();
        private ExamStats stats = new ExamStats(0, 0, 0);

        @Override
        public void insert(ExamRecord record) {
            inserted.add(record);
        }

        @Override
        public ExamStats stats() {
            return stats;
        }
    }
}
