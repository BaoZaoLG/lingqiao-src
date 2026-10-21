package com.lingqiao.server.exam;

import java.util.List;

public record ExamPage(List<ExamRecord> records, long total) {
}
