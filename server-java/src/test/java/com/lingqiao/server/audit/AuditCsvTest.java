package com.lingqiao.server.audit;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;

class AuditCsvTest {
    @Test
    void exportMatchesGoCrLfFormatAndHardensSpreadsheetCells() {
        AuditEvent event = new AuditEvent(
            Instant.parse("2026-07-01T02:03:04Z"), "=action", null, null, null, null,
            null, null, null, null, "CARD", "agent", "machine", "127.0.0.1",
            "line1,\n\"line2\""
        );

        byte[] bytes = AuditCsv.export(List.of(event));
        String csv = new String(bytes, StandardCharsets.UTF_8);

        assertThat(bytes).startsWith((byte) 0xef, (byte) 0xbb, (byte) 0xbf);
        assertThat(csv).startsWith("\ufefftime,action,card,machine,agent,ip,detail\r\n");
        assertThat(csv).contains("2026-07-01 02:03:04,'=action,CARD,machine,agent,127.0.0.1,");
        assertThat(csv).endsWith("\"line1,\n\"\"line2\"\"\"\r\n");
    }
}
