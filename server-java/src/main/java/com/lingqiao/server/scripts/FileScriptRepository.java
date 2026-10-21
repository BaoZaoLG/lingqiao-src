package com.lingqiao.server.scripts;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;

public class FileScriptRepository {
    private final Path scriptDir;
    private final ObjectMapper mapper;

    public FileScriptRepository(Path dataDir) {
        this.scriptDir = dataDir.toAbsolutePath().normalize().resolve("scripts").normalize();
        this.mapper = new ObjectMapper().registerModule(new JavaTimeModule());
    }

    public ScriptListResult list() {
        ScriptIndex index = readIndex();
        List<ScriptModule> scripts = new ArrayList<>();
        for (ScriptModuleDocument doc : index.getScripts()) {
            ScriptModule module = doc.toModule();
            if ((module.content() == null || module.content().isBlank()) && !module.id().isBlank()) {
                Optional<ScriptModule> stored = get(module.id());
                if (stored.isPresent()) {
                    ScriptModule payload = stored.get();
                    module = new ScriptModule(
                        module.id(),
                        module.version(),
                        payload.sha256(),
                        payload.content(),
                        payload.size(),
                        module.note(),
                        module.status(),
                        module.createdAt(),
                        module.updatedAt(),
                        module.activatedAt()
                    );
                }
            }
            scripts.add(module);
        }
        scripts.sort(Comparator.comparing(ScriptModule::updatedAt).reversed());
        ScriptModule active = scripts.stream()
            .filter(s -> s.id().equals(index.getActiveId()))
            .findFirst()
            .orElse(null);
        return new ScriptListResult(scripts, index.getActiveId(), active);
    }

    public Optional<ScriptModule> get(String id) {
        Path path = modulePath(id);
        if (!Files.exists(path)) {
            return Optional.empty();
        }
        try {
            ScriptModuleDocument doc = mapper.readValue(path.toFile(), ScriptModuleDocument.class);
            return Optional.of(doc.toModule());
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read script module");
        }
    }

    public ScriptModule require(String id) {
        return get(id).orElseThrow(() -> new SidecarException(ErrorCode.NOT_FOUND, "script not found"));
    }

    public ScriptModule requireActive() {
        ScriptIndex index = readIndex();
        if (index.getActiveId().isBlank()) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "script module not published");
        }
        return require(index.getActiveId());
    }

    private ScriptIndex readIndex() {
        Path indexPath = scriptDir.resolve("index.json");
        if (!Files.exists(indexPath)) {
            return new ScriptIndex();
        }
        try {
            return mapper.readValue(indexPath.toFile(), ScriptIndex.class);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read script index");
        }
    }

    private Path modulePath(String id) {
        String safe = id == null ? "" : id.trim();
        if (!safe.matches("^[A-Za-z0-9._-]+$") || safe.contains("..")) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid script id");
        }
        Path target = scriptDir.resolve(safe + ".json").toAbsolutePath().normalize();
        if (!target.startsWith(scriptDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid script path");
        }
        return target;
    }
}
