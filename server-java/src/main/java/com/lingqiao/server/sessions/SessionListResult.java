package com.lingqiao.server.sessions;

import java.util.List;

public record SessionListResult(
    List<Session> items,
    long total,
    int page,
    int perPage
) {
}
