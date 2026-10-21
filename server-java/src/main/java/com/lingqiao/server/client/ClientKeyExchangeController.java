package com.lingqiao.server.client;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.cards.CardCodes;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.payloads.PayloadCrypto;
import com.lingqiao.server.payloads.PayloadDocument;
import com.lingqiao.server.payloads.PayloadStateReader;
import jakarta.servlet.http.HttpServletRequest;
import java.security.SecureRandom;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client key exchange mirroring Go payloads.Handler.HandleKeyExchange.
 * Wraps payload key material with a KEK derived from card code + machine id.
 */
@RestController
@RequestMapping("/java/api/v1")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-download-enabled", havingValue = "true")
public class ClientKeyExchangeController {
    private final PayloadStateReader stateReader;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;
    private final ObjectMapper mapper = new ObjectMapper();
    private final SecureRandom random = new SecureRandom();

    public ClientKeyExchangeController(PayloadStateReader stateReader,
                                       ClientSessionValidator sessions,
                                       HmacAuthService hmac) {
        this.stateReader = stateReader;
        this.sessions = sessions;
        this.hmac = hmac;
    }

    @PostMapping(path = "/key-exchange", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> keyExchange(HttpServletRequest request, @RequestBody String rawBody) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        KeyExchangeRequest req = parse(rawBody);
        ClientSessionValidator.ValidatedSession session =
            sessions.validateBound(req.session_token(), req.machine_id(), "");

        PayloadDocument payload = stateReader.read().getPayloads().get(req.payload_id());
        if (payload == null) {
            throw new SidecarException(ErrorCode.PAYLOAD_NOT_FOUND, "payload not found");
        }

        byte[] kek = PayloadCrypto.deriveKek(
            CardCodes.normalize(session.cardCode()), req.machine_id(), req.payload_id());

        byte[] aesKey = PayloadCrypto.parseHex(payload.getAesKey(), 32, "aes_key");
        byte[] hmacKey = PayloadCrypto.parseHex(payload.getHmacKey(), 32, "hmac_key");
        byte[] iv = PayloadCrypto.parseHex(payload.getIv(), 16, "iv");

        byte[] combined = PayloadCrypto.combineKeyMaterial(aesKey, hmacKey, iv);
        byte[] encryptionIv = new byte[16];
        random.nextBytes(encryptionIv);
        byte[] wrapped = PayloadCrypto.wrapKeyMaterial(kek, combined, encryptionIv);
        byte[] tag = PayloadCrypto.macKeyMaterial(kek, wrapped);

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("encrypted_key", PayloadCrypto.hex(wrapped));
        body.put("encrypted_iv", PayloadCrypto.hex(encryptionIv));
        body.put("encrypted_hmac", PayloadCrypto.hex(tag));
        return CompatResponse.ok(body);
    }

    private KeyExchangeRequest parse(String rawBody) {
        try {
            KeyExchangeRequest parsed = mapper.readValue(rawBody, KeyExchangeRequest.class);
            if (parsed == null) {
                throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid JSON");
            }
            return parsed;
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid JSON");
        }
    }

    public record KeyExchangeRequest(String session_token, String machine_id, String payload_id) {
    }
}
