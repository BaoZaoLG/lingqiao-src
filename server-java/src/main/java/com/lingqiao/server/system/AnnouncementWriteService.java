package com.lingqiao.server.system;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;

/**
 * Java write path for announcements. Mirrors Go system.AnnouncementService
 * revision/publish/delete semantics on the same on-disk layout.
 */
public class AnnouncementWriteService {
    static final int MAX_ANNOUNCEMENT_BYTES = 64 << 10;
    static final String STATUS_DRAFT = "draft";
    static final String STATUS_PUBLISHED = "published";
    static final String STATUS_SUPERSEDED = "superseded";

    private static final DateTimeFormatter ID_FORMAT =
        DateTimeFormatter.ofPattern("yyyyMMdd'T'HHmmssSSS'Z'").withZone(ZoneOffset.UTC);

    private final Path dataDir;
    private final ObjectMapper mapper;

    public AnnouncementWriteService(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .enable(SerializationFeature.INDENT_OUTPUT);
    }

    public AnnouncementDocument saveRevision(String content, boolean publish) {
        String value = content == null ? "" : content;
        if (value.getBytes(StandardCharsets.UTF_8).length > MAX_ANNOUNCEMENT_BYTES) {
            throw new SidecarException(ErrorCode.BAD_REQUEST,
                "content exceeds " + MAX_ANNOUNCEMENT_BYTES + " bytes");
        }
        Instant now = Instant.now();
        AnnouncementDocument revision = new AnnouncementDocument();
        revision.setId(newId(now));
        revision.setContent(value);
        revision.setCreatedAt(now);
        revision.setUpdatedAt(now);
        revision.setStatus(publish ? STATUS_PUBLISHED : STATUS_DRAFT);
        if (publish) {
            revision.setPublishedAt(now);
        }

        AnnouncementIndex index = readIndex();
        List<AnnouncementDocument> revisions = new ArrayList<>();
        revisions.add(revision);
        revisions.addAll(index.getAnnouncements());
        index.setAnnouncements(revisions);
        if (publish) {
            index.setActiveId(revision.getId());
            markActive(index, revision.getId());
        }
        writeIndex(index);
        if (publish) {
            writeLegacyActive(revision);
        }
        return revision;
    }

    public AnnouncementDocument publishRevision(String id) {
        AnnouncementIndex index = readIndex();
        AnnouncementDocument selected = index.getAnnouncements().stream()
            .filter(a -> a.getId().equals(id))
            .findFirst()
            .orElseThrow(() -> new SidecarException(ErrorCode.NOT_FOUND, "announcement revision not found"));

        Instant now = Instant.now();
        selected.setStatus(STATUS_PUBLISHED);
        selected.setUpdatedAt(now);
        selected.setPublishedAt(now);
        index.setActiveId(id);
        markActive(index, id);
        writeIndex(index);
        writeLegacyActive(selected);
        return selected;
    }

    public void deleteRevision(String id) {
        AnnouncementIndex index = readIndex();
        if (index.getActiveId().equals(id)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "cannot delete active announcement");
        }
        List<AnnouncementDocument> remaining = new ArrayList<>();
        boolean found = false;
        for (AnnouncementDocument doc : index.getAnnouncements()) {
            if (doc.getId().equals(id)) {
                found = true;
                continue;
            }
            remaining.add(doc);
        }
        if (!found) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "announcement revision not found");
        }
        index.setAnnouncements(remaining);
        writeIndex(index);
    }

    static void markActive(AnnouncementIndex index, String activeId) {
        for (AnnouncementDocument doc : index.getAnnouncements()) {
            if (doc.getId().equals(activeId)) {
                doc.setStatus(STATUS_PUBLISHED);
            } else if (STATUS_PUBLISHED.equals(doc.getStatus())) {
                doc.setStatus(STATUS_SUPERSEDED);
            }
        }
    }

    private String newId(Instant now) {
        return "ann-" + ID_FORMAT.format(now) + "-" + (now.getEpochSecond() * 1_000_000_000L + now.getNano());
    }

    private AnnouncementIndex readIndex() {
        Path path = indexPath();
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

    private void writeIndex(AnnouncementIndex index) {
        writeAtomic(indexPath(), serialize(index));
    }

    private void writeLegacyActive(AnnouncementDocument active) {
        writeAtomic(dataDir.resolve("announcement.json"), serialize(active));
    }

    private byte[] serialize(Object value) {
        try {
            return mapper.writeValueAsBytes(value);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to serialize announcement data");
        }
    }

    private void writeAtomic(Path target, byte[] data) {
        try {
            Files.createDirectories(target.getParent());
            Path tmp = Files.createTempFile(target.getParent(), "announcement", ".tmp");
            Files.write(tmp, data);
            Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist announcement data");
        }
    }

    private Path indexPath() {
        return dataDir.resolve("announcements").resolve("index.json");
    }
}
