package com.lingqiao.server.cards;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-write-enabled", havingValue = "true")
public class CardsWriteConfiguration {
    @Bean
    CardsWriteService cardsWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new CardsWriteService(sidecarWriteJdbcTemplate, new CardsService(sidecarWriteJdbcTemplate));
    }

    @Bean
    CardsCsvService cardsCsvService(
        JdbcTemplate sidecarWriteJdbcTemplate,
        CardsWriteService cardsWriteService
    ) {
        return new CardsCsvService(
            sidecarWriteJdbcTemplate,
            new CardsService(sidecarWriteJdbcTemplate),
            cardsWriteService
        );
    }
}
