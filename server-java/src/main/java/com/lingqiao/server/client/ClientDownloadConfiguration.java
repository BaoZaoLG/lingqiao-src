package com.lingqiao.server.client;

import com.lingqiao.server.payloads.PayloadStateReader;
import com.lingqiao.server.scripts.FileScriptRepository;
import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-download-enabled", havingValue = "true")
public class ClientDownloadConfiguration {
    @Bean
    PayloadStateReader payloadStateReader(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new PayloadStateReader(Path.of(dataDir));
    }

    @Bean
    ClientSessionValidator clientSessionValidator(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new ClientSessionValidator(sidecarWriteJdbcTemplate);
    }

    @Bean
    FileScriptRepository clientScriptRepository(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new FileScriptRepository(Path.of(dataDir));
    }

    @Bean
    DllSourceResolver dllSourceResolver(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir,
        PayloadStateReader payloadStateReader
    ) {
        return new DllSourceResolver(Path.of(dataDir), payloadStateReader);
    }
}
