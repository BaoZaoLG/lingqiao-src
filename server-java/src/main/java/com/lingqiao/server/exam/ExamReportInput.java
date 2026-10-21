package com.lingqiao.server.exam;

import java.time.Instant;
import java.util.List;

public record ExamReportInput(
    String cardCode,
    String machineId,
    String examId,
    String courseId,
    String classId,
    List<ExamQuestionInput> questions,
    Instant createdAt
) {
}
