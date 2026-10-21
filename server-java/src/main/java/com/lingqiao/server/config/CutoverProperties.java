package com.lingqiao.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "lingqiao.cutover")
public record CutoverProperties(
    boolean enabled,
    boolean admin,
    boolean exam,
    boolean audit,
    boolean scripts,
    boolean cards,
    boolean sessions,
    boolean agents,
    boolean chat,
    boolean blacklist,
    boolean announcement,
    boolean releases,
    boolean payloads
) {
}
