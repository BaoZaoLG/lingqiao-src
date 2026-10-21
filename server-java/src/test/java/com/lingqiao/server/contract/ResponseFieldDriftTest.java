package com.lingqiao.server.contract;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;

/**
 * Guard test: Java responses must not invent fields the Go handlers never send.
 *
 * <p>The admin and agent panels are shared between both backends, so an extra
 * field is a silent contract drift that only shows up after cutover.
 */
class ResponseFieldDriftTest {
    /** Field names that exist in no Go handler response. */
    private static final List<String> FORBIDDEN_FIELDS = List.of("read_only");

    @Test
    void noControllerEmitsFieldsGoDoesNotHave() throws IOException {
        Path root = Path.of("src", "main", "java", "com", "lingqiao", "server");
        assertThat(Files.isDirectory(root)).isTrue();

        List<String> offenders = new ArrayList<>();
        try (Stream<Path> files = Files.walk(root)) {
            for (Path file : files.filter(p -> p.toString().endsWith(".java")).toList()) {
                String source = Files.readString(file, StandardCharsets.UTF_8);
                for (String field : FORBIDDEN_FIELDS) {
                    if (source.contains("\"" + field + "\"")) {
                        offenders.add(file.getFileName() + " -> " + field);
                    }
                }
            }
        }

        assertThat(offenders)
            .as("Go sends no such field; remove it: %s", offenders)
            .isEmpty();
    }
}
