package com.lingqiao.server.payloads;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestPart;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

@RestController
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "payloads-write-enabled", havingValue = "true")
public class PayloadsWriteController {
    static final int LEGACY_BODY_LIMIT = 100 << 20;

    private final PayloadsWriteService service;
    private final AdminAuthService adminAuthService;
    private final PayloadUploadKey uploadKey;
    private final ObjectMapper mapper;

    public PayloadsWriteController(
        PayloadsWriteService service,
        AdminAuthService adminAuthService,
        PayloadUploadKey uploadKey,
        ObjectMapper mapper
    ) {
        this.service = service;
        this.adminAuthService = adminAuthService;
        this.uploadKey = uploadKey;
        this.mapper = mapper;
    }

    @PostMapping(value = "/payloads/upload-dll", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public Map<String, Object> uploadDll(
        HttpServletRequest request,
        @RequestPart(value = "file", required = false) MultipartFile file,
        @RequestParam(value = "payload_id", required = false) String payloadId,
        @RequestParam(value = "activate", required = false) String activate
    ) {
        requireWrite(request);
        if (file == null) {
            throw badRequest("file is required");
        }
        String filename = file.getOriginalFilename() == null ? "" : file.getOriginalFilename();
        if (!filename.toLowerCase(java.util.Locale.ROOT).endsWith(".dll")) {
            throw badRequest("only .dll payloads are supported");
        }

        byte[] dll;
        try {
            dll = file.getBytes();
        } catch (IOException ex) {
            throw badRequest("failed to read file");
        }
        PayloadPacker.PackedPayload packed =
            service.upload(payloadId, dll, PayloadPacker.DEFAULT_CHUNK_SIZE);
        String id = packed.document().getPayloadId();
        if ("true".equals(activate) || "1".equals(activate)) {
            try {
                service.activate(id);
            } catch (RuntimeException ex) {
                throw badRequest("failed to activate payload");
            }
        }

        Map<String, Object> fields = new LinkedHashMap<>();
        fields.put("payload", service.get(id));
        return CompatResponse.ok(fields);
    }

    @PostMapping("/payloads/manage")
    public Map<String, Object> manage(HttpServletRequest request, @RequestBody ManageRequest body) {
        requireWrite(request);
        if (body == null) {
            throw badRequest("invalid request body");
        }
        String action = body.action() == null ? "" : body.action();
        switch (action) {
            case "activate" -> {
                try {
                    service.activate(body.payload_id());
                } catch (RuntimeException ex) {
                    throw badRequest("failed to activate payload");
                }
                Map<String, Object> fields = new LinkedHashMap<>();
                fields.put("payload", service.get(body.payload_id()));
                return CompatResponse.ok(fields);
            }
            case "delete" -> {
                try {
                    service.delete(body.payload_id());
                } catch (RuntimeException ex) {
                    throw badRequest("failed to delete payload");
                }
                return CompatResponse.ok((Map<String, ?>) null);
            }
            default -> throw badRequest("unknown action");
        }
    }

    @PostMapping("/payload/upload")
    public Map<String, Object> legacyUpload(HttpServletRequest request) {
        requireWrite(request);
        if (!uploadKey.matches(request.getHeader("X-Upload-Key"))) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid upload key");
        }

        LegacyUploadRequest body = readLegacyBody(request);
        if (isEmpty(body.payload_id()) || isEmpty(body.aes_key())
            || body.chunks() == null || body.chunks().isEmpty()) {
            throw badRequest("missing required fields");
        }
        try {
            PayloadPacker.validatePayloadId(body.payload_id());
        } catch (SidecarException ex) {
            throw badRequest("invalid payload_id");
        }
        try {
            PayloadCrypto.parseHex(body.aes_key(), 32, "aes_key");
            PayloadCrypto.parseHex(body.hmac_key(), 32, "hmac_key");
            PayloadCrypto.parseHex(body.iv(), 16, "iv");
        } catch (SidecarException ex) {
            throw badRequest("invalid key material");
        }
        try {
            PayloadCrypto.parseHex(body.exe_hash(), 32, "exe_hash");
        } catch (SidecarException ex) {
            throw badRequest("invalid exe_hash");
        }
        if (body.chunk_count() != body.chunks().size()) {
            throw badRequest("chunk_count does not match chunks length");
        }
        if (body.chunk_size() <= 0) {
            throw badRequest("chunk_size must be positive");
        }
        if (body.total_size() <= 0) {
            throw badRequest("total_size must be positive");
        }

        List<byte[]> chunks = new ArrayList<>(body.chunks().size());
        int actualTotal = 0;
        for (int i = 0; i < body.chunks().size(); i++) {
            byte[] chunk = decodeLegacyChunk(body.chunks().get(i), i);
            if (chunk.length == 0) {
                throw badRequest("chunk " + i + " is empty");
            }
            if (chunk.length > body.chunk_size()) {
                throw badRequest("chunk " + i + " exceeds chunk_size");
            }
            if (i < body.chunks().size() - 1 && chunk.length != body.chunk_size()) {
                throw badRequest("chunk " + i + " size does not match chunk_size");
            }
            chunks.add(chunk);
            actualTotal += chunk.length;
        }
        if (actualTotal != body.total_size()) {
            throw badRequest("total_size does not match decoded chunks");
        }

        PayloadDocument document = new PayloadDocument();
        document.setPayloadId(body.payload_id());
        document.setAesKey(body.aes_key());
        document.setHmacKey(body.hmac_key());
        document.setIv(body.iv());
        document.setExeHash(body.exe_hash());
        document.setChunkCount(chunks.size());
        document.setChunkSize(body.chunk_size());
        document.setTotalSize(actualTotal);
        document.setCreatedAt(Instant.now());
        service.importEncrypted(document, chunks);

        Map<String, Object> fields = new LinkedHashMap<>();
        fields.put("payload_id", body.payload_id());
        fields.put("chunk_count", chunks.size());
        return CompatResponse.ok(fields);
    }

    private LegacyUploadRequest readLegacyBody(HttpServletRequest request) {
        try {
            byte[] bytes = request.getInputStream().readNBytes(LEGACY_BODY_LIMIT + 1);
            if (bytes.length > LEGACY_BODY_LIMIT) {
                throw badRequest("invalid JSON");
            }
            return mapper.readValue(bytes, LegacyUploadRequest.class);
        } catch (SidecarException ex) {
            throw ex;
        } catch (IOException ex) {
            throw badRequest("invalid JSON");
        }
    }

    private static byte[] decodeLegacyChunk(String value, int index) {
        String normalized = value == null ? "" : value.replace("\r", "").replace("\n", "");
        try {
            if (normalized.length() % 4 != 0) {
                throw new IllegalArgumentException("missing base64 padding");
            }
            return Base64.getDecoder().decode(normalized);
        } catch (IllegalArgumentException ex) {
            throw badRequest("invalid chunk " + index);
        }
    }

    private void requireWrite(HttpServletRequest request) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "payloads.write");
    }

    private static boolean isEmpty(String value) {
        return value == null || value.isEmpty();
    }

    private static SidecarException badRequest(String message) {
        return new SidecarException(ErrorCode.BAD_REQUEST, message);
    }

    public record ManageRequest(String payload_id, String action) {
    }

    public record LegacyUploadRequest(
        String payload_id,
        String aes_key,
        String hmac_key,
        String iv,
        String exe_hash,
        int chunk_count,
        int chunk_size,
        int total_size,
        List<String> chunks
    ) {
    }
}
