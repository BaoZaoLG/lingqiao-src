package com.lingqiao.server.config;

import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@EnableConfigurationProperties(CutoverProperties.class)
public class CutoverConfiguration {
}
