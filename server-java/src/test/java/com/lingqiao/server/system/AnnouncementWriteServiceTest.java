package com.lingqiao.server.system;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class AnnouncementWriteServiceTest {
    @TempDir
    Path tempDir;

    private AnnouncementWriteService service;

    @BeforeEach
    void setUp() {
        service = new AnnouncementWriteService(tempDir);
    }

    @Test
    void publishSupersedesPreviousActiveRevision() {
        AnnouncementDocument first = service.saveRevision("first", true);
        AnnouncementDocument second = service.saveRevision("second", true);

        assertThat(first.getId()).isNotEqualTo(second.getId());
        assertThat(second.getStatus()).isEqualTo(AnnouncementWriteService.STATUS_PUBLISHED);
        assertThat(second.getPublishedAt()).isNotNull();

        AnnouncementIndex index = readIndex();
        assertThat(index.getActiveId()).isEqualTo(second.getId());
        assertThat(statusOf(index, first.getId())).isEqualTo(AnnouncementWriteService.STATUS_SUPERSEDED);
        assertThat(statusOf(index, second.getId())).isEqualTo(AnnouncementWriteService.STATUS_PUBLISHED);
    }

    @Test
    void draftRevisionDoesNotChangeActivePointer() {
        AnnouncementDocument published = service.saveRevision("live", true);
        AnnouncementDocument draft = service.saveRevision("draft", false);

        AnnouncementIndex index = readIndex();
        assertThat(draft.getStatus()).isEqualTo(AnnouncementWriteService.STATUS_DRAFT);
        assertThat(draft.getPublishedAt()).isNull();
        assertThat(index.getActiveId()).isEqualTo(published.getId());
    }

    @Test
    void publishRevisionActivatesExistingDraft() {
        AnnouncementDocument live = service.saveRevision("live", true);
        AnnouncementDocument draft = service.saveRevision("draft", false);

        AnnouncementDocument activated = service.publishRevision(draft.getId());

        AnnouncementIndex index = readIndex();
        assertThat(activated.getStatus()).isEqualTo(AnnouncementWriteService.STATUS_PUBLISHED);
        assertThat(index.getActiveId()).isEqualTo(draft.getId());
        assertThat(statusOf(index, live.getId())).isEqualTo(AnnouncementWriteService.STATUS_SUPERSEDED);
    }

    @Test
    void deleteProtectsActiveRevisionAndReportsMissing() {
        AnnouncementDocument live = service.saveRevision("live", true);
        AnnouncementDocument draft = service.saveRevision("draft", false);

        assertThatThrownBy(() -> service.deleteRevision(live.getId()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("cannot delete active announcement");

        service.deleteRevision(draft.getId());
        assertThat(readIndex().getAnnouncements()).hasSize(1);

        assertThatThrownBy(() -> service.deleteRevision(draft.getId()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("announcement revision not found");
    }

    @Test
    void oversizedContentIsRejected() {
        assertThatThrownBy(() -> service.saveRevision(
            "x".repeat(AnnouncementWriteService.MAX_ANNOUNCEMENT_BYTES + 1), true))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("exceeds");
    }

    @Test
    void publishWritesLegacyActiveFileForGoCompatibility() throws Exception {
        service.saveRevision("legacy", true);

        Path legacy = tempDir.resolve("announcement.json");
        assertThat(Files.exists(legacy)).isTrue();
        assertThat(Files.readString(legacy)).contains("legacy");
    }

    private AnnouncementIndex readIndex() {
        return new AnnouncementReadSupport(tempDir).read();
    }

    private static String statusOf(AnnouncementIndex index, String id) {
        return index.getAnnouncements().stream()
            .filter(a -> a.getId().equals(id))
            .map(AnnouncementDocument::getStatus)
            .findFirst()
            .orElseThrow();
    }
}
