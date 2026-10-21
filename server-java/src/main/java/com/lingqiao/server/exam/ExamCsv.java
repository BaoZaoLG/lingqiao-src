package com.lingqiao.server.exam;

import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.Locale;

final class ExamCsv {
    private static final byte[] BOM = {(byte) 0xef, (byte) 0xbb, (byte) 0xbf};
    private static final DateTimeFormatter TIME =
        DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss").withZone(ZoneOffset.UTC);

    private ExamCsv() {
    }

    static byte[] export(List<ExamRecord> records) {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        output.writeBytes(BOM);
        writeRow(output, List.of(
            "ID", "卡号", "机器ID", "考试ID", "课程ID", "班级ID",
            "题目ID", "题目文本", "题型", "选项JSON", "提交答案", "正确答案",
            "模型", "置信度", "答题耗时(ms)", "填充成功", "时间"
        ));
        for (ExamRecord record : records) {
            writeRow(output, List.of(
                Long.toString(record.id()),
                safe(record.cardCode()),
                safe(record.machineId()),
                safe(record.examId()),
                safe(record.courseId()),
                safe(record.classId()),
                safe(record.questionId()),
                safe(record.questionText()),
                safe(record.questionType()),
                safe(record.optionsJson()),
                safe(record.providedAnswer()),
                safe(record.correctAnswer()),
                safe(record.modelName()),
                String.format(Locale.ROOT, "%.4f", record.confidence()),
                Long.toString(record.answerTimeMs()),
                record.fillOk() ? "是" : "否",
                TIME.format(record.createdAt())
            ));
        }
        return output.toByteArray();
    }

    private static String safe(String value) {
        if (value == null || value.isEmpty()) {
            return value == null ? "" : value;
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
