package com.lingqiao.server.sessions;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

class SessionsServiceTest {
    @Test
    @SuppressWarnings("unchecked")
    void listReturnsPagedResult() {
        JdbcTemplate jdbc = mock(JdbcTemplate.class);
        Session session = new Session("tok", "C1", "m1", "fp", "1.0.0",
            Instant.parse("2026-07-01T00:00:00Z"), Instant.parse("2026-07-01T01:00:00Z"),
            Instant.parse("2026-07-02T00:00:00Z"), "127.0.0.1");
        when(jdbc.queryForObject(anyString(), eq(Long.class), any(Object[].class))).thenReturn(1L);
        when(jdbc.query(anyString(), any(RowMapper.class), any(Object[].class))).thenReturn(List.of(session));

        SessionsService service = new SessionsService(jdbc);
        SessionListResult result = service.list(new SessionListFilter("C1", "m1", true, 1, 20));

        assertThat(result.total()).isEqualTo(1);
        assertThat(result.items()).containsExactly(session);
        assertThat(result.page()).isEqualTo(1);
        assertThat(result.perPage()).isEqualTo(20);
    }
}
