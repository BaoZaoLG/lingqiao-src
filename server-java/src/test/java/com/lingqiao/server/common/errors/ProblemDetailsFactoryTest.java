package com.lingqiao.server.common.errors;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class ProblemDetailsFactoryTest {
    @Test
    void createsStableProblemDetailsUriFromErrorCode() {
        ProblemDetails problem = ProblemDetailsFactory.from(ErrorCode.HMAC_INVALID, "/api/v1/activate", "HMAC verification failed");

        assertThat(problem.type()).isEqualTo("https://lingqiao.local/problems/hmac-invalid");
        assertThat(problem.title()).isEqualTo("Unauthorized");
        assertThat(problem.status()).isEqualTo(401);
        assertThat(problem.code()).isEqualTo("HMAC_INVALID");
        assertThat(problem.instance()).isEqualTo("/api/v1/activate");
        assertThat(problem.detail()).isEqualTo("HMAC verification failed");
    }
}
