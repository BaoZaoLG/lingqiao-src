package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

class ClientSessionRequestTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void acceptsClientIdFromCompatibleClientActivationPayload() throws Exception {
        ClientSessionController.ClientRequest request = mapper.readValue(
            """
            {
              "client_id": "injector_v1",
              "card": "INVALID-TEST",
              "machine_id": "test-machine",
              "fingerprint": "test-fingerprint",
              "client_version": "2.4.0"
            }
            """,
            ClientSessionController.ClientRequest.class);

        assertThat(request.card()).isEqualTo("INVALID-TEST");
        assertThat(request.machine_id()).isEqualTo("test-machine");
        assertThat(request.fingerprint()).isEqualTo("test-fingerprint");
        assertThat(request.client_version()).isEqualTo("2.4.0");
    }
}