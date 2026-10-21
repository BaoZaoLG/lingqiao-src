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
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.bcrypt.BCrypt;

class AgentPortalAuthServiceTest {
    private final JdbcTemplate jdbc = mock(JdbcTemplate.class);
    private final AgentPortalAuthService service =
        new AgentPortalAuthService(jdbc, Duration.ofHours(12));

    @Test
    void loginIssuesPrefixedTokenAndResolvesSession() {
        stubAgent("agent-1", "alice", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        var result = service.login("alice", "secret123", "10.0.0.1");

        assertThat(result.token()).startsWith("agent_");
        assertThat(result.agentId()).isEqualTo("agent-1");
        assertThat(service.requireSession(result.token())).isEqualTo("agent-1");
    }

    @Test
    void loginRejectsDisabledAgentBeforePasswordCheck() {
        stubAgent("agent-2", "bob", BCrypt.hashpw("secret123", BCrypt.gensalt()), 1);

        assertThatThrownBy(() -> service.login("bob", "secret123", "10.0.0.2"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("账号已被禁用");
    }

    @Test
    void loginRejectsWrongPassword() {
        stubAgent("agent-3", "carol", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        assertThatThrownBy(() -> service.login("carol", "wrong", "10.0.0.3"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("用户名或密码错误");
    }

    @Test
    void loginRateLimitsAfterFiveFailures() {
        stubAgent("agent-4", "dave", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        for (int i = 0; i < AgentPortalAuthService.LOGIN_ATTEMPT_LIMIT; i++) {
            assertThatThrownBy(() -> service.login("dave", "wrong", "10.0.0.4"))
                .isInstanceOf(SidecarException.class);
        }
        assertThatThrownBy(() -> service.login("dave", "secret123", "10.0.0.4"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("登录尝试过于频繁");
    }

    @Test
    void loginUpgradesLegacySha256Hash() {
        String legacy = "c8b6a5a2d9b8e0a1f2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f6071829";
        stubAgent("agent-5", "erin", legacy, 0);

        assertThatThrownBy(() -> service.login("erin", "secret123", "10.0.0.5"))
            .isInstanceOf(SidecarException.class);
        verify(jdbc, never()).update(anyString(), eq("secret123"), anyString());
    }

    @Test
    void requireSessionRejectsUnknownAndUnprefixedTokens() {
        assertThatThrownBy(() -> service.requireSession("nope"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("unauthorized");
        assertThatThrownBy(() -> service.requireSession("agent_deadbeef"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("unauthorized");
    }

    @Test
    void logoutInvalidatesSession() {
        stubAgent("agent-6", "frank", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);
        var result = service.login("frank", "secret123", "10.0.0.6");

        service.logout(result.token());

        assertThatThrownBy(() -> service.requireSession(result.token()))
            .isInstanceOf(SidecarException.class);
    }

    @Test
    void requireSessionRejectsAgentDisabledAfterLogin() {
        stubAgent("agent-7", "gina", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);
        var result = service.login("gina", "secret123", "10.0.0.7");

        stubAgent("agent-7", "gina", BCrypt.hashpw("secret123", BCrypt.gensalt()), 1);

        assertThatThrownBy(() -> service.requireSession(result.token()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("unauthorized");
    }

    @Test
    void changePasswordRejectsShortAndOverlongValues() {
        stubAgent("agent-8", "hank", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        assertThatThrownBy(() -> service.changePassword("agent-8", "secret123", "12345"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
        assertThatThrownBy(() -> service.changePassword("agent-8", "secret123", "x".repeat(73)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
    }

    @Test
    void changePasswordRequiresCorrectOldPassword() {
        stubAgent("agent-9", "iris", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        assertThatThrownBy(() -> service.changePassword("agent-9", "wrong", "newsecret"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("旧密码错误");
    }

    @Test
    void passwordChangeRateLimitsAfterThreeAttemptsPerIp() {
        stubAgent("agent-9b", "iris2", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        for (int i = 0; i < AgentPortalAuthService.PASSWORD_ATTEMPT_LIMIT; i++) {
            service.changePasswordFromIp(
                "agent-9b", "secret123", "newsecret", "10.0.0.9");
        }
        assertThatThrownBy(() -> service.changePasswordFromIp(
            "agent-9b", "secret123", "newsecret", "10.0.0.9"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("密码修改过于频繁");
    }

    @Test
    void passwordLengthUsesUtf8BytesLikeGo() {
        stubAgent("agent-9c", "iris3", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        service.changePassword("agent-9c", "secret123", "密码");

        verify(jdbc).update(anyString(), anyString(), eq("agent-9c"));
    }

    @Test
    void statsReadsCardCountsForAgent() {
        when(jdbc.queryForObject(anyString(), eq(Integer.class), any(Object[].class)))
            .thenReturn(7);

        var stats = service.stats("agent-10");

        assertThat(stats.totalCards()).isEqualTo(7);
        assertThat(stats.activeCards()).isEqualTo(7);
        assertThat(stats.expiredCards()).isEqualTo(7);
    }

    @Test
    void registrationRateLimitAllowsThreeAttemptsPerIp() {
        assertThat(service.allowRegistration("10.0.0.11")).isTrue();
        assertThat(service.allowRegistration("10.0.0.11")).isTrue();
        assertThat(service.allowRegistration("10.0.0.11")).isTrue();
        assertThat(service.allowRegistration("10.0.0.11")).isFalse();
    }

    @Test
    void defaultSessionTtlMatchesGoFourHourFallback() {
        AgentPortalAuthService defaultTtlService = new AgentPortalAuthService(jdbc, null);
        stubAgent("agent-12", "jane", BCrypt.hashpw("secret123", BCrypt.gensalt()), 0);

        var result = defaultTtlService.login("jane", "secret123", "10.0.0.12");

        assertThat(result.maxAgeSeconds()).isEqualTo(4 * 60 * 60);
    }

    @Test
    void profileMatchesGoAgentInfoFields() {
        Map<String, Object> row = new LinkedHashMap<>();
        row.put("id", "agent-13");
        row.put("username", "kate");
        row.put("password", BCrypt.hashpw("secret123", BCrypt.gensalt()));
        row.put("prefix", "KT");
        row.put("created_at", "2026-07-30T12:00:00Z");
        row.put("disabled", 0);
        when(jdbc.queryForList(anyString(), any(Object[].class))).thenReturn(List.of(row));

        var profile = service.profile("agent-13");

        assertThat(profile.id()).isEqualTo("agent-13");
        assertThat(profile.username()).isEqualTo("kate");
        assertThat(profile.prefix()).isEqualTo("KT");
        assertThat(profile.createdAt()).isEqualTo(Instant.parse("2026-07-30T12:00:00Z"));
    }

    private void stubAgent(String id, String username, String password, int disabled) {
        Map<String, Object> row = new LinkedHashMap<>();
        row.put("id", id);
        row.put("username", username);
        row.put("password", password);
        row.put("disabled", disabled);
        when(jdbc.queryForList(anyString(), any(Object[].class))).thenReturn(List.of(row));
    }
}
