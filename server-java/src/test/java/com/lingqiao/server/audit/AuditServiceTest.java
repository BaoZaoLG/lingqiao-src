package com.lingqiao.server.audit;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;

class AuditServiceTest {
    @Test
    void queryPaginatesReadOnlyResults() {
        AuditRepository repository = (filter, limit) -> List.of(
            new AuditEvent(Instant.parse("2026-07-14T00:00:01Z"), "a1", null, null, null, null, null, null, null, null, null, null, null, null, "one"),
            new AuditEvent(Instant.parse("2026-07-14T00:00:02Z"), "a2", null, null, null, null, null, null, null, null, null, null, null, null, "two")
        );
        AuditService service = new AuditService(repository);

        AuditPage page = service.query(new AuditFilter(null, null, null, null, null, null, null, null, null, null, null, null, 1, 1));

        assertThat(page.total()).isEqualTo(2);
        assertThat(page.entries()).hasSize(1);
        assertThat(page.entries().get(0).action()).isEqualTo("a2");
        assertThat(page.page()).isEqualTo(1);
        assertThat(page.perPage()).isEqualTo(1);
    }

    @Test
    void queryAllReturnsTheCompleteExportWindow() {
        final int[] requestedLimit = {0};
        AuditRepository repository = (filter, limit) -> {
            requestedLimit[0] = limit;
            return List.of();
        };

        new AuditService(repository).queryAll(
            new AuditFilter(null, null, null, null, null, null, null, null, null,
                null, null, null, 1, 50));

        assertThat(requestedLimit[0]).isEqualTo(10000);
    }
}
