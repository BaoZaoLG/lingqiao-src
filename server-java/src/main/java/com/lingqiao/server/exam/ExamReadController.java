package com.lingqiao.server.exam;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.dao.DataAccessException;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/exam")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "exam-enabled", havingValue = "true")
public class ExamReadController {
    private static final DateTimeFormatter EXPORT_TIME = DateTimeFormatter.ofPattern("yyyyMMdd_HHmmss");
    private final ExamService service;
    private final AdminAuthService adminAuthService;

    public ExamReadController(ExamService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping("/stats")
    public Map<String, Object> stats(HttpServletRequest request) {
        requireRead(request);
        ExamStats stats;
        try {
            stats = service.stats();
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to load exam stats");
        }
        return CompatResponse.ok(Map.of(
            "total", stats.total(),
            "filled", stats.filled(),
            "correct", stats.correct()
        ));
    }

    @GetMapping("/records")
    public Map<String, Object> records(
        HttpServletRequest request,
        @RequestParam(name = "card_code", required = false) String cardCode,
        @RequestParam(name = "exam_id", required = false) String examId,
        @RequestParam(name = "q", required = false) String question,
        @RequestParam(defaultValue = "1") int page,
        @RequestParam(name = "page_size", defaultValue = "50") int pageSize
    ) {
        requireRead(request);
        ExamFilter filter = new ExamFilter(cardCode, examId, question, page, pageSize);
        ExamPage result;
        try {
            result = service.list(filter);
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to list exam records");
        }
        List<ExamRecordResponse> records = result.records().stream().map(ExamRecordResponse::from).toList();
        Map<String, Object> fields = new LinkedHashMap<>();
        // Go's nil slice is encoded as JSON null when the result set is empty.
        fields.put("records", records.isEmpty() ? null : records);
        fields.put("total", result.total());
        fields.put("page", filter.page());
        fields.put("page_size", filter.pageSize());
        return CompatResponse.ok(fields);
    }

    @GetMapping("/export")
    public void export(
        HttpServletRequest request,
        HttpServletResponse response,
        @RequestParam(name = "card_code", required = false) String cardCode,
        @RequestParam(name = "exam_id", required = false) String examId,
        @RequestParam(name = "q", required = false) String question
    ) throws IOException {
        requireRead(request);
        List<ExamRecord> records;
        try {
            records = service.listAll(new ExamFilter(cardCode, examId, question, 1, 50));
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to export exam records");
        }
        String filename = "exam_records_" + EXPORT_TIME.format(LocalDateTime.now()) + ".csv";
        response.setContentType("text/csv; charset=utf-8");
        response.setHeader("Content-Disposition", "attachment; filename=\"" + filename + "\"");
        response.getOutputStream().write(ExamCsv.export(records));
    }

    private void requireRead(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "exam.read");
    }
}
