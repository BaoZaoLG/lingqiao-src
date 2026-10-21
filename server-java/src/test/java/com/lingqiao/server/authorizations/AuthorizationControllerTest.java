package com.lingqiao.server.authorizations;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.lingqiao.server.common.security.AdminAuthService;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest({AuthorizationReadController.class, AuthorizationWriteController.class})
@TestPropertySource(properties = {
    "lingqiao.sidecar.cards-enabled=true",
    "lingqiao.sidecar.cards-write-enabled=true"
})
class AuthorizationControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private AuthorizationControlService service;

    @MockBean
    private AdminAuthService adminAuthService;

    @Test
    void listParsesGoFiltersAndUsesCardsReadPermission() throws Exception {
        when(service.list(any())).thenReturn(List.of());

        mvc.perform(get("/java/admin/api/control/authorizations")
                .param("search", "needle")
                .param("effective_status", "in_use")
                .param("bound", "true")
                .param("expires_after", "2026-07-01T00:00:00Z")
                .param("expires_before", "bad")
                .param("max_sessions", "2")
                .header("Authorization", "Bearer token"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.authorizations").isArray());

        ArgumentCaptor<AuthorizationControlService.ListFilter> captor =
            ArgumentCaptor.forClass(AuthorizationControlService.ListFilter.class);
        verify(service).list(captor.capture());
        AuthorizationControlService.ListFilter filter = captor.getValue();
        org.assertj.core.api.Assertions.assertThat(filter.query()).isEqualTo("needle");
        org.assertj.core.api.Assertions.assertThat(filter.effectiveStatus()).isEqualTo("in_use");
        org.assertj.core.api.Assertions.assertThat(filter.expiresAfter())
            .isEqualTo(Instant.parse("2026-07-01T00:00:00Z"));
        org.assertj.core.api.Assertions.assertThat(filter.expiresBefore()).isNull();
        org.assertj.core.api.Assertions.assertThat(filter.maxSessions()).isEqualTo(2);
        verify(adminAuthService).requireAdmin("token", "cards.read");
    }

    @Test
    void detailUsesDynamicPathAndFlatAuthorizationShape() throws Exception {
        AuthorizationControlService.AuthorizationSummary summary =
            new AuthorizationControlService.AuthorizationSummary(
                "AUTH-CODE", "", "", Instant.parse("2026-07-01T00:00:00Z"),
                null, Instant.parse("2026-08-01T00:00:00Z"), "issued", "active",
                "", 1, 0, null);
        when(service.detail("AUTH-CODE")).thenReturn(
            new AuthorizationControlService.AuthorizationDetail(summary, List.of()));

        mvc.perform(get("/java/admin/api/control/authorizations/AUTH-CODE"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.authorization.code").value("AUTH-CODE"))
            .andExpect(jsonPath("$.authorization.summary").doesNotExist())
            .andExpect(jsonPath("$.authorization.sessions").isArray());
    }

    @Test
    void previewsUseReadAndBulkPermissionsAndAcceptPostWithoutWriteFlagSemantics() throws Exception {
        when(service.preview(any(), eq("archive_delete"), eq(0))).thenReturn(
            new AuthorizationControlService.ImpactPreview(
                "archive_delete", 1, 0, null, true, List.of("warning")));

        mvc.perform(post("/java/admin/api/control/authorizations/AUTH-CODE/impact")
                .contentType(MediaType.APPLICATION_JSON)
                .header("Authorization", "Bearer token")
                .content("{\"action\":\"archive_delete\"}"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.preview.target_count").value(1));
        verify(adminAuthService).requireAdmin("token", "cards.read");

        mvc.perform(post("/java/admin/api/control/authorizations/bulk/impact")
                .contentType(MediaType.APPLICATION_JSON)
                .header("Authorization", "Bearer token")
                .content("{\"codes\":[\"AUTH-CODE\"],\"action\":\"archive_delete\"}"))
            .andExpect(status().isOk());
        verify(adminAuthService).requireAdmin("token", "cards.bulk");
    }

    @Test
    void actionsExposePartialResultAndPreserveGoPermissions() throws Exception {
        when(service.execute(any(), eq("freeze"), eq(0))).thenReturn(
            new AuthorizationControlService.ActionResult(1, 0, 0, List.of("AUTH-CODE")));

        mvc.perform(post("/java/admin/api/control/authorizations/AUTH-CODE/actions")
                .contentType(MediaType.APPLICATION_JSON)
                .header("Authorization", "Bearer token")
                .content("{\"action\":\"freeze\"}"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.result.updated").value(1))
            .andExpect(jsonPath("$.result.codes[0]").value("AUTH-CODE"));
        verify(adminAuthService).requireAdmin("token", "cards.read");

        mvc.perform(post("/java/admin/api/control/authorizations/bulk/actions")
                .contentType(MediaType.APPLICATION_JSON)
                .header("Authorization", "Bearer token")
                .content("{\"codes\":[\"AUTH-CODE\"],\"action\":\"freeze\"}"))
            .andExpect(status().isOk());
        verify(adminAuthService).requireAdmin("token", "cards.bulk");
    }

    @Test
    void malformedJsonReturnsStableBadRequestInsteadOfInternalError() throws Exception {
        mvc.perform(post("/java/admin/api/control/authorizations/AUTH-CODE/impact")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.code").value("INVALID_JSON"))
            .andExpect(jsonPath("$.detail").value("invalid request body"));
    }
}
