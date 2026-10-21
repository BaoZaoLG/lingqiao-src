package com.lingqiao.server.cards;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/cards")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-enabled", havingValue = "true")
public class CardsController {
    private final CardsService service;
    private final AdminAuthService adminAuthService;

    public CardsController(CardsService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(
        HttpServletRequest request,
        @RequestParam(name = "search", required = false) String search,
        @RequestParam(required = false) String status,
        @RequestParam(name = "machine", required = false) String machine,
        @RequestParam(name = "agent_id", required = false) String agentId,
        @RequestParam(name = "bound", required = false) String bound,
        @RequestParam(name = "expires_before", required = false) String expiresBefore,
        @RequestParam(name = "expires_after", required = false) String expiresAfter,
        @RequestParam(name = "max_sessions", required = false) String maxSessions
    ) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "cards.read");

        List<Card> cards = service.list(new CardListFilter(
            search, status, machine, agentId,
            parseBound(bound),
            parseInstant(expiresBefore),
            parseInstant(expiresAfter),
            parseMaxSessions(maxSessions)));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("cards", cards);
        return CompatResponse.ok(body);
    }

    /** Go accepts only 1/true and 0/false; anything else leaves the filter off. */
    static Boolean parseBound(String value) {
        if (value == null) {
            return null;
        }
        return switch (value) {
            case "1", "true" -> Boolean.TRUE;
            case "0", "false" -> Boolean.FALSE;
            default -> null;
        };
    }

    /** Go silently ignores unparsable RFC3339 timestamps. */
    static Instant parseInstant(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(value);
        } catch (DateTimeParseException ex) {
            return null;
        }
    }

    /** Go silently ignores a non-numeric max_sessions. */
    static Integer parseMaxSessions(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Integer.valueOf(value);
        } catch (NumberFormatException ex) {
            return null;
        }
    }

    @GetMapping("/detail")
    public Map<String, Object> detail(HttpServletRequest request, @RequestParam("code") String code) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "cards.read");
        if (code == null || code.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "code is required");
        }
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("card", service.get(code));
        return CompatResponse.ok(body);
    }

    @GetMapping("/export")
    public void export(HttpServletRequest request, HttpServletResponse response) throws IOException {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.read");
        List<Card> cards = service.list(new CardListFilter(
            null, null, null, null, null, null, null, null));
        response.setContentType("text/csv; charset=utf-8");
        response.setHeader("Content-Disposition", "attachment; filename=cards_export.csv");
        response.getOutputStream().write(CardsCsvService.exportCsv(cards));
    }
}
