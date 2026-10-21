package com.lingqiao.server.exam;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record ExamRecordResponse(
    long id,
    @JsonProperty("card_code") String cardCode,
    @JsonProperty("machine_id") String machineId,
    @JsonProperty("exam_id") String examId,
    @JsonProperty("course_id") String courseId,
    @JsonProperty("class_id") String classId,
    @JsonProperty("question_id") String questionId,
    @JsonProperty("question_text") String questionText,
    @JsonProperty("question_type") String questionType,
    @JsonProperty("options_json") String optionsJson,
    @JsonProperty("provided_answer") String providedAnswer,
    @JsonProperty("correct_answer") String correctAnswer,
    @JsonProperty("model_name") String modelName,
    double confidence,
    @JsonProperty("answer_time_ms") long answerTimeMs,
    @JsonProperty("fill_ok") boolean fillOk,
    @JsonProperty("created_at") Instant createdAt
) {
    static ExamRecordResponse from(ExamRecord record) {
        return new ExamRecordResponse(
            record.id(), record.cardCode(), record.machineId(), record.examId(), record.courseId(), record.classId(),
            record.questionId(), record.questionText(), record.questionType(), record.optionsJson(),
            record.providedAnswer(), record.correctAnswer(), record.modelName(), record.confidence(),
            record.answerTimeMs(), record.fillOk(), record.createdAt()
        );
    }
}
