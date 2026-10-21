package com.lingqiao.server.audit;

import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.time.format.DateTimeFormatter;
import java.util.List;

final class AuditCsv {
    private static final byte[] BOM = {(byte) 0xef, (byte) 0xbb, (byte) 0xbf};
    private static final DateTimeFormatter TIME =
        DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");

    private AuditCsv() {
    }

    static byte[] export(List<AuditEvent> entries) {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        output.writeBytes(BOM);
        writeRow(output, List.of("time", "action", "card", "machine", "agent", "ip", "detail"));
        for (AuditEvent event : entries) {
            writeRow(output, List.of(
                TIME.format(event.time()),
                safe(event.action()),
                safe(event.card()),
                safe(event.machine()),
                safe(event.agentId()),
                safe(event.ip()),
                safe(event.detail())
            ));
        }
        return output.toByteArray();
    }

    private static String safe(String value) {
        if (value == null || value.isEmpty()) {
            return "";
        }
        return switch (value.charAt(0)) {
            case '=', '+', '-', '@' -> "'" + value;
            default -> value;
        };
    }

    private static void writeRow(ByteArrayOutputStream output, List<String> cells) {
        for (int i = 0; i < cells.size(); i++) {
            if (i > 0) {
                output.write(',');
            }
            output.writeBytes(escape(cells.get(i)).getBytes(StandardCharsets.UTF_8));
        }
        output.write('\r');
        output.write('\n');
    }

    private static String escape(String value) {
        if (value.indexOf(',') < 0 && value.indexOf('"') < 0
            && value.indexOf('\n') < 0 && value.indexOf('\r') < 0) {
            return value;
        }
        return "\"" + value.replace("\"", "\"\"") + "\"";
    }
}
