package com.lingqiao.server.common.jdbc;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.DriverManager;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

/**
 * Pins the on-disk text format that Java writes into SQLite DATETIME columns.
 * Go parses these columns with a fixed list of layouts, so the stored shape is
 * part of the cross-language contract.
 */
class TimestampWriteFormatTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("CREATE TABLE t (label TEXT, at DATETIME)");
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void driverStoresSqlTimestampAsEpochMillisWhichGoCannotParse() {
        jdbc.update("INSERT INTO t (label, at) VALUES (?, ?)",
            "driver", Timestamp.from(Instant.parse("2026-07-01T12:34:56Z")));

        String raw = jdbc.queryForObject(
            "SELECT at FROM t WHERE label = 'driver'", String.class);

        // The driver writes epoch millis. Go's parseDBTime only tries text layouts,
        // so it would decode this as the zero time. Writes must use TimestampWriter.
        assertThat(raw).isEqualTo("1782909296000");
        assertThat(raw).doesNotContain("T");
    }

    @Test
    void goCompatibleWriterStoresRfc3339Nano() {
        jdbc.update("INSERT INTO t (label, at) VALUES (?, ?)",
            "rfc", TimestampWriter.format(Instant.parse("2026-07-01T12:34:56.123456789Z")));

        String raw = jdbc.queryForObject(
            "SELECT at FROM t WHERE label = 'rfc'", String.class);

        assertThat(raw).isEqualTo("2026-07-01T12:34:56.123456789Z");
    }

    @Test
    void goCompatibleWriterKeepsSecondPrecisionWhenExact() {
        assertThat(TimestampWriter.format(Instant.parse("2026-07-01T12:34:56Z")))
            .isEqualTo("2026-07-01T12:34:56Z");
    }

    @Test
    void goCompatibleWriterNormalizesToUtc() {
        assertThat(TimestampWriter.format(Instant.parse("2026-07-01T00:00:00Z")))
            .endsWith("Z");
    }

    @Test
    void nullInstantBecomesNull() {
        assertThat(TimestampWriter.format(null)).isNull();
    }

    @Test
    void roundTripThroughReaderPreservesValue() {
        Instant original = Instant.parse("2026-07-01T12:34:56.789Z");
        jdbc.update("INSERT INTO t (label, at) VALUES (?, ?)", "round", TimestampWriter.format(original));

        List<Instant> read = jdbc.query(
            "SELECT at FROM t WHERE label = 'round'",
            (rs, rowNum) -> TimestampReader.read(rs, "at"));

        assertThat(read).containsExactly(original);
    }
}
