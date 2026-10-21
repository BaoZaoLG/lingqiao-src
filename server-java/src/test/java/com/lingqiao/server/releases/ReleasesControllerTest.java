package com.lingqiao.server.releases;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.lingqiao.server.common.security.AdminAuthService;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(ReleasesController.class)
@TestPropertySource(properties = "lingqiao.sidecar.releases-enabled=true")
class ReleasesControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private ReleasesService service;

    @MockBean
    private AdminAuthService adminAuthService;

    @MockBean
    private ManifestSeedLoader seedLoader;

    @Test
    void eventsUsesGoLimitAndSnakeCaseFields() throws Exception {
        var event = new ReleaseAdminService.ReleaseEventView(
            1, "rel-1", "1.0.0", "M1", "C1", "", "offered", "", "",
            Instant.parse("2026-07-01T00:00:00Z"));
        when(service.listEvents("rel-1", 100)).thenReturn(List.of(event));
        when(service.metrics("rel-1")).thenReturn(ReleaseMetrics.empty("rel-1"));

        mvc.perform(get("/java/admin/api/releases/rel-1/events"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.events[0].release_id").value("rel-1"))
            .andExpect(jsonPath("$.events[0].machine_id").value("M1"))
            .andExpect(jsonPath("$.events[0].agent_id").doesNotExist())
            .andExpect(jsonPath("$.metrics.release_id").value("rel-1"));

        verify(service).listEvents("rel-1", 100);
        verify(service).metrics("rel-1");
    }
}
