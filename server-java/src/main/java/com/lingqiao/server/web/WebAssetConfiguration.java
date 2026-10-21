package com.lingqiao.server.web;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class WebAssetConfiguration {
    @Bean
    WebAssetResolver webAssetResolver(@Value("${lingqiao.web.dist-dir:./web-dist}") String distDir) {
        return new WebAssetResolver(Path.of(distDir));
    }
}
