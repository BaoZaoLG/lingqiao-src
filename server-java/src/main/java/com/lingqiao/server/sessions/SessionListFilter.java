package com.lingqiao.server.sessions;

public record SessionListFilter(
    String cardCode,
    String machineId,
    boolean activeOnly,
    int page,
    int perPage
) {
    public SessionListFilter {
        page = page < 1 ? 1 : page;
        perPage = perPage < 1 || perPage > 200 ? 20 : perPage;
    }
}
