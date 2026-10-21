package com.lingqiao.server.common.jdbc;

import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;

/**
 * Formats timestamps the way Go writes them into SQLite DATETIME columns.
 *
 * <p>Go uses {@code time.RFC3339Nano} (see cards.formatDBTime and
 * releases.formatTime), which is the first layout its readers try. Passing a
 * {@link java.sql.Timestamp} instead lets the JDBC driver choose its own format,
 * which Go's parser does not accept in every case, so writes must go through here.
 */
public final class TimestampWriter {
    private static final DateTimeFormatter RFC3339_NANO =
        DateTimeFormatter.ofPattern("uuuu-MM-dd'T'HH:mm:ss[.SSSSSSSSS]'Z'").withZone(ZoneOffset.UTC);

    private TimestampWriter() {
    }

    /** Returns null for a null input so callers can bind SQL NULL directly. */
    public static String format(Instant instant) {
        if (instant == null) {
            return null;
        }
        Instant utc = instant.atOffset(ZoneOffset.UTC).toInstant();
        if (utc.getNano() == 0) {
            return DateTimeFormatter.ISO_INSTANT.format(utc);
        }
        return RFC3339_NANO.format(utc);
    }
}
