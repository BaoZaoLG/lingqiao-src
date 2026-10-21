package com.lingqiao.server.scripts;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "scripts-enabled", havingValue = "true")
public class ScriptsConfiguration {
    @Bean
    FileScriptRepository fileScriptRepository(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new FileScriptRepository(Path.of(dataDir));
    }

    @Bean
    ScriptsService scriptsService(
        @Qualifier("fileScriptRepository") FileScriptRepository repository
    ) {
        return new ScriptsService(repository);
    }
}
