package com.lingqiao.server.system;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(controllers = {HealthController.class, CutoverController.class})
class CutoverControllerTest {
    @Autowired
    private MockMvc mvc;

    @Test
    void cutoverDefaultsRemainDisabled() throws Exception {
        mvc.perform(get("/java/api/cutover"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.enabled").value(false))
            .andExpect(jsonPath("$.production_entrypoint_replaced").value(false))
            .andExpect(jsonPath("$.modules.admin.cutover").value(false))
            .andExpect(jsonPath("$.modules.exam.cutover").value(false))
            .andExpect(jsonPath("$.modules.cards.cutover").value(false));
    }
}
