package com.lingqiao.server.agentportal;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import java.io.IOException;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Instant;
import java.util.HashMap;
import java.util.Iterator;
import java.util.Map;

/**
 * Reads and writes the two JSON maps used by Go's AgentHandler. Keys are
 * SHA-256 token hashes; raw bearer tokens are never persisted.
 */
public final class AgentSessionStore {
    private static final TypeReference<Map<String, Instant>> SESSION_TYPE = new TypeReference<>() {
    };
    private static final TypeReference<Map<String, String>> TOKEN_MAP_TYPE = new TypeReference<>() {
    };

    private final ObjectMapper mapper;
    private final Path sessionsPath;
    private final Path tokenMapPath;
    private final boolean persistent;
    private final boolean writable;
    private Map<String, Instant> sessions = new HashMap<>();
    private Map<String, String> tokenMap = new HashMap<>();

    public AgentSessionStore(Path dataDir) {
        this(dataDir, true, true);
    }

    private AgentSessionStore(Path dataDir, boolean persistent, boolean writable) {
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS);
        this.persistent = persistent;
        this.writable = writable;
        Path normalized = dataDir.toAbsolutePath().normalize();
        this.sessionsPath = normalized.resolve("agent_sessions.json");
        this.tokenMapPath = normalized.resolve("agent_token_map.json");
        if (persistent) {
            try {
                Files.createDirectories(normalized);
            } catch (IOException ex) {
                throw new IllegalStateException("failed to create agent session directory", ex);
            }
            reload();
            pruneExpired(Instant.now());
        }
    }

    static AgentSessionStore inMemory() {
        return new AgentSessionStore(Path.of("."), false, true);
    }

    static AgentSessionStore readOnly(Path dataDir) {
        return new AgentSessionStore(dataDir, true, false);
    }

    public synchronized void put(String tokenHash, String agentId, Instant expiresAt) {
        requireWritable();
        reload();
        sessions.put(tokenHash, expiresAt);
        tokenMap.put(tokenHash, agentId);
        persist();
    }

    public synchronized StoredSession find(String tokenHash, Instant now) {
        reload();
        Instant expiresAt = sessions.get(tokenHash);
        String agentId = tokenMap.get(tokenHash);
        if (expiresAt == null || agentId == null || !now.isBefore(expiresAt)) {
            if (expiresAt != null || agentId != null) {
                sessions.remove(tokenHash);
                tokenMap.remove(tokenHash);
                if (writable) {
                    persist();
                }
            }
            return null;
        }
        return new StoredSession(agentId, expiresAt);
    }

    public synchronized void remove(String tokenHash) {
        requireWritable();
        reload();
        boolean changed = sessions.remove(tokenHash) != null;
        changed |= tokenMap.remove(tokenHash) != null;
        if (changed) {
            persist();
        }
    }

    private void pruneExpired(Instant now) {
        boolean changed = false;
        Iterator<Map.Entry<String, Instant>> iterator = sessions.entrySet().iterator();
        while (iterator.hasNext()) {
            Map.Entry<String, Instant> entry = iterator.next();
            if (entry.getValue() == null || !now.isBefore(entry.getValue())) {
                tokenMap.remove(entry.getKey());
                iterator.remove();
                changed = true;
            }
        }
        if (tokenMap.keySet().removeIf(key -> !sessions.containsKey(key))) {
            changed = true;
        }
        if (changed && writable) {
            persist();
        }
    }

    private void reload() {
        if (!persistent) {
            return;
        }
        sessions = read(sessionsPath, SESSION_TYPE);
        tokenMap = read(tokenMapPath, TOKEN_MAP_TYPE);
    }

    private <T> Map<String, T> read(Path path, TypeReference<Map<String, T>> type) {
        if (!Files.isRegularFile(path)) {
            return new HashMap<>();
        }
        try {
            Map<String, T> value = mapper.readValue(path.toFile(), type);
            return value == null ? new HashMap<>() : new HashMap<>(value);
        } catch (IOException ex) {
            // Go treats an unreadable or malformed session file as an empty map.
            return new HashMap<>();
        }
    }

    private void persist() {
        if (!persistent) {
            return;
        }
        requireWritable();
        writeAtomically(sessionsPath, sessions);
        writeAtomically(tokenMapPath, tokenMap);
    }

    private void requireWritable() {
        if (!writable) {
            throw new IllegalStateException("agent session store is read-only");
        }
    }

    private void writeAtomically(Path target, Object value) {
        Path temp = target.resolveSibling(target.getFileName() + ".tmp");
        try {
            mapper.writeValue(temp.toFile(), value);
            try {
                Files.move(temp, target, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
            } catch (AtomicMoveNotSupportedException ex) {
                Files.move(temp, target, StandardCopyOption.REPLACE_EXISTING);
            }
            try {
                Files.setPosixFilePermissions(target, PosixFilePermissions.fromString("rw-------"));
            } catch (UnsupportedOperationException ignored) {
                // Windows does not expose POSIX permissions.
            }
        } catch (IOException ex) {
            try {
                Files.deleteIfExists(temp);
            } catch (IOException ignored) {
                // Preserve the original write failure.
            }
            throw new IllegalStateException("failed to persist agent sessions", ex);
        }
    }

    public record StoredSession(String agentId, Instant expiresAt) {
    }
}
