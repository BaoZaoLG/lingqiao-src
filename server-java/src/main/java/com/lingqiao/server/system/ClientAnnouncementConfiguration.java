package com.lingqiao.server.system;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-announcement-enabled", havingValue = "true")
public class ClientAnnouncementConfiguration {
    @Bean
    AnnouncementReadSupport clientAnnouncementReadSupport(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir
    ) {
        return new AnnouncementReadSupport(Path.of(dataDir));
    }
}
