package com.lingqiao.server.system;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/** Shared announcement index reader used by read controller and tests. */
public class AnnouncementReadSupport {
    private final Path dataDir;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    public AnnouncementReadSupport(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
    }

    public AnnouncementIndex read() {
        Path path = dataDir.resolve("announcements").resolve("index.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid announcement index path");
        }
        if (!Files.exists(path)) {
            return new AnnouncementIndex();
        }
        try {
            AnnouncementIndex index = mapper.readValue(path.toFile(), AnnouncementIndex.class);
            return index == null ? new AnnouncementIndex() : index;
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read announcement index");
        }
    }
}
