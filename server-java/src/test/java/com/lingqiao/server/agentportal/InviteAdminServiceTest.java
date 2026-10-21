package com.lingqiao.server.agentportal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class InviteAdminServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private InviteAdminService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE invites (
              code TEXT PRIMARY KEY,
              created_at DATETIME NOT NULL,
              created_by TEXT NOT NULL DEFAULT '',
              used_by TEXT NOT NULL DEFAULT '',
              used_at DATETIME,
              max_uses INTEGER NOT NULL DEFAULT 0,
              use_count INTEGER NOT NULL DEFAULT 0
            )
            """);
        jdbc.execute("""
            CREATE TABLE audit_log (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              time DATETIME NOT NULL,
              action TEXT NOT NULL,
              detail TEXT NOT NULL DEFAULT ''
            )
            """);
        service = new InviteAdminService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void createListAndDeleteUseGoShapeAndAuditActions() {
        var created = service.create(3, 5);

        assertThat(created).hasSize(3).allSatisfy(invite -> {
            assertThat(invite.code()).matches("INV-[0-9A-F]{16}");
            assertThat(invite.createdBy()).isEqualTo("admin");
            assertThat(invite.maxUses()).isEqualTo(5);
            assertThat(invite.useCount()).isZero();
            assertThat(invite.usedBy()).isEmpty();
            assertThat(invite.usedAt()).isNull();
        });
        assertThat(service.list()).extracting(InviteAdminService.InviteView::code)
            .containsExactlyInAnyOrderElementsOf(
                created.stream().map(InviteAdminService.InviteView::code).toList());
        assertThat(jdbc.queryForObject(
            "SELECT COUNT(*) FROM audit_log WHERE action = 'invite_created'", Integer.class))
            .isEqualTo(1);

        service.delete(created.get(0).code());

        assertThat(service.list()).hasSize(2);
        assertThat(jdbc.queryForObject(
            "SELECT COUNT(*) FROM audit_log WHERE action = 'invite_deleted'", Integer.class))
            .isEqualTo(1);
    }

    @Test
    void missingInviteReturnsGoNotFoundMessage() {
        assertThatThrownBy(() -> service.delete("INV-MISSING"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("邀请码不存在");
    }

    @Test
    void auditFailureRollsBackInviteMutation() {
        jdbc.execute("DROP TABLE audit_log");

        assertThatThrownBy(() -> service.create(1, 1)).isInstanceOf(Exception.class);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM invites", Integer.class)).isZero();
    }
}
