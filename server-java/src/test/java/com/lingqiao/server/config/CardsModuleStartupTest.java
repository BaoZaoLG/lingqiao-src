package com.lingqiao.server.config;

import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.cards.CardsController;
import com.lingqiao.server.cards.CardsCsvService;
import com.lingqiao.server.cards.CardsWriteController;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.TestPropertySource;

@SpringBootTest
@TestPropertySource(properties = {
    "lingqiao.sidecar.cards-enabled=true",
    "lingqiao.sidecar.cards-write-enabled=true",
    "lingqiao.write.db-url=jdbc:sqlite:file:memdb-cards-start?mode=memory&cache=shared"
})
class CardsModuleStartupTest {
    @Autowired
    private CardsController readController;

    @Autowired
    private CardsWriteController writeController;

    @Autowired
    private CardsCsvService csvService;

    @Test
    void cardReadWriteAndCsvRoutesStartTogether() {
        assertThat(readController).isNotNull();
        assertThat(writeController).isNotNull();
        assertThat(csvService).isNotNull();
    }
}
