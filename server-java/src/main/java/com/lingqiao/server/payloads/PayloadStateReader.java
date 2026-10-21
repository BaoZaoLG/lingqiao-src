package com.lingqiao.server.payloads;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/** Shared read-only accessor for payloads.json. */
public class PayloadStateReader {
    private final Path dataDir;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    public PayloadStateReader(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
    }

    public PayloadState read() {
        Path path = dataDir.resolve("payloads.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload state path");
        }
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
}
