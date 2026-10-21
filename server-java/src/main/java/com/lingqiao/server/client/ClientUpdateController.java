package com.lingqiao.server.client;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.releases.ManifestSeedLoader;
import com.lingqiao.server.releases.ManifestSigner;
import com.lingqiao.server.releases.ReleaseEventRecorder;
import com.lingqiao.server.releases.ReleaseSelector;
import com.lingqiao.server.releases.ReleaseUpdateService;
import com.lingqiao.server.releases.SignedUpdateManifest;
import com.lingqiao.server.releases.UpdateManifest;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client update check mirroring Go releases.Handler.HandleUpdateCheck:
 * HMAC over body, bound session validation, rollout selection, Ed25519 signed
 * manifest plus derived-key manifest HMAC, and an "offered" event record.
 */
@RestController
@RequestMapping("/java/api/v1/update")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-update-enabled", havingValue = "true")
public class ClientUpdateController {
    private final ReleaseUpdateService updates;
    private final ReleaseEventRecorder events;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;
    private final ManifestSeedLoader seedLoader;
    private final ObjectMapper mapper =
        new ObjectMapper().enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    public ClientUpdateController(ReleaseUpdateService updates,
                                  ReleaseEventRecorder events,
                                  ClientSessionValidator sessions,
                                  HmacAuthService hmac,
                                  ManifestSeedLoader seedLoader) {
        this.updates = updates;
        this.events = events;
        this.sessions = sessions;
        this.hmac = hmac;
        this.seedLoader = seedLoader;
    }

    @PostMapping("/check")
    public Map<String, Object> check(
        HttpServletRequest request,
        @RequestBody(required = false) String rawBody
    ) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        UpdateCheckRequest req = parse(rawBody);
        String channel = isEmpty(req.channel()) ? "stable" : req.channel();
        String cardCode = isEmpty(req.card_code()) ? req.card() : req.card_code();
        requireBound(req.session_token(), req.machine_id(), cardCode);

        Optional<ReleaseUpdateService.SelectedRelease> selected = updates.select(
            new ReleaseSelector.ClientContext(
                req.client_version(), channel, cardCode, req.machine_id(), req.agent_id()));
        if (selected.isEmpty()) {
            return CompatResponse.ok(Map.of("update_available", false));
        }

        ReleaseUpdateService.SelectedRelease selection = selected.get();
        UpdateManifest manifest = updates.toManifest(selection);
        ManifestSigner signer = new ManifestSigner(seedLoader.load());
        SignedUpdateManifest signed = signer.sign(manifest);

        byte[] manifestPayload = ManifestSigner.canonical(manifest);
        String manifestHmac;
        try {
            manifestHmac = hmac.signDerived(
                request.getHeader("X-Client-ID"),
                new String(manifestPayload, java.nio.charset.StandardCharsets.UTF_8));
        } catch (SidecarException ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "client key unavailable");
        }

        events.recordQuietly(new ReleaseEventRecorder.ReleaseEvent(
            selection.release().id(), selection.release().version(), req.machine_id(),
            cardCode, req.agent_id(), ReleaseEventRecorder.OFFERED, "", "", Instant.now()));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("update_available", true);
        body.put("manifest", manifest);
        body.put("manifest_payload", Base64.getEncoder().encodeToString(manifestPayload));
        body.put("manifest_hmac", manifestHmac);
        body.put("signature", signed.signature());
        body.put("public_key", signer.publicKeyHex());
        return CompatResponse.ok(body);
    }

    @PostMapping("/events")
    public Map<String, Object> event(
        HttpServletRequest request,
        @RequestBody(required = false) String rawBody
    ) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        UpdateEventRequest req = parseEvent(rawBody);
        if (isEmpty(req.release_id()) || isEmpty(req.type())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "release_id and type are required");
        }
        String token = firstNonEmpty(request.getHeader("X-Session-Token"), req.session_token());
        String machine = firstNonEmpty(request.getHeader("X-Machine-ID"), req.machine_id());
        String card = firstNonEmpty(request.getHeader("X-Card-Code"), req.card_code());
        ClientSessionValidator.ValidatedSession session;
        try {
            session = sessions.validateBound(token, machine, card);
        } catch (SidecarException ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid or expired session");
        }
        events.record(new ReleaseEventRecorder.ReleaseEvent(
            req.release_id(), req.version(), session.machineId(), session.cardCode(), req.agent_id(),
            req.type(), req.error_code(), capDetail(req.detail()), Instant.now()));
        return CompatResponse.ok((Map<String, ?>) null);
    }

    private UpdateCheckRequest parse(String rawBody) {
        try {
            UpdateCheckRequest parsed = mapper.readValue(rawBody, UpdateCheckRequest.class);
            if (parsed == null) {
                throw new SidecarException(ErrorCode.INVALID_JSON, "请求体不是有效 JSON");
            }
            return parsed;
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "请求体不是有效 JSON");
        }
    }

    private UpdateEventRequest parseEvent(String rawBody) {
        try {
            UpdateEventRequest parsed = mapper.readValue(rawBody, UpdateEventRequest.class);
            if (parsed == null) {
                throw new SidecarException(ErrorCode.INVALID_JSON, "invalid JSON");
            }
            return parsed;
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "invalid JSON");
        }
    }

    private ClientSessionValidator.ValidatedSession requireBound(
        String token,
        String machine,
        String card
    ) {
        try {
            return sessions.validateBound(token, machine, card);
        } catch (SidecarException ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, ex.getMessage());
        }
    }

    private static boolean isEmpty(String value) {
        return value == null || value.isEmpty();
    }

    private static String firstNonEmpty(String preferred, String fallback) {
        return isEmpty(preferred) ? fallback : preferred;
    }

    private static String capDetail(String detail) {
        if (detail == null || detail.length() <= 500) {
            return detail;
        }
        return detail.substring(0, 500);
    }

    /**
     * Unknown fields are ignored, matching Go's encoding/json. The client sends
     * client_id and the Go handler drops it; without this, FAIL_ON_UNKNOWN_PROPERTIES
     * rejected the same valid body and parse() reported it as INVALID_JSON.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public record UpdateCheckRequest(
        String session_token,
        String machine_id,
        String card,
        String card_code,
        String agent_id,
        String client_version,
        String channel
    ) {
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public record UpdateEventRequest(
        String session_token,
        String release_id,
        String version,
        String machine_id,
        String card_code,
        String agent_id,
        String type,
        String error_code,
        String detail
    ) {
    }
}
