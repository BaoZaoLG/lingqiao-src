package com.lingqiao.server.common.web;

import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.common.errors.ProblemDetails;
import org.junit.jupiter.api.Test;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.mock.http.MockHttpInputMessage;
import org.springframework.mock.web.MockHttpServletRequest;

class GlobalExceptionHandlerTest {
    @Test
    void malformedJsonMapsToGoCompatibleBadRequest() {
        MockHttpServletRequest request = new MockHttpServletRequest(
            "POST", "/java/admin/api/control/authorizations/A/impact");
        ResponseEntity<ProblemDetails> response = new GlobalExceptionHandler()
            .handleInvalidJson(
                new HttpMessageNotReadableException(
                    "bad", new MockHttpInputMessage(new byte[0])),
                request);

        assertThat(response.getStatusCode().value()).isEqualTo(400);
        assertThat(response.getBody()).isNotNull();
        assertThat(response.getBody().code()).isEqualTo("INVALID_JSON");
        assertThat(response.getBody().detail()).isEqualTo("invalid request body");
        assertThat(response.getBody().instance()).isEqualTo(
            "/java/admin/api/control/authorizations/A/impact");
    }
}
