package com.lingqiao.server.client;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import jakarta.servlet.http.HttpServletRequest;
import java.nio.file.Files;
import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client DLL download mirroring Go APIHandler.HandleDllDownload:
 * HMAC over request path, bound session validation, then AES-256-GCM transport
 * encryption keyed by the per-client DLL key.
 */
@RestController
@RequestMapping("/java/api/v1/dll")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-download-enabled", havingValue = "true")
public class ClientDllController {
    private final DllSourceResolver resolver;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;
    private final String configuredSecret;
    private final Path secretFile;

    public ClientDllController(DllSourceResolver resolver,
                               ClientSessionValidator sessions,
                               HmacAuthService hmac,
                               @Value("${lingqiao.auth.hmac-secret:}") String configuredSecret,
                               @Value("${lingqiao.auth.hmac-secret-file:./data/hmac_secret.key}") String secretFile) {
        this.resolver = resolver;
        this.sessions = sessions;
        this.hmac = hmac;
        this.configuredSecret = configuredSecret == null ? "" : configuredSecret.trim();
        this.secretFile = Path.of(secretFile);
    }

    @GetMapping
    public ResponseEntity<byte[]> download(HttpServletRequest request) {
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
        sessions.validateBound(token, request.getHeader("X-Machine-ID"), request.getHeader("X-Card-Code"));

        DllSourceResolver.DllSource source = resolver.resolve();
        byte[] key = DllTransportCipher.deriveKey(resolveClientSecret());
        byte[] encrypted = DllTransportCipher.encrypt(source.bytes(), key);

        ResponseEntity.BodyBuilder builder = ResponseEntity.ok()
            .contentType(MediaType.APPLICATION_OCTET_STREAM)
            .contentLength(encrypted.length)
            .header("X-DLL-Source", source.kind())
            .header("X-DLL-SHA256", source.sha256());
        if (!source.payloadId().isEmpty()) {
            builder = builder.header("X-DLL-Payload-ID", source.payloadId());
        }
        return builder.body(encrypted);
    }

    private String resolveClientSecret() {
        if (!configuredSecret.isEmpty()) {
            return configuredSecret;
        }
        try {
            if (Files.exists(secretFile)) {
                String secret = Files.readString(secretFile).trim();
                if (!secret.isEmpty()) return secret;
            }
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "client key unavailable");
        }
        throw new SidecarException(ErrorCode.UNAUTHORIZED, "client key unavailable");
    }
}
