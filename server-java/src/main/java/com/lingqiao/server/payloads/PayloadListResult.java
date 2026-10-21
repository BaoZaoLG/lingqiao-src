package com.lingqiao.server.payloads;

import java.util.List;

public record PayloadListResult(List<Payload> payloads, String activeId) {
}
