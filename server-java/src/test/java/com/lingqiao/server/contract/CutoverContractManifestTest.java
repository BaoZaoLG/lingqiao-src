package com.lingqiao.server.contract;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.Set;
import org.junit.jupiter.api.Test;

class CutoverContractManifestTest {
    @Test
    void sharedManifestContainsUniqueJavaReadRoutes() throws Exception {
        Path manifestPath = Path.of("..", "docs", "api", "java-cutover-contracts.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(manifestPath));
        assertThat(root.path("version").asInt()).isEqualTo(14);
        JsonNode routes = root.path("routes");
        assertThat(routes.isArray()).isTrue();
        assertThat(routes.size()).isGreaterThanOrEqualTo(23);

        Set<String> javaRoutes = new HashSet<>();
        for (JsonNode route : routes) {
            String javaPath = route.path("java_path").asText();
            JsonNode methods = route.path("methods");
            assertThat(methods.size()).isBetween(1, 2);
            boolean write = route.path("write").asBoolean(false);
            String response = route.path("response").asText("json");
            assertThat(javaPath).startsWith("/java/");
            assertThat(response).isIn("json", "csv", "binary", "text");
            if (response.equals("csv")) {
                assertThat(methods.get(0).asText()).isEqualTo("GET");
            }
            for (JsonNode methodNode : methods) {
                String method = methodNode.asText();
                assertThat(method).isIn("GET", "POST", "DELETE");
                assertThat(javaRoutes.add(method + " " + javaPath))
                    .as("unique route %s %s", method, javaPath).isTrue();
            }
        }
    }
}
