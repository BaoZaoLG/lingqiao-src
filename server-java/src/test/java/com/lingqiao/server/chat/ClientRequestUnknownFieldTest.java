package com.lingqiao.server.chat;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.client.ClientUpdateController;
import org.junit.jupiter.api.Test;

/**
 * Contract guard for the Go -> Java client request bodies.
 *
 * The Windows client sends client_id in every body it posts (see
 * client/src/ui/impl_chat.h buildChatAuthRequest and impl_update.h). Go's
 * encoding/json silently drops unknown fields, so those requests always worked
 * against the Go server. The sidecar used a stock Jackson mapper, whose
 * FAIL_ON_UNKNOWN_PROPERTIES default is true, so the identical payload was
 * rejected — and the generic catch in each controller reported it as
 * INVALID_JSON. That is why /api/v1/chat/presence, /profile, /messages and
 * /api/v1/update/check all returned 400 with a misleading "invalid json" the
 * moment the cutover routed them to the sidecar.
 *
 * The request records must therefore tolerate fields they do not model, while
 * genuinely malformed JSON must still be rejected.
 */
class ClientRequestUnknownFieldTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void chatRequestToleratesTheClientIdSentByTheClient() throws Exception {
        String body = "{\"client_id\":\"lq-client\",\"session_token\":\"s\",\"machine_id\":\"m\",\"card\":\"c\"}";

        ClientChatController.ChatRequest req =
            mapper.readValue(body, ClientChatController.ChatRequest.class);

        assertThat(req.session_token()).isEqualTo("s");
        assertThat(req.machine_id()).isEqualTo("m");
        assertThat(req.card()).isEqualTo("c");
    }

    @Test
    void updateCheckRequestToleratesTheClientIdSentByTheClient() throws Exception {
        String body = "{\"client_id\":\"lq-client\",\"session_token\":\"s\",\"machine_id\":\"m\","
            + "\"client_version\":\"1.0.0\",\"channel\":\"stable\"}";

        ClientUpdateController.UpdateCheckRequest req =
            mapper.readValue(body, ClientUpdateController.UpdateCheckRequest.class);

        assertThat(req.client_version()).isEqualTo("1.0.0");
        assertThat(req.channel()).isEqualTo("stable");
    }

    @Test
    void genuinelyMalformedJsonIsStillRejected() {
        assertThatThrownBy(() -> mapper.readValue("{not json", ClientChatController.ChatRequest.class))
            .isInstanceOf(Exception.class);
    }
}
