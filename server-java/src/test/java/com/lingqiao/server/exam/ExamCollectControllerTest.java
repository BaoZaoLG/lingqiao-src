package com.lingqiao.server.exam;

import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.MediaType;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(ExamCollectController.class)
@TestPropertySource(properties = "lingqiao.sidecar.exam-write-enabled=true")
class ExamCollectControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private ExamWriteService writeService;

    @MockBean
    private ExamSessionLookup sessions;

    @Test
    void acceptsSessionAuthenticatedPageContextWithoutHmac() throws Exception {
        when(sessions.requireActive("tok-1"))
            .thenReturn(new ExamSessionLookup.ExamSession("CARD-1", "M1"));
        when(writeService.insertBatch(anyList())).thenReturn(1);

        mvc.perform(post("/java/api/v1/exam/collect")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {"session_token":"tok-1","exam_id":"exam-1","questions":[
                      {"question_id":"q1","provided_answer":"A","fill_ok":true}
                    ]}
                    """))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.inserted").value(1));

        verify(sessions).requireActive("tok-1");
        verify(writeService).insertBatch(anyList());
    }

    @Test
    void malformedJsonUsesGoCompatibleMessage() throws Exception {
        mvc.perform(post("/java/api/v1/exam/collect")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.detail").value("invalid json"));

        mvc.perform(post("/java/api/v1/exam/collect").content(""))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.detail").value("invalid json"));

        mvc.perform(post("/java/api/v1/exam/collect")
                .content("{} {}"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.detail").value("invalid json"));
    }

    @Test
    void writeFailureDoesNotLeakDatabaseDetails() throws Exception {
        when(sessions.requireActive("tok-1"))
            .thenReturn(new ExamSessionLookup.ExamSession("CARD-1", "M1"));
        when(writeService.insertBatch(anyList()))
            .thenThrow(new DataIntegrityViolationException("secret database detail"));

        mvc.perform(post("/java/api/v1/exam/collect")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"session_token\":\"tok-1\",\"questions\":[]}"))
            .andExpect(status().isInternalServerError())
            .andExpect(jsonPath("$.detail").value("failed to save exam records"));
    }
}
