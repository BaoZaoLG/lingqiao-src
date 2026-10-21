package com.lingqiao.server.scripts;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Path;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class ScriptsWriteServiceTest {
    @TempDir
    Path tempDir;

    private ScriptsWriteService writer;
    private FileScriptRepository reader;

    @BeforeEach
    void setUp() {
        writer = new ScriptsWriteService(tempDir);
        reader = new FileScriptRepository(tempDir);
    }

    @Test
    void createStoresDraftWithSha256AndSize() {
        ScriptModule module = writer.create("1.0.0", "  console.log(1)  ", " note ");

        assertThat(module.status()).isEqualTo(ScriptsWriteService.STATUS_DRAFT);
        assertThat(module.content()).isEqualTo("console.log(1)");
        assertThat(module.sha256()).isEqualTo(ScriptsWriteService.sha256Hex("console.log(1)"));
        assertThat(module.size()).isEqualTo("console.log(1)".length());
        assertThat(module.note()).isEqualTo("note");

        ScriptListResult listed = reader.list();
        assertThat(listed.scripts()).hasSize(1);
        assertThat(listed.activeId()).isEmpty();
    }

    @Test
    void publishActivatesNewVersionAndArchivesPrevious() {
        ScriptModule first = writer.publish("1.0.0", "console.log(1)", "");
        ScriptModule second = writer.publish("2.0.0", "console.log(2)", "");

        ScriptListResult listed = reader.list();
        assertThat(listed.activeId()).isEqualTo(second.id());
        assertThat(reader.require(second.id()).status()).isEqualTo(ScriptsWriteService.STATUS_ACTIVE);
        assertThat(reader.require(first.id()).status()).isEqualTo(ScriptsWriteService.STATUS_ARCHIVED);
        assertThat(reader.requireActive().content()).isEqualTo("console.log(2)");
    }

    @Test
    void activateSwitchesActivePointerAndRejectsDoubleActivate() {
        ScriptModule active = writer.publish("1.0.0", "console.log(1)", "");
        ScriptModule draft = writer.create("2.0.0", "console.log(2)", "");

        ScriptModule activated = writer.activate(draft.id());

        assertThat(activated.status()).isEqualTo(ScriptsWriteService.STATUS_ACTIVE);
        assertThat(reader.list().activeId()).isEqualTo(draft.id());
        assertThat(reader.require(active.id()).status()).isEqualTo(ScriptsWriteService.STATUS_ARCHIVED);

        assertThatThrownBy(() -> writer.activate(draft.id()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("already active");
    }

    @Test
    void deleteRemovesInactiveOnly() {
        ScriptModule active = writer.publish("1.0.0", "console.log(1)", "");
        ScriptModule draft = writer.create("2.0.0", "console.log(2)", "");

        assertThatThrownBy(() -> writer.delete(active.id()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("cannot delete active script");

        writer.delete(draft.id());
        assertThat(reader.list().scripts()).hasSize(1);
        assertThatThrownBy(() -> writer.delete(draft.id()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("script not found");
    }

    @Test
    void rejectsEmptyAndOversizedContent() {
        assertThatThrownBy(() -> writer.create("1.0.0", "   ", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("content is required");
        assertThatThrownBy(() -> writer.create("1.0.0", "x".repeat(ScriptsWriteService.MAX_SCRIPT_BYTES + 1), ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("exceeds");
    }

    @Test
    void duplicateVersionsGetUniqueSuffix() {
        ScriptModule first = writer.create("1.0.0", "console.log(1)", "");
        ScriptModule second = writer.create("1.0.0", "console.log(2)", "");

        assertThat(first.version()).isEqualTo("1.0.0");
        assertThat(second.version()).startsWith("1.0.0-");
        assertThat(second.id()).isNotEqualTo(first.id());
    }
}
