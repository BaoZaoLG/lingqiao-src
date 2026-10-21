package com.lingqiao.server.common.security;

import java.util.Set;

public record AdminPrincipal(
    String id,
    String username,
    String role,
    Set<String> permissions
) {
    public boolean hasPermission(String permission) {
        return permissions != null && permissions.contains(permission);
    }
}
