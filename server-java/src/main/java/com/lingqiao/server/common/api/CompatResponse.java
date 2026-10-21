package com.lingqiao.server.common.api;

import java.util.LinkedHashMap;
import java.util.Map;

public final class CompatResponse {
    private CompatResponse() {
    }

    public static Map<String, Object> ok(Map<String, ?> legacyFields) {
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("status", "ok");
        if (legacyFields != null) {
            response.putAll(legacyFields);
        }
        return response;
    }

    public static Map<String, Object> ok(Object data) {
        Map<String, Object> response = new LinkedHashMap<>();
        response.put("status", "ok");
        if (data instanceof String message) {
            response.put("message", message);
        } else {
            response.put("data", data);
        }
        return response;
    }
}
