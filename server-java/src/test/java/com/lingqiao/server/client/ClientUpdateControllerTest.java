package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.releases.ManifestSeedLoader;
import com.lingqiao.server.releases.ReleaseEventRecorder;
import com.lingqiao.server.releases.ReleaseUpdateService;
import jakarta.servlet.http.HttpServletRequest;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

class ClientUpdateControllerTest {
    @Test
    void eventUsesHeaderCredentialsAndValidatedSessionIdentity() {
        ReleaseUpdateService updates = mock(ReleaseUpdateService.class);
        ReleaseEventRecorder events = mock(ReleaseEventRecorder.class);
        ClientSessionValidator sessions = mock(ClientSessionValidator.class);
        HmacAuthService hmac = mock(HmacAuthService.class);
        ManifestSeedLoader seeds = mock(ManifestSeedLoader.class);
        ClientUpdateController controller =
            new ClientUpdateController(updates, events, sessions, hmac, seeds);
        HttpServletRequest request = requestWithUpdateHeaders();
        when(sessions.validateBound("header-token", "header-machine", "header-card"))
            .thenReturn(new ClientSessionValidator.ValidatedSession("REAL-CARD", "REAL-MACHINE"));

        var response = controller.event(request, """
            {"session_token":"body-token","release_id":"rel-1","version":"1.0.0",
             "machine_id":"spoof-machine","card_code":"spoof-card","agent_id":"agent-1",
             "type":"custom_event","detail":"ok"}
            """);

        assertThat(response.get("status")).isEqualTo("ok");
        verify(sessions).validateBound("header-token", "header-machine", "header-card");
        ArgumentCaptor<ReleaseEventRecorder.ReleaseEvent> event =
            ArgumentCaptor.forClass(ReleaseEventRecorder.ReleaseEvent.class);
        verify(events).record(event.capture());
        assertThat(event.getValue().machineId()).isEqualTo("REAL-MACHINE");
        assertThat(event.getValue().cardCode()).isEqualTo("REAL-CARD");
        assertThat(event.getValue().type()).isEqualTo("custom_event");
    }

    @Test
    void requiredEventFieldsAreCheckedBeforeSessionLookup() {
        ReleaseEventRecorder events = mock(ReleaseEventRecorder.class);
        ClientSessionValidator sessions = mock(ClientSessionValidator.class);
        ClientUpdateController controller = new ClientUpdateController(
            mock(ReleaseUpdateService.class), events, sessions,
            mock(HmacAuthService.class), mock(ManifestSeedLoader.class));

        assertThatThrownBy(() -> controller.event(requestWithUpdateHeaders(), "{}"))
            .isInstanceOf(SidecarException.class)
            .hasMessage("release_id and type are required");
        verifyNoInteractions(sessions, events);
    }

    @Test
    void eventDetailIsCappedAtGoLimit() {
        ReleaseEventRecorder events = mock(ReleaseEventRecorder.class);
        ClientSessionValidator sessions = mock(ClientSessionValidator.class);
        when(sessions.validateBound(any(), any(), any()))
            .thenReturn(new ClientSessionValidator.ValidatedSession("C", "M"));
        ClientUpdateController controller = new ClientUpdateController(
            mock(ReleaseUpdateService.class), events, sessions,
            mock(HmacAuthService.class), mock(ManifestSeedLoader.class));

        controller.event(requestWithUpdateHeaders(),
            "{\"release_id\":\"r\",\"type\":\"x\",\"detail\":\"" + "a".repeat(700) + "\"}");

        ArgumentCaptor<ReleaseEventRecorder.ReleaseEvent> event =
            ArgumentCaptor.forClass(ReleaseEventRecorder.ReleaseEvent.class);
        verify(events).record(event.capture());
        assertThat(event.getValue().detail()).hasSize(500);
    }

    private static HttpServletRequest requestWithUpdateHeaders() {
        HttpServletRequest request = mock(HttpServletRequest.class);
        when(request.getHeader("X-Client-ID")).thenReturn("injector_v1");
        when(request.getHeader("X-HMAC-Signature")).thenReturn("signature");
        when(request.getHeader("X-Timestamp")).thenReturn("timestamp");
        when(request.getHeader("X-Nonce")).thenReturn("nonce");
        when(request.getHeader("X-Session-Token")).thenReturn("header-token");
        when(request.getHeader("X-Machine-ID")).thenReturn("header-machine");
        when(request.getHeader("X-Card-Code")).thenReturn("header-card");
        return request;
    }
}
