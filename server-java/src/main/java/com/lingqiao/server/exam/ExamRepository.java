package com.lingqiao.server.exam;

import java.util.List;

public interface ExamRepository {
    default void insert(ExamRecord record) {
        throw new UnsupportedOperationException("Java exam writes are disabled during shadow-read migration");
    }

    ExamStats stats();

    default ExamPage list(ExamFilter filter) {
        return new ExamPage(List.of(), 0);
    }

    default List<ExamRecord> listAll(ExamFilter filter) {
        return List.of();
    }
}
