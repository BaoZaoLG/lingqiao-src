package com.lingqiao.server.audit;

import java.util.List;

public record AuditPage(List<AuditEvent> entries, long total, int page, int perPage) {
}
