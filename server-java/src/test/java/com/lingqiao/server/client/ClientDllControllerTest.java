package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.lingqiao.server.common.security.HmacAuthService;
import jakarta.servlet.http.HttpServletRequest;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class ClientDllControllerTest {
    @Test
    void configuredSecretFileIsUsedWhenInlineSecretIsEmpty() throws Exception {
        Path secretFile = Files.createTempFile("lingqiao-hmac", ".key");
        Files.writeString(secretFile, "file-secret\n");

        DllSourceResolver resolver = mock(DllSourceResolver.class);
        ClientSessionValidator sessions = mock(ClientSessionValidator.class);
        HmacAuthService hmac = mock(HmacAuthService.class);
        HttpServletRequest request = mock(HttpServletRequest.class);
        when(request.getRequestURI()).thenReturn("/java/api/v1/dll");
        when(request.getHeader("X-Client-ID")).thenReturn("injector_v1");
        when(request.getHeader("X-HMAC-Signature")).thenReturn("signature");
        when(request.getHeader("X-Timestamp")).thenReturn("timestamp");
        when(request.getHeader("X-Nonce")).thenReturn("nonce");
        when(request.getHeader("X-Session-Token")).thenReturn("token");
        when(request.getHeader("X-Machine-ID")).thenReturn("machine");
        when(request.getHeader("X-Card-Code")).thenReturn("card");
        when(resolver.resolve()).thenReturn(new DllSourceResolver.DllSource(
            new byte[] {'M', 'Z', 1}, "legacy-file", "", "sha256"));

        ClientDllController controller = new ClientDllController(
            resolver, sessions, hmac, "", secretFile.toString());

        var response = controller.download(request);

        assertThat(response.getStatusCode().value()).isEqualTo(200);
        assertThat(response.getBody()).isNotEmpty();
        verify(hmac).verify("injector_v1", "/api/v1/dll", "signature", "timestamp", "nonce");
        verify(sessions).validateBound("token", "machine", "card");
    }
}