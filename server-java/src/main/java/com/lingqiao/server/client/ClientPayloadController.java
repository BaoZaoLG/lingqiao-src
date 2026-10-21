package com.lingqiao.server.client;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.payloads.PayloadCrypto;
import com.lingqiao.server.payloads.PayloadDocument;
import com.lingqiao.server.payloads.PayloadStateReader;
import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client payload distribution endpoints mirroring Go
 * /api/v1/payload/{id}/info and /api/v1/payload/{id}/chunk/{index}.
 */
@RestController
@RequestMapping("/java/api/v1/payload")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-download-enabled", havingValue = "true")
public class ClientPayloadController {
    private final PayloadStateReader stateReader;
    private final HmacAuthService hmac;
    private final Path dataDir;

    public ClientPayloadController(PayloadStateReader stateReader, HmacAuthService hmac,
                                   @Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        this.stateReader = stateReader;
        this.hmac = hmac;
        this.dataDir = Path.of(dataDir).toAbsolutePath().normalize();
    }

    @GetMapping("/{payloadId}/info")
    public Map<String, Object> info(HttpServletRequest request, @PathVariable String payloadId) {
        verifyPathSignature(request);
        PayloadDocument document = requirePayload(payloadId);

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("payload_id", document.getPayloadId());
        body.put("chunk_count", document.getChunkCount());
        body.put("chunk_size", document.getChunkSize());
        body.put("total_size", document.getTotalSize());
        body.put("exe_hash", document.getExeHash());
        body.put("created_at", document.getCreatedAt());
        return CompatResponse.ok(body);
    }

    @GetMapping("/{payloadId}/chunk/{index}")
    public ResponseEntity<byte[]> chunk(HttpServletRequest request,
                                        @PathVariable String payloadId,
                                        @PathVariable int index) {
        verifyPathSignature(request);
        PayloadDocument document = requirePayload(payloadId);
        if (index < 0 || index >= document.getChunkCount()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "chunk index out of range");
        }

        Path chunkPath = chunkPath(payloadId, index);
        if (!Files.exists(chunkPath)) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "chunk file not found");
        }
        try {
            byte[] data = Files.readAllBytes(chunkPath);
            return ResponseEntity.ok()
                .contentType(MediaType.APPLICATION_OCTET_STREAM)
                .contentLength(data.length)
                .body(data);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "chunk file not found");
        }
    }

    private void verifyPathSignature(HttpServletRequest request) {
        String goPath = request.getRequestURI().replaceFirst("^/java", "");
        hmac.verify(
            request.getHeader("X-Client-ID"),
            goPath,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));
    }

    private PayloadDocument requirePayload(String payloadId) {
        com.lingqiao.server.payloads.PayloadPacker.validatePayloadId(payloadId);
        PayloadDocument document = stateReader.read().getPayloads().get(payloadId);
        if (document == null) {
            throw new SidecarException(ErrorCode.PAYLOAD_NOT_FOUND, "payload not found");
        }
        return document;
    }

    private Path chunkPath(String payloadId, int index) {
        Path base = dataDir.resolve("payloads").resolve(payloadId).toAbsolutePath().normalize();
        if (!base.startsWith(dataDir.resolve("payloads"))) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload path");
        }
        Path chunk = base.resolve(String.format("chunk_%04d.bin", index)).toAbsolutePath().normalize();
        if (!chunk.startsWith(base)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid chunk path");
        }
        return chunk;
    }

    static String hex(byte[] data) {
        return PayloadCrypto.hex(data);
    }
}
