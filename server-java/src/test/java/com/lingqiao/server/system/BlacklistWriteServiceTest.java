package com.lingqiao.server.system;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class BlacklistWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private BlacklistWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE blacklist (
              value TEXT PRIMARY KEY,
              type TEXT NOT NULL DEFAULT '',
              reason TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL
            )
            """);
        service = new BlacklistWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void addTrimsValueAndUpsertsLikeGo() {
        service.add("machine", "  M1  ", "abuse");
        service.add("machine", "M1", "updated");

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM blacklist", Integer.class)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT reason FROM blacklist WHERE value = 'M1'", String.class))
            .isEqualTo("updated");
    }

    @Test
    void removeDeletesTrimmedValue() {
        service.add("card", "C1", "");
        service.remove(" C1 ");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM blacklist", Integer.class)).isZero();
    }

    @Test
    void rejectsBlankValues() {
        assertThatThrownBy(() -> service.add("card", "   ", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("value is required");
        assertThatThrownBy(() -> service.remove(null))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("value is required");
    }
}
