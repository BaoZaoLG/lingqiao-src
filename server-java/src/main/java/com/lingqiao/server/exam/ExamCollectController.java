package com.lingqiao.server.exam;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.dao.DataAccessException;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/api/v1/exam")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "exam-write-enabled", havingValue = "true")
public class ExamCollectController {
    private final ExamWriteService writeService;
    private final ExamSessionLookup sessions;
    private final ObjectMapper mapper =
        new ObjectMapper().enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    public ExamCollectController(ExamWriteService writeService, ExamSessionLookup sessions) {
        this.writeService = writeService;
        this.sessions = sessions;
    }

    @PostMapping("/collect")
    public Map<String, Object> collect(@RequestBody(required = false) String rawBody) {
        ExamReportRequest payload = parse(rawBody);
        ExamSessionLookup.ExamSession session = sessions.requireActive(payload.sessionToken());
        int inserted;
        try {
            inserted = writeService.insertBatch(ExamReportController.toRecords(payload, session, Instant.now()));
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
}
