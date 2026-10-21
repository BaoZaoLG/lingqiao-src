package com.lingqiao.server.audit;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.OffsetDateTime;
import org.junit.jupiter.api.Test;

class JdbcAuditRepositoryTimeTest {
    @Test
    void parsesLegacyGoTimeStringWithZoneAndMonotonicSuffix() {
        OffsetDateTime parsed = JdbcAuditRepository.parseStoredTime(
            "2026-07-30 18:57:26.31336828 +0800 CST m=+2256639.663652206");

        assertThat(parsed).isEqualTo(
            OffsetDateTime.parse("2026-07-30T18:57:26.31336828+08:00"));
    }

    @Test
    void preservesRfc3339Offsets() {
        assertThat(JdbcAuditRepository.parseStoredTime(
            "2026-07-30T18:57:26.31336828+08:00"))
            .isEqualTo(OffsetDateTime.parse("2026-07-30T18:57:26.31336828+08:00"));
    }
}
