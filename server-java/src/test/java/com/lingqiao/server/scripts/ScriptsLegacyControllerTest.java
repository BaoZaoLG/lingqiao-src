package com.lingqiao.server.scripts;

import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.lingqiao.server.common.security.AdminAuthService;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(ScriptsLegacyController.class)
@TestPropertySource(properties = "lingqiao.sidecar.scripts-enabled=true")
class ScriptsLegacyControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private ScriptsService reads;

    @MockBean
    private ScriptsWriteService writes;

    @MockBean
    private AdminAuthService auth;

    @Test
    void legacyListIncludesGoTopLevelActiveFieldsAndSnakeCaseTimes() throws Exception {
        ScriptModule active = module("script-1", "active");
        when(reads.list()).thenReturn(new ScriptListResult(
            List.of(active), active.id(), active));

        mvc.perform(get("/java/admin/api/script"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.active_id").value("script-1"))
            .andExpect(jsonPath("$.id").value("script-1"))
            .andExpect(jsonPath("$.updated_at").value("2026-07-01T00:00:00Z"))
            .andExpect(jsonPath("$.script.created_at").value("2026-07-01T00:00:00Z"))
            .andExpect(jsonPath("$.script.createdAt").doesNotExist())
            .andExpect(jsonPath("$.script.activated_at").value("2026-07-01T00:00:00Z"));
    }

    @Test
    void emptyLegacyActionPublishesAndReturnsFlatGoResponse() throws Exception {
        when(writes.publish(anyString(), anyString(), anyString()))
            .thenReturn(module("script-2", "active"));

        mvc.perform(post("/java/admin/api/script")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {"version":"v2","content":"code","note":"note"}
                    """))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.id").value("script-2"))
            .andExpect(jsonPath("$.version").value("v2"));
    }

    private static ScriptModule module(String id, String status) {
        Instant time = Instant.parse("2026-07-01T00:00:00Z");
        return new ScriptModule(
            id, "v2", "sha", "code", 4, "", status, time, time, time);
    }
}
