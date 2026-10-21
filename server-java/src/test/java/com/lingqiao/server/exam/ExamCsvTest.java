package com.lingqiao.server.exam;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;

class ExamCsvTest {
    @Test
    void exportMatchesGoFormatAndHardensSpreadsheetCells() {
        ExamRecord record = new ExamRecord(
            7, "=CARD", "M1", "E1", "course", "class", "q1", "line1,\n\"line2\"",
            "single", "[]", "+A", "A", "model", 0.9, 120, true,
            Instant.parse("2026-07-01T02:03:04Z")
        );

        byte[] bytes = ExamCsv.export(List.of(record));
        String csv = new String(bytes, StandardCharsets.UTF_8);

        assertThat(bytes).startsWith((byte) 0xef, (byte) 0xbb, (byte) 0xbf);
        assertThat(csv).contains("ID,卡号,机器ID,考试ID");
        assertThat(csv).contains("7,'=CARD,M1,E1");
        assertThat(csv).contains("\"line1,\n\"\"line2\"\"\"");
        assertThat(csv).contains(",'+A,A,model,0.9000,120,是,2026-07-01 02:03:04\n");
        assertThat(csv).doesNotContain("\r\n");
    }
}
