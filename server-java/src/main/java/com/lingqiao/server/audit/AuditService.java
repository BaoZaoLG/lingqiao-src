package com.lingqiao.server.audit;

import java.util.Comparator;
import java.util.List;

public class AuditService {
    private final AuditRepository repository;

    public AuditService(AuditRepository repository) {
        this.repository = repository;
    }

    public AuditPage query(AuditFilter filter) {
        List<AuditEvent> all = sorted(repository.list(filter, 10000));
        int start = Math.min((filter.page() - 1) * filter.perPage(), all.size());
        int end = Math.min(start + filter.perPage(), all.size());
        return new AuditPage(all.subList(start, end), all.size(), filter.page(), filter.perPage());
    }

    public List<AuditEvent> queryAll(AuditFilter filter) {
        return sorted(repository.list(filter, 10000));
    }

    private static List<AuditEvent> sorted(List<AuditEvent> events) {
        return events.stream()
            .sorted(Comparator.comparing(AuditEvent::time).reversed())
            .toList();
    }
}
