package com.lingqiao.server.cards;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping({"/java/admin/api/cards", "/java/admin/api/card"})
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-write-enabled", havingValue = "true")
public class CardsWriteController {
    private final CardsWriteService service;
    private final CardsCsvService csvService;
    private final AdminAuthService adminAuthService;

    public CardsWriteController(
        CardsWriteService service,
        CardsCsvService csvService,
        AdminAuthService adminAuthService
    ) {
        this.service = service;
        this.csvService = csvService;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/generate")
    public Map<String, Object> generate(HttpServletRequest request, @RequestBody GenerateRequest body) {
        requireWrite(request);
        int duration = clamp(body.duration_hours(), 1, 17520, 720);
        int maxSessions = clamp(body.max_sessions(), 1, 100, 1);
        Card card = service.create(Duration.ofHours(duration), truncateNote(body.note()), maxSessions, "");
        return CompatResponse.ok(Map.of("card", card));
    }

    @PostMapping("/batch-generate")
    public Map<String, Object> batchGenerate(HttpServletRequest request, @RequestBody BatchGenerateRequest body) {
        requireWrite(request);
        int count = clamp(body.count(), 1, 500, 1);
        int duration = clamp(body.duration_hours(), 1, 17520, 720);
        int maxSessions = clamp(body.max_sessions(), 1, 100, 1);
        List<Card> cards = service.batchCreate(count, Duration.ofHours(duration), truncateNote(body.note()), maxSessions, "");
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("cards", cards);
        response.put("count", cards.size());
        return CompatResponse.ok(response);
    }

    @PostMapping("/update")
    public Map<String, Object> update(HttpServletRequest request, @RequestBody UpdateRequest body) {
        requireWrite(request);
        String action = body.action() == null ? "" : body.action();
        switch (action) {
            case "disable" -> service.updateStatus(body.code(), CardStatuses.DISABLED);
            case "enable" -> service.updateStatus(body.code(), CardStatuses.ACTIVE);
            case "expire" -> service.updateStatus(body.code(), CardStatuses.EXPIRED);
            case "extend" -> service.extend(body.code(), body.extend_hours());
            case "unbind" -> service.unbind(body.code());
            case "delete" -> service.delete(body.code());
            default -> throw new SidecarException(ErrorCode.BAD_REQUEST, "unknown action");
        }
        return CompatResponse.ok((Map<String, ?>) null);
    }

    @PostMapping("/update-details")
    public Map<String, Object> updateDetails(HttpServletRequest request, @RequestBody UpdateDetailsRequest body) {
        requireWrite(request);
        service.updateDetails(body.code(), body.note(), body.max_sessions());
        return CompatResponse.ok((Map<String, ?>) null);
    }

    @PostMapping("/bulk")
    public Map<String, Object> bulk(HttpServletRequest request, @RequestBody BulkRequest body) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.bulk");
        if (body.codes() == null || body.codes().isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "codes is required");
        }
        String action = body.action() == null ? "" : body.action();
        CardsWriteService.BulkResult result =
            service.bulkUpdateDetailed(body.codes(), action, body.extend_hours(), body.dry_run());
        if (!body.dry_run()) {
            csvService.recordBulkAudit(action, result.updated());
        }
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("affected", result.updated());
        response.put("dry_run", body.dry_run());
        response.put("result", result);
        return CompatResponse.ok(response);
    }

    @PostMapping(path = "/import", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> importCards(
        HttpServletRequest request, @RequestBody ImportRequest body
    ) {
        requireWrite(request);
        if (body.csv() == null || body.csv().isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "CSV data is required");
        }
        int duration = clamp(body.duration(), 1, 17520, 720);
        int maxSessions = clamp(body.max_sessions(), 1, 100, 1);
        CardsCsvService.ImportReport report;
        try {
            report = csvService.importCsv(
                body.csv(), Duration.ofHours(duration), maxSessions, body.dry_run());
        } catch (IllegalArgumentException ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid CSV data");
        } catch (SidecarException ex) {
            throw ex;
        } catch (RuntimeException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to import cards");
        }
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("imported", report.imported());
        response.put("dry_run", body.dry_run());
        response.put("report", report);
        return CompatResponse.ok(response);
    }

    private void requireWrite(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "cards.write");
    }

    private static int clamp(Integer value, int min, int max, int fallback) {
        if (value == null || value < min) {
            return fallback;
        }
        return Math.min(value, max);
    }

    private static String truncateNote(String note) {
        if (note == null) {
            return "";
        }
        return note.length() > 200 ? note.substring(0, 200) : note;
    }

    public record GenerateRequest(Integer duration_hours, Integer max_sessions, String note) {
    }

    public record BatchGenerateRequest(Integer count, Integer duration_hours, Integer max_sessions, String note) {
    }

    public record UpdateRequest(String code, String action, int extend_hours) {
    }

    public record UpdateDetailsRequest(String code, String note, Integer max_sessions) {
    }

    public record BulkRequest(List<String> codes, String action, int extend_hours, boolean dry_run) {
    }

    public record ImportRequest(String csv, Integer duration, Integer max_sessions, boolean dry_run) {
    }
}
