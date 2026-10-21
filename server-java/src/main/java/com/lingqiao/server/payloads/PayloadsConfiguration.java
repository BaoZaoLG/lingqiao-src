package com.lingqiao.server.payloads;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "payloads-enabled", havingValue = "true")
public class PayloadsConfiguration {
    @Bean
    PayloadsService payloadsService(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new PayloadsService(Path.of(dataDir));
    }
}
