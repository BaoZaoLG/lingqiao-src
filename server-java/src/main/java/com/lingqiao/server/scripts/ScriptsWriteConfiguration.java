package com.lingqiao.server.scripts;

import com.lingqiao.server.audit.AuditWriteService;
import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "scripts-write-enabled", havingValue = "true")
public class ScriptsWriteConfiguration {
    @Bean
    ScriptsWriteService scriptsWriteService(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir,
        JdbcTemplate sidecarWriteJdbcTemplate
    ) {
        return new ScriptsWriteService(
            Path.of(dataDir), new AuditWriteService(sidecarWriteJdbcTemplate));
    }
}
