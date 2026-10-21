package com.lingqiao.server.cards;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.charset.StandardCharsets;
import java.sql.DriverManager;
import java.time.Duration;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class CardsCsvServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private CardsCsvService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY,
              machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL,
              activated_at DATETIME,
              expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active',
              note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        createAuditTable();
        CardsService reader = new CardsService(jdbc);
        CardsWriteService writer = new CardsWriteService(jdbc, reader);
        service = new CardsCsvService(jdbc, reader, writer);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void dryRunAcceptsBomQuotedFieldsAndReportsDuplicates() {
        String csv = "\uFEFFcode,note\r\n"
            + "012345-6789AB-CDEFGH,\"hello, world\"\r\n"
            + "0123456789ABCDEFGH,again\r\n"
            + "bad-code,nope\r\n";

        CardsCsvService.ImportReport report =
            service.importCsv(csv, Duration.ofHours(24), 2, true);

        assertThat(report.totalRows()).isEqualTo(3);
        assertThat(report.validRows()).isEqualTo(1);
        assertThat(report.duplicates()).isEqualTo(1);
        assertThat(report.invalid()).isEqualTo(1);
        assertThat(report.imported()).isZero();
        assertThat(report.items()).extracting(CardsCsvService.ImportItem::status)
            .containsExactly("valid", "duplicate", "invalid");
        assertThat(report.items().get(0).note()).isEqualTo("hello, world");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM cards", Integer.class)).isZero();
    }

    @Test
    void importCreatesCardsAndPerItemPlusSummaryAudits() {
        String csv = "卡密,备注\n"
            + "0123456789ABCDEFGH,first\n"
            + "JKMNPQRSTVWXYZ0123,second\n";

        CardsCsvService.ImportReport report =
            service.importCsv(csv, Duration.ofHours(24), 4, false);

        assertThat(report.imported()).isEqualTo(2);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM cards", Integer.class)).isEqualTo(2);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM audit_log", Integer.class)).isEqualTo(3);
        assertThat(jdbc.queryForList(
            "SELECT action FROM audit_log ORDER BY id", String.class))
            .containsExactly("card_imported", "card_imported", "cards_import_completed");
        assertThat(jdbc.queryForObject(
            "SELECT detail FROM audit_log WHERE action = 'card_imported' LIMIT 1",
            String.class)).contains("duration=24h0m0s");
    }

    @Test
    void exportUsesBomCrlfCsvEscapingAndFormulaHardening() {
        jdbc.update("""
            INSERT INTO cards (
              code, machine_id, agent_id, created_at, activated_at, expires_at,
              status, note, max_sessions
            ) VALUES (?, ?, '', ?, NULL, ?, 'active', ?, 2)
            """,
            "012345-6789AB-CDEFGH",
            "@machine",
            "2025-01-02T03:04:05Z",
            "2025-01-03T03:04:05Z",
            "=SUM(1,2)");

        String csv = new String(service.exportCsv(), StandardCharsets.UTF_8);

        assertThat(csv).startsWith("\uFEFFcode,status,note,max_sessions,machine_id,");
        assertThat(csv).contains("'=SUM(1,2)");
        assertThat(csv).contains("'@machine");
        assertThat(csv).contains("expired");
        assertThat(csv).contains("\r\n");
        assertThat(csv).endsWith("\r\n");
    }

    @Test
    void malformedQuotedCsvIsRejected() {
        assertThatThrownBy(() ->
            service.importCsv("\"unterminated", Duration.ofHours(1), 1, true))
            .isInstanceOf(IllegalArgumentException.class)
            .hasMessage("invalid CSV data");
        assertThatThrownBy(() ->
            service.importCsv("bad\"quote,note", Duration.ofHours(1), 1, true))
            .isInstanceOf(IllegalArgumentException.class)
            .hasMessage("invalid CSV data");
        assertThatThrownBy(() ->
            service.importCsv("\"closed\"tail,note", Duration.ofHours(1), 1, true))
            .isInstanceOf(IllegalArgumentException.class)
            .hasMessage("invalid CSV data");
    }

    @Test
    void auditFailureRollsBackImportedCard() {
        jdbc.execute("DROP TABLE audit_log");

        assertThatThrownBy(() -> service.importCsv(
            "0123456789ABCDEFGH,note", Duration.ofHours(1), 1, false))
            .isInstanceOf(DataAccessException.class);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM cards", Integer.class)).isZero();
    }

    private void createAuditTable() {
        jdbc.execute("""
            CREATE TABLE audit_log (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              time DATETIME NOT NULL,
              action TEXT NOT NULL,
              card TEXT NOT NULL DEFAULT '',
              detail TEXT NOT NULL DEFAULT ''
            )
            """);
    }
}
