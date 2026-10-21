package com.lingqiao.server.web;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Locale;
import java.util.Optional;

/**
 * Serves the built web panels from disk, mirroring Go's embedded web-dist tree:
 * /admin/** -> web-dist/admin, /agent/** -> web-dist/agent, /assets/** -> web-dist/assets.
 * SPA routes fall back to the panel's index.html.
 */
public class WebAssetResolver {
    private final Path root;

    public WebAssetResolver(Path webDistRoot) {
        this.root = webDistRoot.toAbsolutePath().normalize();
    }

    public boolean available() {
        return Files.isDirectory(root);
    }

    public Optional<ResolvedAsset> resolve(String panel, String relativePath, boolean spaFallback) {
        Path base = safeChild(root, panel);
        String cleaned = normalizeRelative(relativePath);
        if (!cleaned.isEmpty()) {
            Path candidate = safeChild(base, cleaned);
            if (Files.isRegularFile(candidate)) {
                return Optional.of(new ResolvedAsset(candidate, contentTypeOf(candidate)));
            }
        }
        if (!spaFallback) {
            return Optional.empty();
        }
        Path index = base.resolve("index.html");
        if (Files.isRegularFile(index)) {
            return Optional.of(new ResolvedAsset(index, "text/html;charset=UTF-8"));
        }
        return Optional.empty();
    }

    static String normalizeRelative(String relativePath) {
        String value = relativePath == null ? "" : relativePath.trim();
        while (value.startsWith("/")) {
            value = value.substring(1);
        }
        if (value.isEmpty() || value.equals("/")) {
            return "";
        }
        if (value.contains("..") || value.contains("\\") || value.contains("\0")) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid asset path");
        }
        return value;
    }

    private Path safeChild(Path base, String relative) {
        Path resolved = base.resolve(relative).toAbsolutePath().normalize();
        if (!resolved.startsWith(base)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid asset path");
        }
        return resolved;
    }

    static String contentTypeOf(Path path) {
        String name = path.getFileName().toString().toLowerCase(Locale.ROOT);
        if (name.endsWith(".html") || name.endsWith(".htm")) {
            return "text/html;charset=UTF-8";
        }
        if (name.endsWith(".js") || name.endsWith(".mjs")) {
            return "text/javascript;charset=UTF-8";
        }
        if (name.endsWith(".css")) {
            return "text/css;charset=UTF-8";
        }
        if (name.endsWith(".json")) {
            return "application/json;charset=UTF-8";
        }
        if (name.endsWith(".svg")) {
            return "image/svg+xml";
        }
        if (name.endsWith(".png")) {
            return "image/png";
        }
        if (name.endsWith(".jpg") || name.endsWith(".jpeg")) {
            return "image/jpeg";
        }
        if (name.endsWith(".ico")) {
            return "image/x-icon";
        }
        if (name.endsWith(".woff2")) {
            return "font/woff2";
        }
        if (name.endsWith(".woff")) {
            return "font/woff";
        }
        if (name.endsWith(".map")) {
            return "application/json;charset=UTF-8";
        }
        try {
            String probed = Files.probeContentType(path);
            return probed == null ? "application/octet-stream" : probed;
        } catch (IOException ex) {
            return "application/octet-stream";
        }
    }

    public record ResolvedAsset(Path path, String contentType) {
    }
}
