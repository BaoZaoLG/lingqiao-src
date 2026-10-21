package com.lingqiao.server.releases;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.multipart;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.lingqiao.server.common.security.AdminAuthService;
import java.time.Instant;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.mock.web.MockMultipartFile;

@WebMvcTest(ReleasesWriteController.class)
@TestPropertySource(properties = "lingqiao.sidecar.releases-write-enabled=true")
class ReleasesWriteControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private ReleaseAdminService service;

    @MockBean
    private AdminAuthService adminAuthService;

    @Test
    void basePostUsesGoCreateProtocol() throws Exception {
        when(service.create(any())).thenReturn(release("rel-1", "draft"));

        mvc.perform(post("/java/admin/api/releases")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {"version":"2.0.0","channel":"beta","min_version":"1.5.0",
                     "force_update":true,"rollout_percent":25,
                     "targeting":{"allow_cards":["C1"]}}
                    """))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.release.id").value("rel-1"));

        ArgumentCaptor<ReleaseAdminService.CreateRelease> input =
            ArgumentCaptor.forClass(ReleaseAdminService.CreateRelease.class);
        verify(service).create(input.capture());
        org.assertj.core.api.Assertions.assertThat(input.getValue().version()).isEqualTo("2.0.0");
        org.assertj.core.api.Assertions.assertThat(input.getValue().targeting().allowCards())
            .containsExactly("C1");
    }

    @Test
    void malformedRollbackBodyIsIgnoredLikeGo() throws Exception {
        when(service.rollback("rel-1", "")).thenReturn(release("rel-1", "rolled_back"));

        mvc.perform(post("/java/admin/api/releases/rel-1/rollback")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.release.status").value("rolled_back"));

        verify(service).rollback("rel-1", "");
    }

    @Test
    void packageUploadUsesDynamicMultipartRoute() throws Exception {
        MockMultipartFile file = new MockMultipartFile(
            "file", "setup.exe", "application/octet-stream", "body".getBytes());
        ReleasePackage pkg = new ReleasePackage(
            "rel-1-bundle", "rel-1", "bundle", "setup.exe", "updates/setup.exe",
            4, "sha", Instant.parse("2026-07-01T00:00:00Z"));
        when(service.uploadPackage("rel-1", "bundle", file)).thenReturn(pkg);

        mvc.perform(multipart("/java/admin/api/releases/rel-1/packages")
                .file(file)
                .param("kind", "bundle"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.package.id").value("rel-1-bundle"));

        verify(service).uploadPackage("rel-1", "bundle", file);
    }

    private static Release release(String id, String status) {
        return new Release(
            id, "2.0.0", "stable", status, "", false, 100, "",
            TargetingRules.empty(), "s1", "1", "sha", Instant.parse("2026-07-01T00:00:00Z"),
            null, null, null, "");
    }
}
