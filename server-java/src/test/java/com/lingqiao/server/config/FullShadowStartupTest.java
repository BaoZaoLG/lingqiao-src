package com.lingqiao.server.config;

import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.agentportal.AgentPortalController;
import com.lingqiao.server.client.ClientScriptController;
import com.lingqiao.server.scripts.ScriptsReadController;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.TestPropertySource;

@SpringBootTest
@TestPropertySource(properties = {
    "lingqiao.sidecar.exam-enabled=true",
    "lingqiao.sidecar.audit-enabled=true",
    "lingqiao.sidecar.scripts-enabled=true",
    "lingqiao.sidecar.cards-enabled=true",
    "lingqiao.sidecar.sessions-enabled=true",
    "lingqiao.sidecar.agents-enabled=true",
    "lingqiao.sidecar.chat-enabled=true",
    "lingqiao.sidecar.blacklist-enabled=true",
    "lingqiao.sidecar.announcement-enabled=true",
    "lingqiao.sidecar.releases-enabled=true",
    "lingqiao.sidecar.payloads-enabled=true",
    "lingqiao.sidecar.admin-auth-enabled=true",
    "lingqiao.sidecar.admin-overview-enabled=true",
    "lingqiao.sidecar.agent-portal-enabled=true",
    "lingqiao.sidecar.agent-portal-write-enabled=false",
    "lingqiao.sidecar.client-download-enabled=true",
    "lingqiao.sidecar.client-update-enabled=true",
    "lingqiao.sidecar.client-announcement-enabled=true",
    "lingqiao.auth.data-dir=${java.io.tmpdir}/lingqiao-full-shadow-startup",
    "lingqiao.write.db-url=jdbc:sqlite:file:memdb-full-shadow-start?mode=memory&cache=shared"
})
class FullShadowStartupTest {
    @Autowired
    private ScriptsReadController scriptsReadController;

    @Autowired
    private ClientScriptController clientScriptController;

    @Autowired
    private AgentPortalController agentPortalController;

    @Test
    void allReadAndCompatibilityControllersStartTogether() {
        assertThat(scriptsReadController).isNotNull();
        assertThat(clientScriptController).isNotNull();
        assertThat(agentPortalController).isNotNull();
    }
}
