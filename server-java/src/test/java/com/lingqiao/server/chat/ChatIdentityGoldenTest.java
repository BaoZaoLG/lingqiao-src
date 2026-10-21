package com.lingqiao.server.chat;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class ChatIdentityGoldenTest {
    @Test
    void authorIdentityMatchesGoGoldenVectors() throws Exception {
        JsonNode root = golden();

        for (JsonNode testCase : root.path("author_id").path("cases")) {
            String cardCode = testCase.path("card_code").asText();
            String machineId = testCase.path("machine_id").asText();

            String authorId = ChatIdentity.authorId(cardCode, machineId);
            assertThat(authorId)
                .as("authorId %s|%s", cardCode, machineId)
                .isEqualTo(testCase.path("author_id").asText());
            assertThat(ChatIdentity.defaultAuthorName(authorId))
                .isEqualTo(testCase.path("author_name").asText());
        }
    }

    @Test
    void nicknameRulesMatchGoGoldenVectors() throws Exception {
        JsonNode nickname = golden().path("nickname");
        assertThat(nickname.path("max_runes").asInt()).isEqualTo(ChatIdentity.MAX_NICKNAME_RUNES);

        for (JsonNode testCase : nickname.path("cases")) {
            String input = testCase.path("input").asText();
            assertThat(ChatIdentity.normalizeNickname(input))
                .as("normalize %s", input)
                .isEqualTo(testCase.path("normalized").asText());
            assertThat(ChatIdentity.nicknameAllowed(input))
                .as("allowed %s", input)
                .isEqualTo(testCase.path("allowed").asBoolean());
        }
    }

    @Test
    void reactionWhitelistMatchesGo() throws Exception {
        JsonNode reactions = golden().path("reactions");
        for (JsonNode allowed : reactions.path("allowed")) {
            assertThat(ChatIdentity.validReaction(allowed.asText())).isTrue();
        }
        for (JsonNode rejected : reactions.path("rejected")) {
            assertThat(ChatIdentity.validReaction(rejected.asText())).isFalse();
        }
        assertThat(ChatIdentity.validReaction(null)).isFalse();
    }

    @Test
    void replyPreviewTruncatesByCodePoints() {
        String longContent = "字".repeat(ChatIdentity.REPLY_PREVIEW_RUNES + 20);

        String preview = ChatIdentity.replyPreview(longContent);

        assertThat(preview.codePointCount(0, preview.length()))
            .isEqualTo(ChatIdentity.REPLY_PREVIEW_RUNES);
        assertThat(ChatIdentity.replyPreview("短")).isEqualTo("短");
        assertThat(ChatIdentity.replyPreview(null)).isEmpty();
    }

    private static JsonNode golden() throws Exception {
        Path path = Path.of("..", "docs", "api", "chat-identity-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));
        assertThat(root.path("version").asInt()).isEqualTo(1);
        return root;
    }
}
