package com.lingqiao.server.cards;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import org.junit.jupiter.api.Test;

class CardCodesGoldenTest {
    @Test
    void matchesSharedCardCodeGoldenVectors() throws Exception {
        Path path = Path.of("..", "docs", "api", "cardcode-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));

        assertThat(root.path("version").asInt()).isEqualTo(1);
        assertThat(root.path("alphabet").asText()).isEqualTo(CardCodes.ALPHABET);
        assertThat(root.path("normalized_length").asInt()).isEqualTo(CardCodes.NORMALIZED_LENGTH);

        for (JsonNode node : root.path("normalize")) {
            assertThat(CardCodes.normalize(node.path("input").asText()))
                .as("normalize %s", node.path("input").asText())
                .isEqualTo(node.path("normalized").asText());
        }
        for (JsonNode node : root.path("same")) {
            assertThat(CardCodes.same(node.path("a").asText(), node.path("b").asText()))
                .as("same %s/%s", node.path("a").asText(), node.path("b").asText())
                .isEqualTo(node.path("equal").asBoolean());
        }
        for (JsonNode node : root.path("format_cases")) {
            assertThat(CardCodes.format(node.path("input").asText()))
                .as("format %s", node.path("input").asText())
                .isEqualTo(node.path("formatted").asText());
        }
        for (JsonNode node : root.path("validate")) {
            Optional<String> normalized = CardCodes.validateImport(node.path("input").asText());
            if (node.path("valid").asBoolean()) {
                assertThat(normalized).contains(node.path("normalized").asText());
            } else {
                assertThat(normalized).isEmpty();
            }
        }
    }
}
