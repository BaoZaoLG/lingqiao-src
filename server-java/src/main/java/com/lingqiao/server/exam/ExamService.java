package com.lingqiao.server.exam;

import java.util.ArrayList;
import java.util.List;

public class ExamService {
    private final ExamRepository repository;

    public ExamService(ExamRepository repository) {
        this.repository = repository;
    }

    public ExamStats stats() {
        return repository.stats();
    }

    public ExamPage list(ExamFilter filter) {
        return repository.list(filter);
    }

    public List<ExamRecord> listAll(ExamFilter filter) {
        return repository.listAll(filter);
    }

    int report(ExamReportInput input) {
        List<ExamRecord> records = new ArrayList<>(input.questions().size());
        for (ExamQuestionInput question : input.questions()) {
            records.add(new ExamRecord(
                0, input.cardCode(), input.machineId(), input.examId(), input.courseId(), input.classId(),
                question.questionId(), question.questionText(), question.questionType(), question.optionsJson(),
                question.providedAnswer(), question.correctAnswer(), question.modelName(), question.confidence(),
                question.answerTimeMs(), question.fillOk(), input.createdAt()
            ));
        }
        records.forEach(repository::insert);
        return records.size();
    }
}
