package com.lingqiao.server.exam;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;

@JsonIgnoreProperties(ignoreUnknown = true)
public record ExamReportRequest(
    @JsonProperty("session_token") String sessionToken,
    @JsonProperty("exam_id") String examId,
    @JsonProperty("course_id") String courseId,
    @JsonProperty("class_id") String classId,
    @JsonProperty("questions") List<Question> questions
) {
    @JsonIgnoreProperties(ignoreUnknown = true)
    public record Question(
        @JsonProperty("question_id") String questionId,
        @JsonProperty("question_text") String questionText,
        @JsonProperty("question_type") String questionType,
        @JsonProperty("options_json") String optionsJson,
        @JsonProperty("provided_answer") String providedAnswer,
        @JsonProperty("correct_answer") String correctAnswer,
        @JsonProperty("model_name") String modelName,
        @JsonProperty("confidence") double confidence,
        @JsonProperty("answer_time_ms") long answerTimeMs,
        @JsonProperty("fill_ok") boolean fillOk
    ) {
    }
}
