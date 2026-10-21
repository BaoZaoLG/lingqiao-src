package com.lingqiao.server.common.jdbc;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.format.DateTimeParseException;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Reads timestamp columns written by the Go server.
 *
 * <p>Go stores SQLite DATETIME values as RFC3339 text, but the JDBC driver only
 * parses its own "yyyy-MM-dd HH:mm:ss.SSS" shape and throws on anything else.
 * Every read path must therefore fall back to string parsing.
 */
public final class TimestampReader {
    private static final Pattern GO_TIME_STRING = Pattern.compile(
        "^(\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}(?:\\.\\d{1,9})?)"
            + "\\s+([+-]\\d{4})(?:\\s+\\S+)?(?:\\s+m=[+-]?\\d+(?:\\.\\d+)?)?$");

    private TimestampReader() {
    }

    /** Returns null when the column is NULL, blank, or unparsable. */
    public static Instant read(ResultSet rs, String column) throws SQLException {
        String raw = rs.getString(column);
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            // Always prefer the stored RFC3339 text. Xerial's getTimestamp()
            // silently misinterprets Go timestamps with offsets and nanoseconds.
            return Instant.parse(raw.trim());
        } catch (DateTimeParseException ignored) {
            // Driver-native timestamps are handled below.
        }
        Matcher goTime = GO_TIME_STRING.matcher(raw.trim());
        if (goTime.matches()) {
            String offset = goTime.group(2);
            String normalized = goTime.group(1).replace(' ', 'T')
                + offset.substring(0, 3) + ":" + offset.substring(3);
            try {
                return OffsetDateTime.parse(normalized).toInstant();
            } catch (DateTimeParseException ignored) {
                // Driver-native timestamps are handled below.
            }
        }
        try {
            Timestamp timestamp = rs.getTimestamp(column);
            if (timestamp != null) {
                return timestamp.toInstant();
            }
        } catch (SQLException ignored) {
            // Driver could not parse the native form.
        }
        return null;
    }

    /** Same as {@link #read} but substitutes the epoch for a missing value. */
    public static Instant readOrEpoch(ResultSet rs, String column) throws SQLException {
        Instant value = read(rs, column);
        return value == null ? Instant.EPOCH : value;
    }
}
