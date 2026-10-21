package com.lingqiao.server.scripts;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.audit.AuditEvent;
import com.lingqiao.server.audit.AuditWriteService;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.security.MessageDigest;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.regex.Pattern;

/**
 * Java write path for the script repository. Mirrors Go scripts.Repository
 * create/publish/activate/delete semantics on the same on-disk layout.
 */
public class ScriptsWriteService {
    static final int MAX_SCRIPT_BYTES = 2 << 20;
    static final String STATUS_DRAFT = "draft";
    static final String STATUS_ACTIVE = "active";
    static final String STATUS_ARCHIVED = "archived";

    private static final Pattern UNSAFE = Pattern.compile("[^A-Za-z0-9._-]+");
    private static final DateTimeFormatter VERSION_FORMAT =
        DateTimeFormatter.ofPattern("yyyyMMdd.HHmmss").withZone(ZoneOffset.UTC);
    private static final DateTimeFormatter ID_FORMAT =
        DateTimeFormatter.ofPattern("yyyyMMdd'T'HHmmssSSS'Z'").withZone(ZoneOffset.UTC);

    private final Path scriptDir;
    private final ObjectMapper mapper;
    private final AuditWriteService audit;

    public ScriptsWriteService(Path dataDir) {
        this(dataDir, null);
    }

    public ScriptsWriteService(Path dataDir, AuditWriteService audit) {
        this.scriptDir = dataDir.toAbsolutePath().normalize().resolve("scripts").normalize();
        this.audit = audit;
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .enable(SerializationFeature.INDENT_OUTPUT);
    }

    public ScriptModule create(String version, String content, String note) {
        String normalizedContent = requireContent(content);
        ScriptIndex index = readIndex();
        ScriptModuleDocument module = makeModule(uniqueVersion(index, version), normalizedContent, note, STATUS_DRAFT);
        writeModule(module);
        List<ScriptModuleDocument> scripts = new ArrayList<>();
        scripts.add(module);
        scripts.addAll(index.getScripts());
        index.setScripts(scripts);
        writeIndex(index);
        ScriptModule result = module.toModule();
        recordAudit("script_created", result);
        return result;
    }

    public ScriptModule publish(String version, String content, String note) {
        String normalizedContent = requireContent(content);
        ScriptIndex index = readIndex();
        Instant now = Instant.now();
        ScriptModuleDocument module = makeModule(uniqueVersion(index, version), normalizedContent, note, STATUS_ACTIVE);
        module.setActivatedAt(module.getUpdatedAt());

        archivePreviousActive(index, now);
        writeModule(module);

        index.setActiveId(module.getId());
        List<ScriptModuleDocument> scripts = new ArrayList<>();
        scripts.add(module);
        scripts.addAll(index.getScripts());
        index.setScripts(scripts);
        writeIndex(index);
        ScriptModule result = module.toModule();
        recordAudit("script_published", result);
        return result;
    }

    public ScriptModule activate(String id) {
        ScriptIndex index = readIndex();
        ScriptModuleDocument target = index.getScripts().stream()
            .filter(s -> s.getId().equals(id))
            .findFirst()
            .orElseThrow(() -> new SidecarException(ErrorCode.NOT_FOUND, "script not found"));
        if (STATUS_ACTIVE.equals(target.getStatus())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "already active");
        }

        Instant now = Instant.now();
        archivePreviousActive(index, now);

        target.setStatus(STATUS_ACTIVE);
        target.setActivatedAt(now);
        target.setUpdatedAt(now);
        writeModule(target);

        index.setActiveId(target.getId());
        writeIndex(index);
        ScriptModule result = target.toModule();
        recordAudit("script_activated", result);
        return result;
    }

    public void delete(String id) {
        ScriptIndex index = readIndex();
        if (index.getActiveId().equals(id)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "cannot delete active script");
        }
        List<ScriptModuleDocument> remaining = new ArrayList<>();
        boolean found = false;
        for (ScriptModuleDocument doc : index.getScripts()) {
            if (doc.getId().equals(id)) {
                found = true;
                continue;
            }
            remaining.add(doc);
        }
        if (!found) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "script not found");
        }
        index.setScripts(remaining);
        writeIndex(index);
        try {
            Files.deleteIfExists(modulePath(id));
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to delete script module");
        }
        recordAudit("script_deleted", new ScriptModule(
            id, "", "", "", 0, "", "", Instant.EPOCH, Instant.EPOCH, null));
    }

    private void archivePreviousActive(ScriptIndex index, Instant now) {
        for (ScriptModuleDocument doc : index.getScripts()) {
            if (STATUS_ACTIVE.equals(doc.getStatus())) {
                doc.setStatus(STATUS_ARCHIVED);
                doc.setUpdatedAt(now);
                writeModule(doc);
            }
        }
    }

    private ScriptModuleDocument makeModule(String version, String content, String note, String status) {
        Instant now = Instant.now();
        ScriptModuleDocument doc = new ScriptModuleDocument();
        doc.setVersion(version);
        doc.setId(version + "-" + ID_FORMAT.format(now));
        doc.setContent(content);
        doc.setSha256(sha256Hex(content));
        doc.setSize(content.getBytes(StandardCharsets.UTF_8).length);
        doc.setNote(note == null ? "" : note.trim());
        doc.setStatus(status);
        doc.setCreatedAt(now);
        doc.setUpdatedAt(now);
        return doc;
    }

    private String uniqueVersion(ScriptIndex index, String requested) {
        String version = sanitizeVersion(requested);
        if (version.isEmpty()) {
            version = VERSION_FORMAT.format(Instant.now());
        }
        while (versionExists(index, version)) {
            version = version + "-" + DateTimeFormatter.ofPattern("HHmmss")
                .withZone(ZoneOffset.UTC).format(Instant.now());
        }
        return version;
    }

    private static boolean versionExists(ScriptIndex index, String version) {
        return index.getScripts().stream().anyMatch(s -> version.equals(s.getVersion()));
    }

    static String sanitizeVersion(String version) {
        if (version == null) {
            return "";
        }
        String replaced = UNSAFE.matcher(version.trim()).replaceAll("-");
        int start = 0;
        int end = replaced.length();
        while (start < end && isTrimmable(replaced.charAt(start))) {
            start++;
        }
        while (end > start && isTrimmable(replaced.charAt(end - 1))) {
            end--;
        }
        return replaced.substring(start, end);
    }

    private static boolean isTrimmable(char c) {
        return c == '.' || c == '-' || c == '_';
    }

    private static String requireContent(String content) {
        String trimmed = content == null ? "" : content.trim();
        if (trimmed.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "content is required");
        }
        if (trimmed.getBytes(StandardCharsets.UTF_8).length > MAX_SCRIPT_BYTES) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "content exceeds " + MAX_SCRIPT_BYTES + " bytes");
        }
        return trimmed;
    }

    static String sha256Hex(String content) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(content.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    private ScriptIndex readIndex() {
        Path path = indexPath();
        if (!Files.exists(path)) {
            return new ScriptIndex();
        }
        try {
            ScriptIndex index = mapper.readValue(path.toFile(), ScriptIndex.class);
            return index == null ? new ScriptIndex() : index;
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read script index");
        }
    }

    private void writeIndex(ScriptIndex index) {
        writeAtomic(indexPath(), serialize(index));
    }

    private void writeModule(ScriptModuleDocument module) {
        writeAtomic(modulePath(module.getId()), serialize(module));
    }

    private byte[] serialize(Object value) {
        try {
            return mapper.writeValueAsBytes(value);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to serialize script data");
        }
    }

    private void writeAtomic(Path target, byte[] data) {
        try {
            Files.createDirectories(scriptDir);
            Path tmp = Files.createTempFile(scriptDir, "script", ".tmp");
            Files.write(tmp, data);
            Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist script data");
        }
    }

    private Path indexPath() {
        return scriptDir.resolve("index.json");
    }

    private Path modulePath(String id) {
        String safe = sanitizeVersion(id);
        if (safe.isEmpty() || safe.contains("..")) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid script id");
        }
        Path target = scriptDir.resolve(safe + ".json").toAbsolutePath().normalize();
        if (!target.startsWith(scriptDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid script path");
        }
        return target;
    }

    private void recordAudit(String action, ScriptModule module) {
        if (audit == null || module == null) {
            return;
        }
        String detail = "id=%s version=%s sha256=%s size=%d".formatted(
            module.id(), module.version(), module.sha256(), module.size());
        try {
            audit.append(new AuditEvent(
                (Instant) null, action, null, null, null, null, null, null, null, null,
                null, null, null, null, detail));
        } catch (RuntimeException ignored) {
            // Go keeps script state changes when the best-effort audit append fails.
        }
    }
}
