package com.lingqiao.server.exam;

public record ExamFilter(
    String cardCode,
    String examId,
    String question,
    int page,
    int pageSize
) {
    public ExamFilter {
        page = page < 1 ? 1 : page;
        pageSize = pageSize < 1 || pageSize > 200 ? 50 : pageSize;
    }
}
