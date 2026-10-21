package com.lingqiao.server.chat;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class ChatWriteServiceTest {
    @TempDir
    Path tempDir;

    private ChatWriteService service;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    @BeforeEach
    void setUp() {
        service = new ChatWriteService(tempDir);
    }

    @Test
    void systemMessageIncrementsNextIdAndPersists() {
        ChatDocument first = service.addSystemMessage("  hello  ");
        ChatDocument second = service.addSystemMessage("world");

        assertThat(first.getId()).isEqualTo(1);
        assertThat(second.getId()).isEqualTo(2);
        assertThat(first.getType()).isEqualTo("system");
        assertThat(first.getAuthor()).isEqualTo("系统");
        assertThat(first.getContent()).isEqualTo("hello");

        ChatState state = readState();
        assertThat(state.getNextId()).isEqualTo(3);
        assertThat(state.getMessages()).hasSize(2);
    }

    @Test
    void deleteMarksMessageDeletedAndRejectsUnknown() {
        ChatDocument message = service.addSystemMessage("hello");

        service.deleteMessage(message.getId());
        assertThat(readState().getMessages().get(0).isDeleted()).isTrue();

        assertThatThrownBy(() -> service.deleteMessage(message.getId()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("message not found");
        assertThatThrownBy(() -> service.deleteMessage(0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("id is required");
    }

    @Test
    void muteUsesDefaultDurationAndTrimsFields() {
        ChatMuteDocument mute = service.mute("  author-1  ", 0, "  spam  ");

        assertThat(mute.getAuthorId()).isEqualTo("author-1");
        assertThat(mute.getReason()).isEqualTo("spam");
        assertThat(mute.getExpires()).isAfter(Instant.now().plusSeconds(3500));

        assertThat(readState().getMutes()).containsKey("author-1");
        assertThatThrownBy(() -> service.mute("  ", 10, ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("author_id is required");
    }

    @Test
    void contentIsTruncatedToRuneLimit() {
        String content = "字".repeat(ChatWriteService.MAX_CHAT_RUNES + 50);
        ChatDocument message = service.addSystemMessage(content);

        assertThat(message.getContent().codePointCount(0, message.getContent().length()))
            .isEqualTo(ChatWriteService.MAX_CHAT_RUNES);
        assertThatThrownBy(() -> service.addSystemMessage("   "))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("message is required");
    }

    @Test
    void retentionPruningDropsMessagesOlderThanMaxAge() {
        Instant now = Instant.parse("2026-07-02T00:00:00Z");
        ChatDocument stale = new ChatDocument();
        stale.setId(1);
        stale.setCreatedAt(now.minus(ChatWriteService.MAX_AGE).minusSeconds(60));
        ChatDocument fresh = new ChatDocument();
        fresh.setId(2);
        fresh.setCreatedAt(now.minusSeconds(60));

        assertThat(ChatWriteService.prune(java.util.List.of(stale, fresh), now))
            .extracting(ChatDocument::getId)
            .containsExactly(2L);
    }

    @Test
    void expiredMutesAreDroppedOnWrite() {
        Instant now = Instant.now();
        ChatMuteDocument expired = new ChatMuteDocument();
        expired.setAuthorId("old");
        expired.setExpires(now.minusSeconds(60));
        ChatMuteDocument active = new ChatMuteDocument();
        active.setAuthorId("new");
        active.setExpires(now.plusSeconds(600));

        assertThat(ChatWriteService.pruneExpiredMutes(
            new java.util.LinkedHashMap<>(java.util.Map.of("old", expired, "new", active)), now))
            .containsOnlyKeys("new");
    }

    private ChatState readState() {
        try {
            Path path = tempDir.resolve("chat.json");
            assertThat(Files.exists(path)).isTrue();
            return mapper.readValue(path.toFile(), ChatState.class);
        } catch (Exception ex) {
            throw new AssertionError(ex);
        }
    }
}
