package com.lingqiao.server.payloads;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.audit.AuditEvent;
import com.lingqiao.server.audit.AuditWriteService;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Java write path for DLL payloads. Mirrors Go payloads store/packer:
 * atomic chunk directory replacement plus payloads.json metadata rollback.
 */
public class PayloadsWriteService {
    private final Path dataDir;
    private final PayloadPacker packer;
    private final ObjectMapper mapper;
    private final AuditWriteService audit;

    public PayloadsWriteService(Path dataDir) {
        this(dataDir, null);
    }

    public PayloadsWriteService(Path dataDir, AuditWriteService audit) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.packer = new PayloadPacker();
        this.audit = audit;
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .enable(SerializationFeature.INDENT_OUTPUT);
    }

    public synchronized PayloadPacker.PackedPayload upload(String payloadId, byte[] dll, int chunkSize) {
        String id = payloadId == null || payloadId.isEmpty() ? packer.generatePayloadId() : payloadId;
        PayloadState state = readState();
        if (state.getPayloads().containsKey(id)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "payload_id already exists");
        }
        PayloadPacker.PackedPayload packed = packer.pack(id, dll, chunkSize);
        persist(state, packed.document(), packed.chunks());
        recordAudit("payload_uploaded", packed.document());
        return packed;
    }

    public synchronized Payload importEncrypted(PayloadDocument document, List<byte[]> chunks) {
        PayloadPacker.validatePayloadId(document == null ? null : document.getPayloadId());
        PayloadState state = readState();
        persist(state, document, chunks);
        recordAudit("payload_uploaded", document);
        return document.toSummary(document.getPayloadId().equals(state.getActiveId()));
    }

    public synchronized void activate(String payloadId) {
        PayloadPacker.validatePayloadId(payloadId);
        PayloadState state = readState();
        if (!state.getPayloads().containsKey(payloadId)) {
            throw new SidecarException(ErrorCode.PAYLOAD_NOT_FOUND, "payload not found");
        }
        String previousActive = state.getActiveId();
        state.setActiveId(payloadId);
        try {
            writeState(state);
        } catch (RuntimeException ex) {
            state.setActiveId(previousActive);
            throw ex;
        }
        recordAudit("payload_activated", state.getPayloads().get(payloadId));
    }

    public synchronized void delete(String payloadId) {
        PayloadPacker.validatePayloadId(payloadId);
        PayloadState state = readState();
        if (payloadId.equals(state.getActiveId())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "cannot delete active payload");
        }
        Map<String, PayloadDocument> payloads = new LinkedHashMap<>(state.getPayloads());
        PayloadDocument removed = payloads.remove(payloadId);
        if (removed == null) {
            throw new SidecarException(ErrorCode.PAYLOAD_NOT_FOUND, "payload not found");
        }
        state.setPayloads(payloads);
        writeState(state);
        deleteQuietly(payloadDir(payloadId));
        recordAudit("payload_deleted", removed);
    }

    public synchronized List<Payload> list() {
        PayloadState state = readState();
        List<Payload> payloads = new ArrayList<>();
        state.getPayloads().forEach((id, doc) ->
            payloads.add(doc.toSummary(id.equals(state.getActiveId()))));
        payloads.sort(Comparator.comparing(Payload::createdAt).reversed());
        return payloads;
    }

    public synchronized Payload get(String payloadId) {
        PayloadState state = readState();
        PayloadDocument document = state.getPayloads().get(payloadId);
        return document == null ? null : document.toSummary(payloadId.equals(state.getActiveId()));
    }

    private void persist(PayloadState state, PayloadDocument document, List<byte[]> chunks) {
        String id = document.getPayloadId();
        writeChunks(id, chunks);
        PayloadState previous = copyOf(state);
        Map<String, PayloadDocument> payloads = new LinkedHashMap<>(state.getPayloads());
        payloads.put(id, document);
        state.setPayloads(payloads);
        try {
            writeState(state);
        } catch (RuntimeException ex) {
            deleteQuietly(payloadDir(id));
            writeState(previous);
            throw ex;
        }
    }

    private void writeChunks(String id, List<byte[]> chunks) {
        Path finalDir = payloadDir(id);
        Path tmpDir = finalDir.resolveSibling(id + ".tmp");
        try {
            deleteQuietly(tmpDir);
            Files.createDirectories(tmpDir);
            for (int i = 0; i < chunks.size(); i++) {
                Files.write(tmpDir.resolve(String.format("chunk_%04d.bin", i)), chunks.get(i));
            }
            deleteQuietly(finalDir);
            Files.move(tmpDir, finalDir, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            deleteQuietly(tmpDir);
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist payload chunks");
        }
    }

    private PayloadState readState() {
        Path path = statePath();
        if (!Files.exists(path)) {
            return new PayloadState();
        }
        try {
            PayloadState state = mapper.readValue(path.toFile(), PayloadState.class);
            return state == null ? new PayloadState() : state;
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read payload state");
        }
    }

    private void writeState(PayloadState state) {
        Path target = statePath();
        try {
            Files.createDirectories(target.getParent());
            Path tmp = Files.createTempFile(target.getParent(), "payloads", ".tmp");
            Files.write(tmp, mapper.writeValueAsBytes(state));
            Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist payload state");
        }
    }

    private static PayloadState copyOf(PayloadState state) {
        PayloadState copy = new PayloadState();
        copy.setActiveId(state.getActiveId());
        copy.setPayloads(new LinkedHashMap<>(state.getPayloads()));
        return copy;
    }

    private void recordAudit(String action, PayloadDocument document) {
        if (audit == null || document == null) {
            return;
        }
        String detail = "id=%s sha256=%s chunks=%d size=%d".formatted(
            document.getPayloadId(), document.getExeHash(),
            document.getChunkCount(), document.getTotalSize());
        try {
            audit.append(new AuditEvent(
                (Instant) null, action, null, null, null, null, null, null, null, null,
                null, null, null, null, detail));
        } catch (RuntimeException ignored) {
            // Go treats payload audit as best effort and keeps the completed state change.
        }
    }

    private Path payloadDir(String payloadId) {
        Path path = dataDir.resolve("payloads").resolve(payloadId).toAbsolutePath().normalize();
        if (!path.startsWith(dataDir.resolve("payloads"))) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload path");
        }
        return path;
    }

    private Path statePath() {
        Path path = dataDir.resolve("payloads.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload state path");
        }
        return path;
    }

    private static void deleteQuietly(Path path) {
        if (!Files.exists(path)) {
            return;
        }
        try (var stream = Files.walk(path)) {
            stream.sorted(Comparator.reverseOrder()).forEach(p -> {
                try {
                    Files.deleteIfExists(p);
                } catch (IOException ignored) {
                    // best effort cleanup
                }
            });
        } catch (IOException ignored) {
            // best effort cleanup
        }
    }
}
