package com.lingqiao.server.contract;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;

/**
 * Verifies every Java admin controller checks the same permission Go registers
 * for the corresponding route. A mismatch would silently widen or narrow admin
 * access the moment a module is cut over.
 */
class PermissionParityTest {
    private static final Pattern REQUIRE_ADMIN =
        Pattern.compile("requireAdmin\\([^;]*?\"([a-z_]+\\.[a-z_]+)\"", Pattern.DOTALL);

    @Test
    void everyPermissionUsedByJavaExistsInTheSharedManifest() throws IOException {
        Map<String, Boolean> manifestPermissions = manifestPermissions();
        List<String> unknown = new ArrayList<>();

        for (Map.Entry<String, List<String>> entry : javaPermissionsByFile().entrySet()) {
            for (String permission : entry.getValue()) {
                if (!manifestPermissions.containsKey(permission)) {
                    unknown.add(entry.getKey() + " -> " + permission);
                }
            }
        }

        assertThat(unknown)
            .as("permissions not present in java-cutover-contracts.json: %s", unknown)
            .isEmpty();
    }

    @Test
    void manifestDeclaresAPermissionForEveryAdminRoute() throws IOException {
        JsonNode routes = manifest().path("routes");
        List<String> missing = new ArrayList<>();

        for (JsonNode route : routes) {
            String goPath = route.path("go_path").asText();
            if (!goPath.startsWith("/admin/")) {
                continue;
            }
            String auth = route.path("auth").asText("");
            if (route.path("permission").asText("").isEmpty()
                && !auth.equals("admin") && !auth.equals("public")) {
                missing.add(goPath);
            }
        }

        assertThat(missing).as("admin routes without a permission: %s", missing).isEmpty();
    }

    @Test
    void releasesListUsesGoWritePermission() throws IOException {
        // Go registers GET /admin/api/releases under releases.write, not releases.read,
        // so release_manager keeps access but operator does not gain it.
        assertThat(permissionFor("/admin/api/releases")).isEqualTo("releases.write");
        assertThat(permissionsOf("ReleasesController.java")).containsOnly("releases.write");
    }

    @Test
    void scriptsAndPayloadsListsUseWritePermissionLikeGo() throws IOException {
        assertThat(permissionFor("/admin/api/scripts")).isEqualTo("scripts.write");
        assertThat(permissionFor("/admin/api/payloads")).isEqualTo("payloads.write");
        assertThat(permissionsOf("ScriptsReadController.java")).containsOnly("scripts.write");
        assertThat(permissionsOf("PayloadsController.java")).containsOnly("payloads.write");
    }

    @Test
    void blacklistUsesSystemReadForBothDirectionsLikeGo() throws IOException {
        // Go guards the combined GET/POST handler with system.read only.
        assertThat(permissionFor("/admin/api/blacklist")).isEqualTo("system.read");
        assertThat(permissionsOf("BlacklistReadController.java")).containsOnly("system.read");
        assertThat(permissionsOf("BlacklistWriteController.java")).containsOnly("system.read");
    }

    private static String permissionFor(String goPath) throws IOException {
        for (JsonNode route : manifest().path("routes")) {
            if (route.path("go_path").asText().equals(goPath)) {
                return route.path("permission").asText("");
            }
        }
        return "";
    }

    private static List<String> permissionsOf(String fileName) throws IOException {
        return javaPermissionsByFile().getOrDefault(fileName, List.of());
    }

    private static Map<String, Boolean> manifestPermissions() throws IOException {
        Map<String, Boolean> permissions = new HashMap<>();
        for (JsonNode route : manifest().path("routes")) {
            String permission = route.path("permission").asText("");
            if (!permission.isEmpty()) {
                permissions.put(permission, Boolean.TRUE);
            }
        }
        // Admin-only endpoints that have no cutover route still need their permission.
        permissions.put("admins.write", Boolean.TRUE);
        permissions.put("cards.bulk", Boolean.TRUE);
        permissions.put("invites.write", Boolean.TRUE);
        permissions.put("announcement.write", Boolean.TRUE);
        return permissions;
    }

    private static Map<String, List<String>> javaPermissionsByFile() throws IOException {
        Map<String, List<String>> byFile = new HashMap<>();
        Path root = Path.of("src", "main", "java", "com", "lingqiao", "server");
        try (Stream<Path> files = Files.walk(root)) {
            for (Path file : files.filter(p -> p.toString().endsWith(".java")).toList()) {
                String source = Files.readString(file, StandardCharsets.UTF_8);
                Matcher matcher = REQUIRE_ADMIN.matcher(source);
                List<String> found = new ArrayList<>();
                while (matcher.find()) {
                    found.add(matcher.group(1));
                }
                if (!found.isEmpty()) {
                    byFile.put(file.getFileName().toString(), found);
                }
            }
        }
        return byFile;
    }

    private static JsonNode manifest() throws IOException {
        Path path = Path.of("..", "docs", "api", "java-cutover-contracts.json");
        return new ObjectMapper().readTree(Files.readString(path));
    }
}
