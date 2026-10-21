package com.lingqiao.server.agentportal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.bcrypt.BCrypt;

class AgentSessionStoreTest {
    @TempDir
    Path tempDir;

    @Test
    void loginSurvivesServiceRestartUsingGoCompatibleFiles() throws Exception {
        JdbcTemplate jdbc = enabledAgentJdbc("agent-1", "alice");
        AgentPortalAuthService first = new AgentPortalAuthService(
            jdbc, Duration.ofHours(4), new AgentSessionStore(tempDir));

        var login = first.login("alice", "secret123", "127.0.0.1");
        AgentPortalAuthService restarted = new AgentPortalAuthService(
            jdbc, Duration.ofHours(4), new AgentSessionStore(tempDir));

        assertThat(restarted.requireSession(login.token())).isEqualTo("agent-1");
        String sessionsJson = Files.readString(tempDir.resolve("agent_sessions.json"));
        String tokenMapJson = Files.readString(tempDir.resolve("agent_token_map.json"));
        assertThat(sessionsJson).contains("T").contains("Z");
        assertThat(sessionsJson).doesNotContain(login.token());
        assertThat(tokenMapJson).doesNotContain(login.token());
        assertThat(tokenMapJson).contains(AgentPortalAuthService.hashToken(login.token()));
    }

    @Test
    void readsFilesWrittenInGoRfc3339Shape() throws Exception {
        String token = "agent_" + "ab".repeat(32);
        String hash = AgentPortalAuthService.hashToken(token);
        Files.writeString(
            tempDir.resolve("agent_sessions.json"),
            "{\"" + hash + "\":\"2099-01-02T03:04:05Z\"}"
        );
        Files.writeString(
            tempDir.resolve("agent_token_map.json"),
            "{\"" + hash + "\":\"agent-go\"}"
        );
        AgentPortalAuthService service = new AgentPortalAuthService(
            enabledAgentJdbc("agent-go", "go-agent"),
            Duration.ofHours(4),
            new AgentSessionStore(tempDir)
        );

        assertThat(service.requireSession(token)).isEqualTo("agent-go");
    }

    @Test
    void logoutAndExpiryArePersistedAcrossRestart() throws Exception {
        JdbcTemplate jdbc = enabledAgentJdbc("agent-2", "bob");
        AgentPortalAuthService service = new AgentPortalAuthService(
            jdbc, Duration.ofHours(4), new AgentSessionStore(tempDir));
        var login = service.login("bob", "secret123", "127.0.0.2");

        service.logout(login.token());

        AgentPortalAuthService restarted = new AgentPortalAuthService(
            jdbc, Duration.ofHours(4), new AgentSessionStore(tempDir));
        assertThatThrownBy(() -> restarted.requireSession(login.token()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("unauthorized");

        String expiredToken = "agent_" + "cd".repeat(32);
        String expiredHash = AgentPortalAuthService.hashToken(expiredToken);
        Files.writeString(
            tempDir.resolve("agent_sessions.json"),
            "{\"" + expiredHash + "\":\"" + Instant.now().minusSeconds(1) + "\"}"
        );
        Files.writeString(
            tempDir.resolve("agent_token_map.json"),
            "{\"" + expiredHash + "\":\"agent-2\"}"
        );

        new AgentSessionStore(tempDir);

        assertThat(Files.readString(tempDir.resolve("agent_sessions.json")))
            .doesNotContain(expiredHash);
        assertThat(Files.readString(tempDir.resolve("agent_token_map.json")))
            .doesNotContain(expiredHash);
    }

    @Test
    void readOnlyStoreDoesNotRewriteExpiredSessionFiles() throws Exception {
        String expiredToken = "agent_" + "ef".repeat(32);
        String expiredHash = AgentPortalAuthService.hashToken(expiredToken);
        String sessions = "{\"" + expiredHash + "\":\"" + Instant.now().minusSeconds(1) + "\"}";
        String tokens = "{\"" + expiredHash + "\":\"agent-3\"}";
        Files.writeString(tempDir.resolve("agent_sessions.json"), sessions);
        Files.writeString(tempDir.resolve("agent_token_map.json"), tokens);

        AgentSessionStore store = AgentSessionStore.readOnly(tempDir);

        assertThat(store.find(expiredHash, Instant.now())).isNull();
        assertThat(Files.readString(tempDir.resolve("agent_sessions.json"))).isEqualTo(sessions);
        assertThat(Files.readString(tempDir.resolve("agent_token_map.json"))).isEqualTo(tokens);
        assertThatThrownBy(() -> store.put(expiredHash, "agent-3", Instant.now().plusSeconds(60)))
            .isInstanceOf(IllegalStateException.class)
            .hasMessageContaining("read-only");
    }

    private static JdbcTemplate enabledAgentJdbc(String id, String username) {
        JdbcTemplate jdbc = mock(JdbcTemplate.class);
        Map<String, Object> row = new LinkedHashMap<>();
        row.put("id", id);
        row.put("username", username);
        row.put("password", BCrypt.hashpw("secret123", BCrypt.gensalt()));
        row.put("disabled", 0);
        when(jdbc.queryForList(anyString(), any(Object[].class))).thenReturn(List.of(row));
        return jdbc;
    }
}
