package com.lingqiao.server.common.jdbc;

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
 * Guard test: production code must not bind java.sql.Timestamp into SQLite
 * DATETIME columns. The driver stores epoch millis, which Go's text-layout
 * parsers decode as the zero time, silently corrupting cross-language reads.
 */
class TimestampWriterUsageTest {
    @Test
    void productionCodeNeverBindsSqlTimestamp() throws IOException {
        Path root = Path.of("src", "main", "java", "com", "lingqiao", "server");
        assertThat(Files.isDirectory(root)).isTrue();

        List<String> offenders = new ArrayList<>();
        try (Stream<Path> files = Files.walk(root)) {
            for (Path file : files.filter(p -> p.toString().endsWith(".java")).toList()) {
                String source = Files.readString(file, StandardCharsets.UTF_8);
                if (source.contains("Timestamp.from(") || source.contains("setTimestamp(")) {
                    offenders.add(file.getFileName().toString());
                }
            }
        }

        assertThat(offenders)
            .as("bind timestamps via TimestampWriter.format instead: %s", offenders)
            .isEmpty();
    }
}
