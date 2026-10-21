package com.lingqiao.server.agentportal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.lingqiao.server.common.errors.SidecarException;
import javax.sql.DataSource;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;

class AgentRegistrationServiceTest {
    private final JdbcTemplate jdbc = mock(JdbcTemplate.class);
    private AgentRegistrationService service;

    @BeforeEach
    void setUp() {
        DataSource dataSource = mock(DataSource.class);
        when(jdbc.getDataSource()).thenReturn(dataSource);
        service = new AgentRegistrationService(jdbc);
    }

    @Test
    void rejectsShortAndLongUsernames() {
        assertThatThrownBy(() -> AgentRegistrationService.validateUsername("ab"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("3-20");
        assertThatThrownBy(() -> AgentRegistrationService.validateUsername("a".repeat(21)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("3-20");
    }

    @Test
    void rejectsUsernamesWithDisallowedCharacters() {
        assertThatThrownBy(() -> AgentRegistrationService.validateUsername("bad-name"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("字母、数字和下划线");
        assertThatThrownBy(() -> AgentRegistrationService.validateUsername("用户名abc"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("字母、数字和下划线");
    }

    @Test
    void acceptsUnderscoreAndDigits() {
        AgentRegistrationService.validateUsername("agent_01");
    }

    @Test
    void rejectsPasswordOutsideBcryptBounds() {
        assertThatThrownBy(() -> AgentRegistrationService.validatePassword("12345"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
        assertThatThrownBy(() -> AgentRegistrationService.validatePassword("x".repeat(73)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
        AgentRegistrationService.validatePassword("密码");
    }

    @Test
    void rejectsDuplicateUsernameBeforeTouchingInvite() {
        when(jdbc.queryForObject(anyString(), eq(Integer.class), any(Object[].class))).thenReturn(1);

        assertThatThrownBy(() -> service.register("alice", "secret123", "INV-1"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("用户名已存在");

        verify(jdbc, never()).update(anyString(), any(Object[].class));
    }

    @Test
    void trimsUsernameBeforeValidation() {
        when(jdbc.queryForObject(anyString(), eq(Integer.class), any(Object[].class))).thenReturn(1);

        assertThatThrownBy(() -> service.register("  alice  ", "secret123", "INV-1"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("用户名已存在");
    }

    @Test
    void shortIdIsTwelveHexCharacters() {
        String id = service.shortId();
        assertThat(id).hasSize(12).matches("[0-9a-f]{12}");
    }

    @Test
    void shortIdsDifferBetweenCalls() {
        assertThat(service.shortId()).isNotEqualTo(service.shortId());
    }
}
