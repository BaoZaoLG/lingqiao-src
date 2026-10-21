package com.lingqiao.server.agents;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;

class AgentsCsvTest {
    @Test
    void exportMatchesGoHeaderCrlfAndFormulaProtectionWithoutBom() {
        Agent agent = new Agent(
            "=id", "@user", "", Instant.parse("2025-01-01T00:00:00Z"),
            false, 3, 2, 1, 1, Instant.parse("2025-01-02T03:04:05Z"));

        String csv = new String(AgentsCsv.export(List.of(agent)), StandardCharsets.UTF_8);

        assertThat(csv).startsWith("agent_id,username,disabled,total_cards,");
        assertThat(csv).doesNotStartWith("\uFEFF");
        assertThat(csv).contains("'=id,'@user,false,3,2,1,1,");
        assertThat(csv).contains("2025-01-02T03:04:05Z,2025-01-01T00:00:00Z");
        assertThat(csv).endsWith("\r\n");
    }
}
