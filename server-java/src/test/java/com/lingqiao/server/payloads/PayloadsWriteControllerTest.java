package com.lingqiao.server.payloads;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.multipart;
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
import org.springframework.mock.web.MockMultipartFile;
import org.springframework.test.context.TestPropertySource;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(PayloadsWriteController.class)
@TestPropertySource(properties = "lingqiao.sidecar.payloads-write-enabled=true")
class PayloadsWriteControllerTest {
    @Autowired
    private MockMvc mvc;

    @MockBean
    private PayloadsWriteService service;

    @MockBean
    private AdminAuthService adminAuthService;

    @MockBean
    private PayloadUploadKey uploadKey;

    @Test
    void adminDllUploadUsesGoMultipartProtocolAndCanActivate() throws Exception {
        PayloadDocument document = document("payload-a", 4, 4);
        PayloadPacker.PackedPayload packed = new PayloadPacker.PackedPayload(
            document,
            new PayloadPacker.PayloadKeyMaterial("00".repeat(32), "11".repeat(32), "22".repeat(16)),
            List.of(new byte[]{1, 2, 3, 4}));
        when(service.upload(eq("payload-a"), any(), eq(PayloadPacker.DEFAULT_CHUNK_SIZE)))
            .thenReturn(packed);
        when(service.get("payload-a")).thenReturn(document.toSummary(true));

        MockMultipartFile file = new MockMultipartFile(
            "file", "payload.DLL", "application/octet-stream", new byte[]{'M', 'Z', 1, 2});
        mvc.perform(multipart("/java/admin/api/payloads/upload-dll")
                .file(file)
                .param("payload_id", "payload-a")
                .param("activate", "1"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.payload.payload_id").value("payload-a"))
            .andExpect(jsonPath("$.payload.active").value(true));

        verify(service).activate("payload-a");
    }

    @Test
    void manageActivateReturnsPayloadLikeGo() throws Exception {
        PayloadDocument document = document("payload-a", 4, 4);
        when(service.get("payload-a")).thenReturn(document.toSummary(true));

        mvc.perform(post("/java/admin/api/payloads/manage")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"action\":\"activate\",\"payload_id\":\"payload-a\"}"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.payload.payload_id").value("payload-a"));
    }

    @Test
    void legacyUploadPreservesProvidedEncryptedChunks() throws Exception {
        when(uploadKey.matches("upload-secret")).thenReturn(true);

        mvc.perform(post("/java/admin/api/payload/upload")
                .header("X-Upload-Key", "upload-secret")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {
                      "payload_id":"legacy-a",
                      "aes_key":"%s",
                      "hmac_key":"%s",
                      "iv":"%s",
                      "exe_hash":"%s",
                      "chunk_count":2,
                      "chunk_size":3,
                      "total_size":5,
                      "chunks":["AQID","BAU="]
                    }
                    """.formatted(
                        "00".repeat(32), "11".repeat(32), "22".repeat(16), "33".repeat(32))))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"))
            .andExpect(jsonPath("$.payload_id").value("legacy-a"))
            .andExpect(jsonPath("$.chunk_count").value(2));

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<byte[]>> chunks = ArgumentCaptor.forClass(List.class);
        verify(service).importEncrypted(any(PayloadDocument.class), chunks.capture());
        org.assertj.core.api.Assertions.assertThat(chunks.getValue().get(0))
            .containsExactly(1, 2, 3);
        org.assertj.core.api.Assertions.assertThat(chunks.getValue().get(1))
            .containsExactly(4, 5);
    }

    @Test
    void legacyUploadChecksUploadKeyBeforeReadingBody() throws Exception {
        mvc.perform(post("/java/admin/api/payload/upload")
                .header("X-Upload-Key", "wrong")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{"))
            .andExpect(status().isUnauthorized());
    }

    @Test
    void legacyUploadRejectsNonFinalShortChunk() throws Exception {
        when(uploadKey.matches("upload-secret")).thenReturn(true);

        mvc.perform(post("/java/admin/api/payload/upload")
                .header("X-Upload-Key", "upload-secret")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {"payload_id":"legacy-a","aes_key":"%s","hmac_key":"%s","iv":"%s",
                     "exe_hash":"%s","chunk_count":2,"chunk_size":3,"total_size":4,
                     "chunks":["AQI=","AwQ="]}
                    """.formatted(
                        "00".repeat(32), "11".repeat(32), "22".repeat(16), "33".repeat(32))))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.detail").value("chunk 0 size does not match chunk_size"));
    }

    private static PayloadDocument document(String id, int chunkSize, int totalSize) {
        PayloadDocument document = new PayloadDocument();
        document.setPayloadId(id);
        document.setAesKey("00".repeat(32));
        document.setHmacKey("11".repeat(32));
        document.setIv("22".repeat(16));
        document.setExeHash("33".repeat(32));
        document.setChunkCount(1);
        document.setChunkSize(chunkSize);
        document.setTotalSize(totalSize);
        document.setCreatedAt(Instant.parse("2026-07-01T00:00:00Z"));
        return document;
    }
}
