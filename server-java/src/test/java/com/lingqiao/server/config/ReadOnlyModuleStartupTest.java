package com.lingqiao.server.config;

import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.exam.ExamRepository;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.TestPropertySource;

/**
 * A read-only module must be able to start on its own. Read modules need a
 * JdbcTemplate, and DataSourceAutoConfiguration is excluded, so the datasource
 * configuration has to activate for read flags too — not only write flags.
 */
@SpringBootTest
@TestPropertySource(properties = {
    "lingqiao.sidecar.exam-enabled=true",
    "lingqiao.write.db-url=jdbc:sqlite:file:memdb-readonly-start?mode=memory&cache=shared"
})
class ReadOnlyModuleStartupTest {
    @Autowired
    private JdbcTemplate jdbcTemplate;

    @Autowired
    private ExamRepository examRepository;

    @Test
    void readModuleStartsWithoutAnyWriteFlag() {
        assertThat(jdbcTemplate).isNotNull();
        assertThat(examRepository).isNotNull();
    }
}
