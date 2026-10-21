package com.lingqiao.server.cards;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-enabled", havingValue = "true")
public class CardsConfiguration {
    @Bean
    CardsService cardsService(JdbcTemplate jdbcTemplate) {
        return new CardsService(jdbcTemplate);
    }
}
