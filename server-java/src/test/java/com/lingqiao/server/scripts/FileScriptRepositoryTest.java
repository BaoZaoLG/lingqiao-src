package com.lingqiao.server.scripts;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class FileScriptRepositoryTest {
    @TempDir
    Path tempDir;

    @Test
    void listsScriptsAndResolvesActiveModule() throws Exception {
        Path scriptsDir = tempDir.resolve("scripts");
        Files.createDirectories(scriptsDir);
        Files.writeString(scriptsDir.resolve("index.json"), """
            {
              "active_id": "v1",
              "scripts": [
                {"id":"v1","version":"1.0.0","sha256":"abc","content":"console.log(1)","size":14,"status":"active","updated_at":"2026-07-01T00:00:00Z"},
                {"id":"v0","version":"0.9.0","sha256":"def","content":"console.log(0)","size":14,"status":"archived","updated_at":"2026-06-01T00:00:00Z"}
              ]
            }
            """);
        Files.writeString(scriptsDir.resolve("v1.json"), """
            {"id":"v1","version":"1.0.0","sha256":"abc","content":"console.log(1)","size":14,"status":"active","updated_at":"2026-07-01T00:00:00Z"}
            """);

        FileScriptRepository repository = new FileScriptRepository(tempDir);
        ScriptListResult result = repository.list();

        assertThat(result.activeId()).isEqualTo("v1");
        assertThat(result.scripts()).hasSize(2);
        assertThat(result.active().version()).isEqualTo("1.0.0");
        assertThat(repository.requireActive().content()).contains("console.log(1)");
    }

    @Test
    void rejectsTraversalLikeScriptIdentifiers() {
        FileScriptRepository repository = new FileScriptRepository(tempDir);
        org.assertj.core.api.Assertions.assertThatThrownBy(() -> repository.get(".."))
            .isInstanceOf(com.lingqiao.server.common.errors.SidecarException.class)
            .hasMessageContaining("invalid script id");
    }

    @Test
    void preservesIndexMetadataWhenHydratingContentFromModuleFile() throws Exception {
        Path scriptsDir = tempDir.resolve("scripts");
        Files.createDirectories(scriptsDir);
        Files.writeString(scriptsDir.resolve("index.json"), """
            {
              "active_id": "",
              "scripts": [
                {
                  "id":"v0",
                  "version":"0.9.0",
                  "sha256":"",
                  "content":"",
                  "size":0,
                  "note":"authoritative index note",
                  "status":"archived",
                  "created_at":"2026-06-01T00:00:00Z",
                  "updated_at":"2026-07-01T00:00:00Z",
                  "activated_at":"2026-06-15T00:00:00Z"
                }
              ]
            }
            """);
        Files.writeString(scriptsDir.resolve("v0.json"), """
            {
              "id":"v0",
              "version":"0.9.0",
              "sha256":"payload-sha",
              "content":"console.log('payload')",
              "size":22,
              "note":"stale module note",
              "status":"active",
              "updated_at":"2026-06-01T00:00:00Z"
            }
            """);

        ScriptModule module = new FileScriptRepository(tempDir).list().scripts().get(0);

        assertThat(module.content()).isEqualTo("console.log('payload')");
        assertThat(module.sha256()).isEqualTo("payload-sha");
        assertThat(module.size()).isEqualTo(22);
        assertThat(module.note()).isEqualTo("authoritative index note");
        assertThat(module.status()).isEqualTo("archived");
        assertThat(module.updatedAt()).isEqualTo("2026-07-01T00:00:00Z");
        assertThat(module.activatedAt()).isEqualTo("2026-06-15T00:00:00Z");
    }
}
