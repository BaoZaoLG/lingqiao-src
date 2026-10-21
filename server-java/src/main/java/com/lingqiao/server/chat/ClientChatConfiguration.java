package com.lingqiao.server.chat;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-chat-enabled", havingValue = "true")
public class ClientChatConfiguration {
    @Bean
    ChatStateStore chatStateStore(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new ChatStateStore(Path.of(dataDir));
    }

    @Bean
    ClientChatService clientChatService(ChatStateStore chatStateStore) {
        return new ClientChatService(chatStateStore);
    }
}
