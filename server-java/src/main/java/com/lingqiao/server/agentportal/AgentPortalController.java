package com.lingqiao.server.agentportal;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Agent portal endpoints mirroring Go agents.AgentHandler routes served on the
 * dedicated agent port.
 */
@RestController
@RequestMapping("/java/agent/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agent-portal-enabled", havingValue = "true")
public class AgentPortalController {
    private final AgentPortalAuthService auth;
    private final AgentRegistrationService registration;
    private final AgentPortalCardService cards;

    public AgentPortalController(
        AgentPortalAuthService auth,
        AgentRegistrationService registration,
        AgentPortalCardService cards
    ) {
        this.auth = auth;
        this.registration = registration;
        this.cards = cards;
    }

    @PostMapping(path = "/register", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<Map<String, Object>> register(HttpServletRequest request,
                                                       @RequestBody RegisterRequest body) {
        if (!auth.allowRegistration(clientIp(request))) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "注册过于频繁，请稍后再试");
        }
        AgentRegistrationService.RegisteredAgent agent =
            registration.register(body.username(), body.password(), body.invite_code());

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("agent_id", agent.agentId());
        payload.put("username", agent.username());
        return ResponseEntity.ok()
            .header(HttpHeaders.CACHE_CONTROL, "no-store")
            .body(CompatResponse.ok(payload));
    }

    @PostMapping(path = "/login", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<Map<String, Object>> login(HttpServletRequest request,
                                                     @RequestBody LoginRequest body) {
        AgentPortalAuthService.LoginResult result =
            auth.login(body.username(), body.password(), clientIp(request));

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("token", result.token());
        payload.put("agent_id", result.agentId());
        payload.put("username", result.username());
        return ResponseEntity.ok()
            .header(HttpHeaders.SET_COOKIE, sessionCookie(result.token(), result.maxAgeSeconds()))
            .header(HttpHeaders.CACHE_CONTROL, "no-store")
            .body(CompatResponse.ok(payload));
    }

    @PostMapping("/logout")
    public ResponseEntity<Map<String, Object>> logout(HttpServletRequest request) {
        auth.logout(RequestAuthSupport.extractToken(request, "agent_token"));
        return ResponseEntity.ok()
            .header(HttpHeaders.SET_COOKIE, sessionCookie("", 0))
            .header(HttpHeaders.CACHE_CONTROL, "no-store")
            .body(CompatResponse.ok((Map<String, ?>) null));
    }

    @GetMapping("/dashboard")
    public Map<String, Object> dashboard(HttpServletRequest request) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        AgentPortalAuthService.AgentStats stats = auth.stats(agentId);

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("total_cards", stats.totalCards());
        payload.put("active_cards", stats.activeCards());
        payload.put("expired_cards", stats.expiredCards());
        return CompatResponse.ok(payload);
    }

    @GetMapping("/info")
    public Map<String, Object> info(HttpServletRequest request) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        AgentPortalAuthService.AgentProfile profile = auth.profile(agentId);
        Map<String, Object> agent = new LinkedHashMap<>();
        agent.put("id", profile.id());
        agent.put("username", profile.username());
        agent.put("prefix", profile.prefix());
        agent.put("created_at", profile.createdAt());
        return CompatResponse.ok(Map.of("agent", agent));
    }

    @GetMapping("/cards")
    public Map<String, Object> listCards(HttpServletRequest request) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        return CompatResponse.ok(Map.of("cards", cards.list(agentId)));
    }

    @PostMapping(path = "/card/generate", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> generateCard(
        HttpServletRequest request,
        @RequestBody GenerateCardRequest body
    ) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        int duration = clamp(body.duration_hours(), 1, 8760, 720);
        int maxSessions = clamp(body.max_sessions(), 1, 5, 1);
        String note = body.note() == null ? "" : body.note();
        if (note.length() > 200) {
            note = note.substring(0, 200);
        }
        return CompatResponse.ok(Map.of(
            "card", cards.generate(agentId, duration, note, maxSessions)
        ));
    }

    @PostMapping(path = "/card/batch-generate", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> batchGenerateCards(
        HttpServletRequest request,
        @RequestBody BatchGenerateCardRequest body
    ) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        int count = clamp(body.count(), 1, 50, 1);
        int duration = clamp(body.duration_hours(), 1, 8760, 720);
        int maxSessions = clamp(body.max_sessions(), 1, 5, 1);
        List<AgentPortalCardService.AgentCard> generated = cards.batchGenerate(
            agentId, count, duration, body.note() == null ? "" : body.note(), maxSessions);
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("cards", generated);
        payload.put("count", generated.size());
        return CompatResponse.ok(payload);
    }

    @PostMapping(path = "/password", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> password(HttpServletRequest request, @RequestBody PasswordRequest body) {
        String agentId = auth.requireSession(RequestAuthSupport.extractToken(request, "agent_token"));
        auth.changePasswordFromIp(
            agentId, body.old_password(), body.new_password(), clientIp(request));
        return CompatResponse.ok(Map.of("message", "密码修改成功"));
    }

    private static String sessionCookie(String token, int maxAgeSeconds) {
        return "agent_token=" + token
            + "; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=" + maxAgeSeconds;
    }

    private static String clientIp(HttpServletRequest request) {
        String forwarded = request.getHeader("X-Forwarded-For");
        if (forwarded != null && !forwarded.isBlank()) {
            int comma = forwarded.indexOf(',');
            return comma > 0 ? forwarded.substring(0, comma).trim() : forwarded.trim();
        }
        return request.getRemoteAddr() == null ? "" : request.getRemoteAddr();
    }

    private static int clamp(Integer value, int min, int max, int fallback) {
        if (value == null || value == 0) {
            return fallback;
        }
        if (value < min) {
            return min;
        }
        return Math.min(value, max);
    }

    public record LoginRequest(String username, String password) {
    }

    public record RegisterRequest(String invite_code, String username, String password) {
    }

    public record PasswordRequest(String old_password, String new_password) {
    }

    public record GenerateCardRequest(Integer duration_hours, Integer max_sessions, String note) {
    }

    public record BatchGenerateCardRequest(
        Integer count,
        Integer duration_hours,
        Integer max_sessions,
        String note
    ) {
    }
}
