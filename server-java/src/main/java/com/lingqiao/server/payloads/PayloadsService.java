package com.lingqiao.server.payloads;

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

public class PayloadsService {
    private final Path dataDir;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    public PayloadsService(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
    }

    public PayloadListResult list() {
        PayloadState state = readState();
        List<Payload> payloads = new ArrayList<>();
        for (PayloadDocument doc : state.getPayloads().values()) {
            payloads.add(doc.toSummary(doc.getPayloadId().equals(state.getActiveId())));
        }
        payloads.sort(Comparator.comparing(Payload::createdAt).reversed());
        return new PayloadListResult(payloads, state.getActiveId());
    }

    private PayloadState readState() {
        Path path = dataDir.resolve("payloads.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid payload state path");
        }
        if (!Files.exists(path)) {
            return new PayloadState();
        }
        try {
            return mapper.readValue(path.toFile(), PayloadState.class);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read payload state");
        }
    }
}
