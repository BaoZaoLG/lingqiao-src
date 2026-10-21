package com.lingqiao.server.cards;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class CardsServiceTest {
    private static final Instant NOW = Instant.parse("2026-07-01T00:00:00Z");

    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private CardsService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY, machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL,
              activated_at DATETIME, expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active', note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        service = new CardsService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void listReturnsNewestFirst() {
        insert("C-OLD", "", "", NOW.minusSeconds(100), NOW.plusSeconds(3600), "active", "", 1);
        insert("C-NEW", "", "", NOW.minusSeconds(10), NOW.plusSeconds(3600), "active", "", 1);

        List<Card> cards = service.list(filter(), NOW);

        assertThat(cards).extracting(Card::code).containsExactly("C-NEW", "C-OLD");
    }

    @Test
    void expiredCardReportsEffectiveStatusEvenWhenStoredActive() {
        insert("C1", "", "", NOW.minusSeconds(100), NOW.minusSeconds(1), "active", "", 1);

        Card card = service.list(filter(), NOW).get(0);

        assertThat(card.status()).isEqualTo("expired");
    }

    @Test
    void statusFilterMatchesEffectiveStatusNotStoredColumn() {
        insert("C-LIVE", "", "", NOW, NOW.plusSeconds(3600), "active", "", 1);
        insert("C-STALE", "", "", NOW, NOW.minusSeconds(1), "active", "", 1);

        List<Card> active = service.list(
            new CardListFilter(null, "active", null, null), NOW);
        List<Card> expired = service.list(
            new CardListFilter(null, "expired", null, null), NOW);

        assertThat(active).extracting(Card::code).containsExactly("C-LIVE");
        assertThat(expired).extracting(Card::code).containsExactly("C-STALE");
    }

    @Test
    void nonLifecycleStatusIsPushedIntoSql() {
        insert("C1", "", "", NOW, NOW.plusSeconds(3600), "disabled", "", 1);
        insert("C2", "", "", NOW, NOW.plusSeconds(3600), "active", "", 1);

        List<Card> cards = service.list(
            new CardListFilter(null, "disabled", null, null), NOW);

        assertThat(cards).extracting(Card::code).containsExactly("C1");
    }

    @Test
    void boundFilterSelectsMachineBoundCards() {
        insert("C-BOUND", "M1", "", NOW, NOW.plusSeconds(3600), "active", "", 1);
        insert("C-FREE", "", "", NOW, NOW.plusSeconds(3600), "active", "", 1);

        assertThat(service.list(withBound(true), NOW)).extracting(Card::code).containsExactly("C-BOUND");
        assertThat(service.list(withBound(false), NOW)).extracting(Card::code).containsExactly("C-FREE");
    }

    @Test
    void expiryWindowFiltersAreInclusiveLikeGo() {
        insert("C-EARLY", "", "", NOW, NOW.plusSeconds(60), "active", "", 1);
        insert("C-LATE", "", "", NOW, NOW.plusSeconds(6000), "active", "", 1);

        List<Card> before = service.list(new CardListFilter(
            null, null, null, null, null, NOW.plusSeconds(600), null, null), NOW);
        List<Card> after = service.list(new CardListFilter(
            null, null, null, null, null, null, NOW.plusSeconds(600), null), NOW);

        assertThat(before).extracting(Card::code).containsExactly("C-EARLY");
        assertThat(after).extracting(Card::code).containsExactly("C-LATE");
    }

    @Test
    void maxSessionsFilterIsExactMatch() {
        insert("C1", "", "", NOW, NOW.plusSeconds(3600), "active", "", 1);
        insert("C5", "", "", NOW, NOW.plusSeconds(3600), "active", "", 5);

        List<Card> cards = service.list(new CardListFilter(
            null, null, null, null, null, null, null, 5), NOW);

        assertThat(cards).extracting(Card::code).containsExactly("C5");
    }

    @Test
    void searchMatchesCodeOrNote() {
        insert("ABC123", "", "", NOW, NOW.plusSeconds(3600), "active", "", 1);
        insert("XYZ789", "", "", NOW, NOW.plusSeconds(3600), "active", "special", 1);

        assertThat(service.list(new CardListFilter("ABC", null, null, null), NOW))
            .extracting(Card::code).containsExactly("ABC123");
        assertThat(service.list(new CardListFilter("special", null, null, null), NOW))
            .extracting(Card::code).containsExactly("XYZ789");
    }

    @Test
    void agentAndMachineFiltersNarrowResults() {
        insert("C1", "M1", "A1", NOW, NOW.plusSeconds(3600), "active", "", 1);
        insert("C2", "M2", "A2", NOW, NOW.plusSeconds(3600), "active", "", 1);

        assertThat(service.list(new CardListFilter(null, null, null, "A2"), NOW))
            .extracting(Card::code).containsExactly("C2");
        assertThat(service.list(new CardListFilter(null, null, "M1", null), NOW))
            .extracting(Card::code).containsExactly("C1");
    }

    @Test
    void getThrowsForUnknownCode() {
        assertThatThrownBy(() -> service.get("missing"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("card not found");
    }

    @Test
    void sqlStatusSkipsLifecycleStatuses() {
        assertThat(new CardListFilter(null, "active", null, null).sqlStatus()).isNull();
        assertThat(new CardListFilter(null, "expired", null, null).sqlStatus()).isNull();
        assertThat(new CardListFilter(null, "disabled", null, null).sqlStatus()).isEqualTo("disabled");
        assertThat(new CardListFilter(null, null, null, null).sqlStatus()).isNull();
    }

    private CardListFilter filter() {
        return new CardListFilter(null, null, null, null);
    }

    private CardListFilter withBound(boolean bound) {
        return new CardListFilter(null, null, null, null, bound, null, null, null);
    }

    private void insert(String code, String machineId, String agentId, Instant createdAt,
                        Instant expiresAt, String status, String note, int maxSessions) {
        jdbc.update("""
            INSERT INTO cards (code, machine_id, agent_id, created_at, expires_at, status, note, max_sessions)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """, code, machineId, agentId, createdAt.toString(), expiresAt.toString(),
            status, note, maxSessions);
    }
}
