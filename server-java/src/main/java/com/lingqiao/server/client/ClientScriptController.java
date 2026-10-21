package com.lingqiao.server.client;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.scripts.FileScriptRepository;
import com.lingqiao.server.scripts.ScriptModule;
import jakarta.servlet.http.HttpServletRequest;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client script download mirroring Go /api/v1/script:
 * HMAC over request path plus session validation, then active module delivery.
 */
@RestController
@RequestMapping("/java/api/v1/script")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-download-enabled", havingValue = "true")
public class ClientScriptController {
    private final FileScriptRepository repository;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;

    public ClientScriptController(
                                 @Qualifier("clientScriptRepository") FileScriptRepository repository,
                                 ClientSessionValidator sessions,
                                 HmacAuthService hmac) {
        this.repository = repository;
        this.sessions = sessions;
        this.hmac = hmac;
    }

    @GetMapping
    public ResponseEntity<?> download(
        HttpServletRequest request,
        @RequestParam(name = "format", required = false) String format
    ) {
        String goPath = request.getRequestURI().replaceFirst("^/java", "");
        hmac.verify(
            request.getHeader("X-Client-ID"),
            goPath,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        String token = request.getHeader("X-Session-Token");
        if (token == null || token.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "token is required");
        }
        String machineId = request.getHeader("X-Machine-ID");
        String cardCode = request.getHeader("X-Card-Code");
        if (machineId != null && !machineId.isBlank() && cardCode != null && !cardCode.isBlank()) {
            sessions.validateBound(token, machineId, cardCode);
        } else {
            sessions.validateActive(token);
        }

        ScriptModule module = repository.requireActive();
        boolean wantsJson = "json".equalsIgnoreCase(format)
            || (request.getHeader("Accept") != null
                && request.getHeader("Accept").contains(MediaType.APPLICATION_JSON_VALUE));

        var builder = ResponseEntity.ok()
            .header("X-Script-ID", module.id())
            .header("X-Script-Version", module.version())
            .header("X-Script-SHA256", module.sha256())
            .header("X-Script-Hash", module.sha256())
            .header("X-Script-Size", String.valueOf(module.size()));

        if (wantsJson) {
            Map<String, Object> body = new LinkedHashMap<>();
            body.put("id", module.id());
            body.put("version", module.version());
            body.put("sha256", module.sha256());
            body.put("size", module.size());
            body.put("content", module.content());
            return builder.contentType(MediaType.APPLICATION_JSON).body(CompatResponse.ok(body));
        }
        return builder
            .contentType(MediaType.TEXT_PLAIN)
            .body(module.content().getBytes(StandardCharsets.UTF_8));
    }
}
