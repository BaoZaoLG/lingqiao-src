package com.lingqiao.server.audit;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.lingqiao.server.common.security.AdminAuthService;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.dao.DataAccessResourceFailureException;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(AuditReadController.class)
@TestPropertySource(properties = "lingqiao.sidecar.audit-enabled=true")
class AuditReadControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private AuditService service;

    @MockBean
    private AdminAuthService adminAuthService;

    @Test
    void csvVariantUsesAllFiltersAndSkipsPagination() throws Exception {
        AuditFilter filter = new AuditFilter(
            "act", "admin", "A1", "card", "C1", "CARD", "AG1", "M1", "IP1",
            Instant.parse("2026-07-01T00:00:00Z"), null, "needle", 1, 50
        );
        when(service.queryAll(filter)).thenReturn(List.of());

        mvc.perform(get("/java/admin/api/audit")
                .param("export", "csv")
                .param("action", "act")
                .param("actor_type", "admin")
                .param("actor_id", "A1")
                .param("resource_type", "card")
                .param("resource_id", "C1")
                .param("card", "CARD")
                .param("agent_id", "AG1")
                .param("machine", "M1")
                .param("ip", "IP1")
                .param("from", "2026-07-01T00:00:00Z")
                .param("q", "needle"))
            .andExpect(status().isOk())
            .andExpect(header().string("Content-Disposition", "attachment; filename=audit_export.csv"))
            .andExpect(content().contentType("text/csv; charset=utf-8"))
            .andExpect(content().bytes(AuditCsv.export(List.of())));

        verify(service).queryAll(filter);
    }

    @Test
    void invalidDatesAreIgnoredLikeGo() throws Exception {
        AuditFilter filter = new AuditFilter(
            null, null, null, null, null, null, null, null, null,
            null, null, null, 1, 50
        );
        when(service.query(filter)).thenReturn(new AuditPage(List.of(), 0, 1, 50));

        mvc.perform(get("/java/admin/api/audit").param("from", "not-a-date"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"));
    }

    @Test
    void queryFailureUsesGoCompatibleError() throws Exception {
        when(service.queryAll(org.mockito.ArgumentMatchers.any()))
            .thenThrow(new DataAccessResourceFailureException("database detail"));

        mvc.perform(get("/java/admin/api/audit").param("export", "csv"))
            .andExpect(status().isInternalServerError())
            .andExpect(jsonPath("$.detail").value("failed to query audit log"));
    }
}
