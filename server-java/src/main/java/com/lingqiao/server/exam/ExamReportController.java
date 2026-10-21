package com.lingqiao.server.exam;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.dao.DataAccessException;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * HMAC-authenticated exam ingestion, mirroring Go exam.Handler.HandleReport.
 * The raw request body is used for signature verification exactly like Go.
 */
@RestController
@RequestMapping("/java/api/v1/exam")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "exam-write-enabled", havingValue = "true")
public class ExamReportController {
    private final ExamWriteService writeService;
    private final ExamSessionLookup sessions;
    private final HmacAuthService hmac;
    private final ObjectMapper mapper =
        new ObjectMapper().enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    public ExamReportController(ExamWriteService writeService, ExamSessionLookup sessions, HmacAuthService hmac) {
        this.writeService = writeService;
        this.sessions = sessions;
        this.hmac = hmac;
    }

    @PostMapping("/report")
    public Map<String, Object> report(
        HttpServletRequest request,
        @RequestBody(required = false) String rawBody
    ) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce")
        );
        ExamReportRequest payload = parse(rawBody);
        ExamSessionLookup.ExamSession session = sessions.requireActive(payload.sessionToken());
        List<ExamRecord> records = toRecords(payload, session, Instant.now());
        int inserted;
        try {
            inserted = writeService.insertBatch(records);
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to save exam records");
        }
        return CompatResponse.ok(Map.of("inserted", inserted));
    }

    private ExamReportRequest parse(String rawBody) {
        try {
            ExamReportRequest parsed = mapper.readValue(rawBody, ExamReportRequest.class);
            return parsed == null ? new ExamReportRequest(null, null, null, null, List.of()) : parsed;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "invalid json");
        }
    }

    static List<ExamRecord> toRecords(ExamReportRequest payload, ExamSessionLookup.ExamSession session, Instant now) {
        List<ExamRecord> records = new ArrayList<>();
        if (payload.questions() == null) {
            return records;
        }
        for (ExamReportRequest.Question q : payload.questions()) {
            records.add(new ExamRecord(
                0,
                session.cardCode(),
                session.machineId(),
                nullToEmpty(payload.examId()),
                nullToEmpty(payload.courseId()),
                nullToEmpty(payload.classId()),
                nullToEmpty(q.questionId()),
                nullToEmpty(q.questionText()),
                nullToEmpty(q.questionType()),
                nullToEmpty(q.optionsJson()),
                nullToEmpty(q.providedAnswer()),
                nullToEmpty(q.correctAnswer()),
                nullToEmpty(q.modelName()),
                q.confidence(),
                q.answerTimeMs(),
                q.fillOk(),
                now
            ));
        }
        return records;
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
