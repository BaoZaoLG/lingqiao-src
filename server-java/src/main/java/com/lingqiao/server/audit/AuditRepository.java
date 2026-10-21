package com.lingqiao.server.audit;

import java.util.List;

public interface AuditRepository {
    List<AuditEvent> list(AuditFilter filter, int limit);
}
