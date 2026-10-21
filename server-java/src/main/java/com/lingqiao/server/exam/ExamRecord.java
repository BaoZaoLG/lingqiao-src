package com.lingqiao.server.exam;

import java.time.Instant;

public record ExamRecord(
    long id,
    String cardCode,
    String machineId,
    String examId,
    String courseId,
    String classId,
    String questionId,
    String questionText,
    String questionType,
    String optionsJson,
    String providedAnswer,
    String correctAnswer,
    String modelName,
    double confidence,
    long answerTimeMs,
    boolean fillOk,
    Instant createdAt
) {
}
