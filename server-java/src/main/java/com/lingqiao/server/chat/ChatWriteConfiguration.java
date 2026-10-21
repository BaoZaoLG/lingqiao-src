package com.lingqiao.server.chat;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "chat-write-enabled", havingValue = "true")
public class ChatWriteConfiguration {
    @Bean
    ChatWriteService chatWriteService(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new ChatWriteService(Path.of(dataDir));
    }
}
