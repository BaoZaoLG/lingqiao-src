package com.lingqiao.server.exam;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "exam-write-enabled", havingValue = "true")
public class ExamWriteConfiguration {
    @Bean
    ExamWriteService examWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new ExamWriteService(sidecarWriteJdbcTemplate);
    }

    @Bean
    ExamSessionLookup examSessionLookup(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new ExamSessionLookup(sidecarWriteJdbcTemplate);
    }
}
