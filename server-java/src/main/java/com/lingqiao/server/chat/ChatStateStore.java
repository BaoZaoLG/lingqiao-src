package com.lingqiao.server.chat;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;

/** Shared chat.json reader/writer used by client and admin chat paths. */
public class ChatStateStore {
    private final Path dataDir;
    private final ObjectMapper mapper;

    public ChatStateStore(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .enable(SerializationFeature.INDENT_OUTPUT);
    }

    public ChatState read() {
        Path path = statePath();
        if (!Files.exists(path)) {
            return new ChatState();
        }
        try {
            ChatState state = mapper.readValue(path.toFile(), ChatState.class);
            return state == null ? new ChatState() : state;
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read chat state");
        }
    }

    public void write(ChatState state) {
        Path target = statePath();
        try {
            Files.createDirectories(target.getParent());
            Path tmp = Files.createTempFile(target.getParent(), "chat", ".tmp");
            Files.write(tmp, mapper.writeValueAsBytes(state));
            Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist chat state");
        }
    }

    private Path statePath() {
        Path path = dataDir.resolve("chat.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid chat state path");
        }
        return path;
    }
}
