package com.lingqiao.server.client;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * HMAC-authenticated client lifecycle endpoints mirroring Go
 * /api/v1/activate, /api/v1/heartbeat and /api/v1/deactivate.
 */
@RestController
@RequestMapping("/java/api/v1")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-write-enabled", havingValue = "true")
public class ClientSessionController {
    private final ClientSessionService service;
    private final HmacAuthService hmac;
    private final ObjectMapper mapper = new ObjectMapper();

    public ClientSessionController(ClientSessionService service, HmacAuthService hmac) {
        this.service = service;
        this.hmac = hmac;
    }

    @PostMapping(path = "/activate", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> activate(HttpServletRequest request, @RequestBody String rawBody) {
        ClientRequest req = verifyAndParse(request, rawBody);
        ClientSessionService.ActivationResult result = service.activate(
            new ClientSessionService.ActivationInput(
                req.card(), req.machine_id(), req.fingerprint(), req.client_version(), clientIp(request)));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("session_token", result.sessionToken());
        body.put("expires_at", result.sessionExpiresAt().getEpochSecond());
        body.put("card_expires_at", result.cardExpiresAt().getEpochSecond());
        body.put("message", "激活成功");
        return CompatResponse.ok(body);
    }

    @PostMapping(path = "/heartbeat", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> heartbeat(HttpServletRequest request, @RequestBody String rawBody) {
        ClientRequest req = verifyAndParse(request, rawBody);
        ClientSessionService.HeartbeatResult result = service.heartbeat(
            new ClientSessionService.HeartbeatInput(
                req.session_token(), req.machine_id(), req.client_version()));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("expires_at", result.sessionExpiresAt().getEpochSecond());
        body.put("card_expires_at", result.cardExpiresAt().getEpochSecond());
        body.put("message", "心跳正常");
        return CompatResponse.ok(body);
    }

    @PostMapping(path = "/deactivate", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> deactivate(HttpServletRequest request, @RequestBody String rawBody) {
        ClientRequest req = verifyAndParse(request, rawBody);
        service.deactivate(req.session_token(), req.machine_id());
        return CompatResponse.ok(Map.of("message", "已退出"));
    }

    private ClientRequest verifyAndParse(HttpServletRequest request, String rawBody) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));
        try {
            ClientRequest parsed = mapper.readValue(rawBody, ClientRequest.class);
            return parsed == null ? new ClientRequest(null, null, null, null, null) : parsed;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "invalid json");
        }
    }

    private static String clientIp(HttpServletRequest request) {
        String forwarded = request.getHeader("X-Forwarded-For");
        if (forwarded != null && !forwarded.isBlank()) {
            int comma = forwarded.indexOf(',');
            return comma > 0 ? forwarded.substring(0, comma).trim() : forwarded.trim();
        }
        return request.getRemoteAddr() == null ? "" : request.getRemoteAddr();
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public record ClientRequest(
        String card,
        String machine_id,
        String fingerprint,
        String client_version,
        String session_token
    ) {
    }
}
