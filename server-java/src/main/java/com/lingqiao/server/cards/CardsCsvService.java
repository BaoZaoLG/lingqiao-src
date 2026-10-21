package com.lingqiao.server.cards;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Objects;
import java.util.Set;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/** Go-compatible card CSV import/export, including spreadsheet formula hardening. */
public class CardsCsvService {
    private static final DateTimeFormatter CSV_TIME =
        DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")
            .withZone(ZoneId.of("Asia/Shanghai"));

    private final JdbcTemplate jdbc;
    private final CardsService reader;
    private final CardsWriteService writer;
    private final TransactionTemplate transactions;

    public CardsCsvService(JdbcTemplate jdbc, CardsService reader, CardsWriteService writer) {
        this.jdbc = jdbc;
        this.reader = reader;
        this.writer = writer;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public byte[] exportCsv() {
        return exportCsv(reader.list(new CardListFilter(
            null, null, null, null, null, null, null, null)));
    }

    public static byte[] exportCsv(List<Card> cards) {
        StringBuilder csv = new StringBuilder();
        csv.append('\uFEFF');
        appendRecord(csv, List.of(
            "code", "status", "note", "max_sessions", "machine_id",
            "created_at", "activated_at", "expires_at"));
        for (Card card : cards) {
            appendRecord(csv, List.of(
                spreadsheetSafe(card.code()),
                spreadsheetSafe(card.status()),
                spreadsheetSafe(card.note()),
                Integer.toString(card.maxSessions()),
                spreadsheetSafe(card.machineId()),
                formatTime(card.createdAt()),
                formatTime(card.activatedAt()),
                formatTime(card.expiresAt())
            ));
        }
        return csv.toString().getBytes(StandardCharsets.UTF_8);
    }

    public ImportReport importCsv(
        String source, Duration duration, int maxSessions, boolean dryRun
    ) {
        List<List<String>> records = parseCsv(source);
        return transactions.execute(status -> importRecords(records, duration, maxSessions, dryRun));
    }

    public void recordBulkAudit(String action, int affected) {
        appendAudit(
            "cards_bulk_updated", "", "Action: " + action + ", Affected: " + affected);
    }

    private ImportReport importRecords(
        List<List<String>> records, Duration duration, int maxSessions, boolean dryRun
    ) {
        List<ImportItem> items = new ArrayList<>(records.size());
        Set<String> seen = new HashSet<>();
        MutableCounts counts = new MutableCounts();

        for (int index = 0; index < records.size(); index++) {
            List<String> record = records.get(index);
            if (record.isEmpty()) {
                continue;
            }
            String code = record.get(0).trim();
            String lower = code.toLowerCase(Locale.ROOT);
            if ("code".equals(lower) || lower.contains("卡密")) {
                continue;
            }

            counts.totalRows++;
            if (code.isEmpty()) {
                counts.skipped++;
                items.add(new ImportItem(index + 1, "", "", "skipped", "empty code"));
                continue;
            }

            String note = "imported";
            if (record.size() > 1 && !record.get(1).trim().isEmpty()) {
                note = record.get(1).trim();
            }
            String validationMessage = validationMessage(code);
            if (validationMessage != null) {
                counts.invalid++;
                items.add(new ImportItem(index + 1, code, note, "invalid", validationMessage));
                continue;
            }

            String normalized = CardCodes.normalize(code);
            String formatted = CardCodes.format(normalized);
            if (seen.contains(normalized)) {
                counts.duplicates++;
                items.add(new ImportItem(
                    index + 1, formatted, note, "duplicate", "card already exists"));
                continue;
            }
            boolean alreadyExists;
            try {
                alreadyExists = exists(formatted);
            } catch (DataAccessException ex) {
                counts.invalid++;
                items.add(new ImportItem(
                    index + 1, formatted, note, "error", "database error"));
                continue;
            }
            if (alreadyExists) {
                counts.duplicates++;
                items.add(new ImportItem(
                    index + 1, formatted, note, "duplicate", "card already exists"));
                continue;
            }
            seen.add(normalized);
            counts.validRows++;

            if (dryRun) {
                items.add(new ImportItem(index + 1, formatted, note, "valid", ""));
                continue;
            }

            try {
                writer.createWithCode(formatted, duration, note, maxSessions, "");
            } catch (DataAccessException ex) {
                counts.validRows--;
                counts.invalid++;
                items.add(new ImportItem(
                    index + 1, formatted, note, "invalid", "failed to create card"));
                continue;
            }
            appendAudit(
                "card_imported",
                formatted,
                "duration=" + goDuration(duration) + " max_sessions=" + maxSessions
            );
            counts.imported++;
            items.add(new ImportItem(index + 1, formatted, note, "imported", ""));
        }

        if (!dryRun && counts.imported > 0) {
            appendAudit(
                "cards_import_completed",
                "",
                "imported=" + counts.imported
                    + " duplicates=" + counts.duplicates
                    + " invalid=" + counts.invalid
                    + " skipped=" + counts.skipped
            );
        }
        return counts.toReport(items);
    }

    private boolean exists(String code) {
        Integer count = jdbc.queryForObject(
            "SELECT COUNT(*) FROM cards WHERE code = ?", Integer.class, code);
        return count != null && count > 0;
    }

    private void appendAudit(String action, String card, String detail) {
        jdbc.update(
            "INSERT INTO audit_log (time, action, card, detail) VALUES (?, ?, ?, ?)",
            TimestampWriter.format(Instant.now()), action, card, detail);
    }

    static String spreadsheetSafe(String value) {
        String trimmed = value == null ? "" : value.trim();
        if (!trimmed.isEmpty() && "=+-@".indexOf(trimmed.charAt(0)) >= 0) {
            return "'" + trimmed;
        }
        return trimmed;
    }

    static List<List<String>> parseCsv(String source) {
        String input = source == null ? "" : source.strip();
        if (input.startsWith("\uFEFF")) {
            input = input.substring(1);
        }
        List<List<String>> records = new ArrayList<>();
        List<String> record = new ArrayList<>();
        StringBuilder field = new StringBuilder();
        boolean quoted = false;
        boolean justClosedQuote = false;
        boolean sawContent = false;

        for (int i = 0; i < input.length(); i++) {
            char ch = input.charAt(i);
            if (quoted) {
                if (ch == '"') {
                    if (i + 1 < input.length() && input.charAt(i + 1) == '"') {
                        field.append('"');
                        i++;
                    } else {
                        quoted = false;
                        justClosedQuote = true;
                    }
                } else if (ch == '\r'
                    && i + 1 < input.length()
                    && input.charAt(i + 1) == '\n') {
                    field.append('\n');
                    i++;
                } else {
                    field.append(ch);
                }
                sawContent = true;
                continue;
            }
            if (justClosedQuote && ch != ',' && ch != '\r' && ch != '\n') {
                throw new IllegalArgumentException("invalid CSV data");
            }
            if (ch == '"' && field.length() == 0 && !justClosedQuote) {
                quoted = true;
                sawContent = true;
            } else if (ch == '"') {
                throw new IllegalArgumentException("invalid CSV data");
            } else if (ch == ',') {
                record.add(field.toString());
                field.setLength(0);
                sawContent = true;
                justClosedQuote = false;
            } else if (ch == '\r' || ch == '\n') {
                if (ch == '\r' && i + 1 < input.length() && input.charAt(i + 1) == '\n') {
                    i++;
                }
                if (sawContent || field.length() > 0 || !record.isEmpty()) {
                    record.add(field.toString());
                    records.add(record);
                }
                record = new ArrayList<>();
                field.setLength(0);
                sawContent = false;
                justClosedQuote = false;
            } else {
                field.append(ch);
                sawContent = true;
            }
        }
        if (quoted) {
            throw new IllegalArgumentException("invalid CSV data");
        }
        if (sawContent || field.length() > 0 || !record.isEmpty()) {
            record.add(field.toString());
            records.add(record);
        }
        return records;
    }

    private static void appendRecord(StringBuilder csv, List<String> cells) {
        for (int i = 0; i < cells.size(); i++) {
            if (i > 0) {
                csv.append(',');
            }
            String cell = cells.get(i);
            boolean quote = cell.indexOf(',') >= 0
                || cell.indexOf('"') >= 0
                || cell.indexOf('\r') >= 0
                || cell.indexOf('\n') >= 0;
            if (quote) {
                csv.append('"').append(cell.replace("\"", "\"\"")).append('"');
            } else {
                csv.append(cell);
            }
        }
        csv.append("\r\n");
    }

    private static String validationMessage(String code) {
        String normalized = CardCodes.normalize(code);
        if (normalized.length() != CardCodes.NORMALIZED_LENGTH) {
            return "card code must contain 18 base32 characters";
        }
        for (int i = 0; i < normalized.length(); i++) {
            if (CardCodes.ALPHABET.indexOf(normalized.charAt(i)) < 0) {
                return "card code contains invalid character '" + normalized.charAt(i) + "'";
            }
        }
        return null;
    }

    private static String formatTime(Instant instant) {
        return instant == null ? "" : CSV_TIME.format(instant);
    }

    private static String goDuration(Duration duration) {
        long seconds = duration.getSeconds();
        long hours = seconds / 3600;
        long minutes = (seconds % 3600) / 60;
        long remainingSeconds = seconds % 60;
        return hours + "h" + minutes + "m" + remainingSeconds + "s";
    }

    private static final class MutableCounts {
        private int totalRows;
        private int validRows;
        private int imported;
        private int duplicates;
        private int invalid;
        private int skipped;

        ImportReport toReport(List<ImportItem> items) {
            return new ImportReport(
                totalRows, validRows, imported, duplicates, invalid, skipped, items);
        }
    }

    public record ImportItem(
        int row,
        @JsonInclude(JsonInclude.Include.NON_EMPTY) String code,
        @JsonInclude(JsonInclude.Include.NON_EMPTY) String note,
        String status,
        @JsonInclude(JsonInclude.Include.NON_EMPTY) String message
    ) {
    }

    public record ImportReport(
        @JsonProperty("total_rows") int totalRows,
        @JsonProperty("valid_rows") int validRows,
        int imported,
        int duplicates,
        int invalid,
        int skipped,
        List<ImportItem> items
    ) {
    }
}
