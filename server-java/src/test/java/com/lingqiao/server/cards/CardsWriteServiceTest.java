package com.lingqiao.server.cards;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Duration;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class CardsWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private CardsWriteService writer;
    private CardsService reader;

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
        reader = new CardsService(jdbc);
        writer = new CardsWriteService(jdbc, reader);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void createProducesCanonicalCardCodeAndDefaults() {
        Card card = writer.create(Duration.ofHours(24), "note", 2, "agent-1");

        assertThat(CardCodes.format(card.code())).isEqualTo(card.code());
        assertThat(CardCodes.validateImport(card.code())).isPresent();
        assertThat(card.status()).isEqualTo(CardStatuses.ACTIVE);
        assertThat(card.maxSessions()).isEqualTo(2);
        assertThat(card.machineId()).isEmpty();
        assertThat(card.activatedAt()).isNull();
    }

    @Test
    void batchCreateInsertsAllRequestedCards() {
        List<Card> cards = writer.batchCreate(3, Duration.ofHours(1), "batch", 1, "");
        assertThat(cards).hasSize(3);
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM cards", Integer.class)).isEqualTo(3);
    }

    @Test
    void statusExtendUnbindAndDeleteMirrorGoSemantics() {
        Card card = writer.create(Duration.ofHours(1), "", 1, "");
        String code = card.code();

        writer.updateStatus(code, CardStatuses.DISABLED);
        assertThat(reader.get(code).status()).isEqualTo(CardStatuses.DISABLED);

        writer.updateStatus(code, CardStatuses.EXPIRED);
        writer.extend(code, 2);
        Card extended = reader.get(code);
        assertThat(extended.status()).isEqualTo(CardStatuses.ACTIVE);
        assertThat(extended.expiresAt()).isAfter(card.expiresAt());

        jdbc.update("UPDATE cards SET machine_id = 'M1', activated_at = expires_at WHERE code = ?", code);
        writer.unbind(code);
        Card unbound = reader.get(code);
        assertThat(unbound.machineId()).isEmpty();
        assertThat(unbound.activatedAt()).isNull();

        writer.updateDetails(code, "updated", 5);
        Card detailed = reader.get(code);
        assertThat(detailed.note()).isEqualTo("updated");
        assertThat(detailed.maxSessions()).isEqualTo(5);

        writer.delete(code);
        assertThatThrownBy(() -> reader.get(code)).isInstanceOf(SidecarException.class);
    }

    @Test
    void rejectsInvalidInputs() {
        assertThatThrownBy(() -> writer.create(Duration.ZERO, "", 1, ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("duration must be positive");
        assertThatThrownBy(() -> writer.create(Duration.ofHours(1), "", 0, ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("max sessions");
        assertThatThrownBy(() -> writer.updateStatus("MISSING-CODE-0001", "bogus"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid card status");
        assertThatThrownBy(() -> writer.extend("MISSING", 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("extendHours must be positive");
    }

    @Test
    void createWithCodeUsesCanonicalImportedCode() {
        Card card = writer.createWithCode(
            "0123456789abcdefgh", Duration.ofHours(2), "imported", 3, "");

        assertThat(card.code()).isEqualTo("012345-6789AB-CDEFGH");
        assertThat(card.note()).isEqualTo("imported");
        assertThat(card.maxSessions()).isEqualTo(3);
    }

    @Test
    void bulkDryRunReportsChangedUnchangedAndMissingCards() {
        Card active = writer.create(Duration.ofHours(1), "", 1, "");
        writer.updateStatus(active.code(), CardStatuses.DISABLED);

        CardsWriteService.BulkResult result = writer.bulkUpdateDetailed(
            List.of(active.code(), "MISSING"), "disable", 0, true);

        assertThat(result.updated()).isZero();
        assertThat(result.skipped()).isEqualTo(2);
        assertThat(result.failed()).isZero();
        assertThat(result.items()).extracting(CardsWriteService.BulkItem::status)
            .containsExactly("skipped", "skipped");
        assertThat(reader.get(active.code()).status()).isEqualTo(CardStatuses.DISABLED);
    }

    @Test
    void bulkExtendDefaultsToGoThirtyDayDurationAndReactivatesExpiredCard() {
        Card card = writer.create(Duration.ofHours(1), "", 1, "");
        writer.updateStatus(card.code(), CardStatuses.EXPIRED);

        CardsWriteService.BulkResult result =
            writer.bulkUpdateDetailed(List.of(card.code()), "extend", 0, false);

        Card extended = reader.get(card.code());
        assertThat(result.updated()).isEqualTo(1);
        assertThat(extended.status()).isEqualTo(CardStatuses.ACTIVE);
        assertThat(Duration.between(card.expiresAt(), extended.expiresAt()))
            .isEqualTo(Duration.ofHours(720));
    }
}
