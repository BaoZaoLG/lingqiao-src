package com.lingqiao.server.system;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(HealthController.class)
class HealthControllerTest {
    @Autowired
    private MockMvc mvc;

    @Test
    void javaHealthEndpointIsSidecarOnlyAndCompatOk() throws Exception {
        mvc.perform(get("/java/api/health"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.service").value("lingqiao-java-sidecar"))
            .andExpect(jsonPath("$.mode").value("sidecar"))
            .andExpect(jsonPath("$.exam_enabled").value(false))
            .andExpect(jsonPath("$.audit_enabled").value(false))
            .andExpect(jsonPath("$.production_entrypoint_replaced").value(false));
    }
}
