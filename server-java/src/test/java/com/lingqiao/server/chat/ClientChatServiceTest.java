package com.lingqiao.server.chat;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Path;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class ClientChatServiceTest {
    @TempDir
    Path tempDir;

    private ChatStateStore store;
    private ClientChatService service;
    private ClientChatService.ClientIdentity alice;
    private ClientChatService.ClientIdentity bob;

    @BeforeEach
    void setUp() {
        store = new ChatStateStore(tempDir);
        service = new ClientChatService(store);
        alice = new ClientChatService.ClientIdentity("tok-alice", ChatIdentity.authorId("CARD-A", "M-A"));
        bob = new ClientChatService.ClientIdentity("tok-bob", ChatIdentity.authorId("CARD-B", "M-B"));
    }

    @Test
    void addAssignsDefaultNicknameAndIncrementsId() {
        ClientChatService.ChatMessageView first = service.add(alice, "  hello  ", 0);

        assertThat(first.id()).isEqualTo(1);
        assertThat(first.type()).isEqualTo("user");
        assertThat(first.content()).isEqualTo("hello");
        assertThat(first.author()).isEqualTo(ChatIdentity.defaultAuthorName(alice.authorId()));
        assertThat(store.read().getNextId()).isEqualTo(2);
    }

    @Test
    void sendRateLimitAppliesPerSessionToken() {
        service.add(alice, "first", 0);

        assertThatThrownBy(() -> service.add(alice, "second", 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("发送过于频繁");

        assertThat(service.add(bob, "from bob", 0).id()).isEqualTo(2);
    }

    @Test
    void mutedAuthorCannotSendOrReact() {
        ChatWriteService moderator = new ChatWriteService(tempDir);
        service.add(alice, "before mute", 0);
        moderator.mute(alice.authorId(), 30, "spam");

        assertThatThrownBy(() -> service.add(alice, "after mute", 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("你已被禁言");
        assertThatThrownBy(() -> service.react(alice, 1, "👍"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("你已被禁言");
    }

    @Test
    void profileUpdateRejectsBannedAndOversizedNicknames() {
        assertThat(service.profile(alice).nickname())
            .isEqualTo(ChatIdentity.defaultAuthorName(alice.authorId()));

        ClientChatService.ChatProfileView updated = service.setProfile(alice, "  小明  ");
        assertThat(updated.nickname()).isEqualTo("小明");
        assertThat(service.profile(alice).nickname()).isEqualTo("小明");

        assertThatThrownBy(() -> service.setProfile(alice, "管理员"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("nickname is not allowed");
        assertThatThrownBy(() -> service.setProfile(alice, "十七个字十七个字十七个字十七个字十七"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("nickname is required");
    }

    @Test
    void nicknameIsUsedForSubsequentMessages() {
        service.setProfile(alice, "小红");

        ClientChatService.ChatMessageView message = service.add(alice, "hi", 0);

        assertThat(message.author()).isEqualTo("小红");
    }

    @Test
    void reactToggleCountsAndReportsCurrentUser() {
        service.add(alice, "target", 0);

        ClientChatService.ChatMessageView reacted = service.react(bob, 1, "👍");
        assertThat(reacted.reactions()).containsEntry("👍", 1);
        assertThat(reacted.reacted()).containsExactly("👍");

        ClientChatService.ChatMessageView unreacted = service.react(bob, 1, "👍");
        assertThat(unreacted.reactions()).doesNotContainKey("👍");
        assertThat(unreacted.reacted()).isEmpty();

        assertThatThrownBy(() -> service.react(bob, 1, "😀"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("reaction is not allowed");
        assertThatThrownBy(() -> service.react(bob, 999, "👍"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("message not found");
    }

    @Test
    void listSkipsDeletedAndAppliesAfterId() {
        service.add(alice, "one", 0);
        service.add(bob, "two", 0);
        new ChatWriteService(tempDir).deleteMessage(1);

        List<ClientChatService.ChatMessageView> all = service.list(alice, 0);
        assertThat(all).extracting(ClientChatService.ChatMessageView::id).containsExactly(2L);

        assertThat(service.list(alice, 2)).isEmpty();
    }

    @Test
    void replyPreviewIsAttachedFromSourceMessage() {
        service.add(alice, "original message", 0);

        ClientChatService.ChatMessageView reply = service.add(bob, "reply body", 1);

        assertThat(reply.replyToId()).isEqualTo(1);
        assertThat(reply.replyPreview()).isNotNull();
        assertThat(reply.replyPreview().content()).isEqualTo("original message");
    }

    @Test
    void presenceCountsRecentAuthorsOnly() {
        assertThat(service.touchPresence(alice)).isEqualTo(1);
        assertThat(service.touchPresence(bob)).isEqualTo(2);
        assertThat(service.touchPresence(alice)).isEqualTo(2);
        assertThat(ClientChatService.PRESENCE_AGE).isEqualTo(Duration.ofSeconds(60));
        assertThat(ClientChatService.SEND_INTERVAL).isEqualTo(Duration.ofSeconds(5));
    }

    @Test
    void emptyContentIsRejected() {
        assertThatThrownBy(() -> service.add(alice, "   ", 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("message is required");
    }

    @Test
    void retentionPruningDropsOldMessagesOnRead() {
        service.add(alice, "fresh", 0);
        ChatState state = store.read();
        state.getMessages().get(0).setCreatedAt(Instant.now().minus(Duration.ofHours(25)));
        store.write(state);

        assertThat(service.list(alice, 0)).isEmpty();
    }
}
