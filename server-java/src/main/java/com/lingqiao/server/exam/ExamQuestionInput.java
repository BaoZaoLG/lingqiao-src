package com.lingqiao.server.exam;

public record ExamQuestionInput(
    String questionId,
    String questionText,
    String questionType,
    String optionsJson,
    String providedAnswer,
    String correctAnswer,
    String modelName,
    double confidence,
    long answerTimeMs,
    boolean fillOk
) {
}
