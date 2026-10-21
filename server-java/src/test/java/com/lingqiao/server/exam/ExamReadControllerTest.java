package com.lingqiao.server.exam;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.mockito.Mockito.verify;
import org.springframework.dao.DataAccessResourceFailureException;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

import static org.mockito.Mockito.when;

@WebMvcTest(ExamReadController.class)
@TestPropertySource(properties = "lingqiao.sidecar.exam-enabled=true")
class ExamReadControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private ExamService service;

    @MockBean
    private com.lingqiao.server.common.security.AdminAuthService adminAuthService;

    @Test
    void statsUsesJavaOnlyReadPath() throws Exception {
        when(service.stats()).thenReturn(new ExamStats(3, 2, 1));

        mvc.perform(get("/java/admin/api/exam/stats"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.total").value(3));
    }

    @Test
    void recordsResponseMatchesGoFieldSet() throws Exception {
        when(service.list(new ExamFilter(null, null, null, 1, 50)))
            .thenReturn(new ExamPage(List.of(), 0));

        mvc.perform(get("/java/admin/api/exam/records"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.records").value(org.hamcrest.Matchers.nullValue()))
            .andExpect(jsonPath("$.total").value(0))
            .andExpect(jsonPath("$.page").value(1))
            .andExpect(jsonPath("$.page_size").value(50))
            // Go has no read_only field; the panels must not see one.
            .andExpect(jsonPath("$.read_only").doesNotExist());
    }

    @Test
    void exportUsesFilteredGoCompatibleCsvDownload() throws Exception {
        ExamFilter filter = new ExamFilter("C1", "E1", "q1", 1, 50);
        when(service.listAll(filter)).thenReturn(List.of());

        mvc.perform(get("/java/admin/api/exam/export")
                .param("card_code", "C1")
                .param("exam_id", "E1")
                .param("q", "q1"))
            .andExpect(status().isOk())
            .andExpect(header().string("Content-Disposition",
                org.hamcrest.Matchers.matchesPattern(
                    "attachment; filename=\"exam_records_\\d{8}_\\d{6}\\.csv\"")))
            .andExpect(content().contentType("text/csv;charset=utf-8"))
            .andExpect(content().bytes(ExamCsv.export(List.of())));

        verify(service).listAll(filter);
    }

    @Test
    void exportFailureUsesGoCompatibleError() throws Exception {
        when(service.listAll(new ExamFilter(null, null, null, 1, 50)))
            .thenThrow(new DataAccessResourceFailureException("database detail"));

        mvc.perform(get("/java/admin/api/exam/export"))
            .andExpect(status().isInternalServerError())
            .andExpect(jsonPath("$.detail").value("failed to export exam records"));
    }
}
