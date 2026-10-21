package com.lingqiao.server.common.errors;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;

@JsonInclude(JsonInclude.Include.NON_NULL)
@JsonPropertyOrder({"type", "title", "status", "detail", "instance", "code"})
public record ProblemDetails(
    String type,
    String title,
    int status,
    String detail,
    String instance,
    String code
) {
}
