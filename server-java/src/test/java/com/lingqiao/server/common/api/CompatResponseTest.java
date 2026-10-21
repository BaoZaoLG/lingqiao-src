package com.lingqiao.server.common.api;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Map;
import org.junit.jupiter.api.Test;

class CompatResponseTest {
    @Test
    void okPreservesTopLevelStatusAndLegacyFields() {
        Map<String, Object> response = CompatResponse.ok(Map.of("inserted", 2));

        assertThat(response).containsEntry("status", "ok");
        assertThat(response).containsEntry("inserted", 2);
        assertThat(response).doesNotContainKey("data");
    }

    @Test
    void okAcceptsEmptyLegacyBody() {
        Map<String, Object> response = CompatResponse.ok((Map<String, ?>) null);

        assertThat(response).containsEntry("status", "ok");
        assertThat(response).hasSize(1);
    }
}
