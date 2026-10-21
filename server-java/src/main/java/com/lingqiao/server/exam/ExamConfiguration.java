package com.lingqiao.server.exam;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "exam-enabled", havingValue = "true")
public class ExamConfiguration {
    @Bean
    ExamRepository examRepository(JdbcTemplate jdbcTemplate) {
        return new JdbcExamRepository(jdbcTemplate);
    }

    @Bean
    ExamService examService(ExamRepository repository) {
        return new ExamService(repository);
    }
}
