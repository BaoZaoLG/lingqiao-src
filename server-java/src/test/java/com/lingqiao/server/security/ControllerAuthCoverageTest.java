package com.lingqiao.server.security;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Set;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;

/**
 * Guard test: every Java sidecar controller that exposes business data must
 * authenticate. Only introspection endpoints are allowed to skip auth.
 */
class ControllerAuthCoverageTest {
    private static final Set<String> ALLOWED_WITHOUT_AUTH = Set.of(
        "HealthController.java",
        "CutoverController.java",
        "ClientAnnouncementController.java",
        "WebAssetController.java"
    );

    @Test
    void everyBusinessControllerEnforcesAuthentication() throws IOException {
        Path controllersRoot = Path.of("src", "main", "java", "com", "lingqiao", "server");
        assertThat(Files.isDirectory(controllersRoot)).isTrue();

        List<String> unauthenticated = new ArrayList<>();
        try (Stream<Path> files = Files.walk(controllersRoot)) {
            for (Path file : files.filter(p -> p.toString().endsWith(".java")).toList()) {
                String source = Files.readString(file, StandardCharsets.UTF_8);
                if (!source.contains("@RestController")) {
                    continue;
                }
                if (source.contains("@RestControllerAdvice")) {
                    continue;
                }
                String name = file.getFileName().toString();
                if (ALLOWED_WITHOUT_AUTH.contains(name)) {
                    continue;
                }
                boolean authenticates = source.contains("requireAdmin")
                    || source.contains("hmac.verify")
                    || source.contains("hmacAuthService.verify")
                    || source.contains("sessions.requireActive")
                    || source.contains("auth.requireSession");
                if (!authenticates) {
                    unauthenticated.add(name);
                }
            }
        }

        assertThat(unauthenticated)
            .as("controllers missing authentication: %s", unauthenticated)
            .isEmpty();
    }

    @Test
    void introspectionAllowListStaysMinimal() {
        assertThat(ALLOWED_WITHOUT_AUTH).hasSize(4);
    }
}
