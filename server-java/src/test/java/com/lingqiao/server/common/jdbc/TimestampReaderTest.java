package com.lingqiao.server.common.jdbc;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class TimestampReaderTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("CREATE TABLE stamps (label TEXT PRIMARY KEY, at DATETIME)");
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void parsesGoRfc3339Text() {
        insert("rfc3339", "2026-07-01T12:34:56Z");

        assertThat(read("rfc3339")).isEqualTo(Instant.parse("2026-07-01T12:34:56Z"));
    }

    @Test
    void parsesDriverNativeText() {
        insert("native", "2026-07-01 12:34:56.000");

        assertThat(read("native")).isNotNull();
    }

    @Test
    void returnsNullForNullAndBlank() {
        insert("nullish", null);
        insert("blank", "");

        assertThat(read("nullish")).isNull();
        assertThat(read("blank")).isNull();
    }

    @Test
    void returnsNullForGarbage() {
        insert("garbage", "not-a-timestamp");

        assertThat(read("garbage")).isNull();
    }

    @Test
    void readOrEpochSubstitutesEpoch() {
        insert("nullish", null);

        List<Instant> values = jdbc.query(
            "SELECT at FROM stamps WHERE label = 'nullish'",
            (rs, rowNum) -> TimestampReader.readOrEpoch(rs, "at"));

        assertThat(values).containsExactly(Instant.EPOCH);
    }

    @Test
    void handlesRfc3339WithFractionalSeconds() {
        insert("fractional", "2026-07-01T12:34:56.789Z");

        assertThat(read("fractional")).isEqualTo(Instant.parse("2026-07-01T12:34:56.789Z"));
    }

    @Test
    void preservesGoNanosecondsAndOffsetInsteadOfUsingDriverTimestampParser() {
        insert("go-nano", "2026-07-31T11:35:44.937868479+08:00");

        assertThat(read("go-nano"))
            .isEqualTo(Instant.parse("2026-07-31T03:35:44.937868479Z"));
    }

    @Test
    void parsesGoTimeStringWithZoneAndMonotonicSuffix() {
        insert("go-string",
            "2026-07-31 11:35:44.937868479 +0800 CST m=+28.034802110");

        assertThat(read("go-string"))
            .isEqualTo(Instant.parse("2026-07-31T03:35:44.937868479Z"));
    }

    @Test
    void parsesGoTimeStringWithoutMonotonicSuffix() {
        insert("go-string", "2026-05-22 17:05:41.518787631 +0800 CST");

        assertThat(read("go-string"))
            .isEqualTo(Instant.parse("2026-05-22T09:05:41.518787631Z"));
    }

    private void insert(String label, String value) {
        jdbc.update("INSERT INTO stamps (label, at) VALUES (?, ?)", label, value);
    }

    private Instant read(String label) {
        List<Instant> values = jdbc.query(
            "SELECT at FROM stamps WHERE label = ?",
            (rs, rowNum) -> TimestampReader.read(rs, "at"), label);
        return values.isEmpty() ? null : values.get(0);
    }
}
