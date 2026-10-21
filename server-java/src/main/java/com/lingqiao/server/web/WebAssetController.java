package com.lingqiao.server.web;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import org.springframework.core.io.FileSystemResource;
import org.springframework.core.io.Resource;
import org.springframework.http.HttpHeaders;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Static panel hosting mirroring Go's file servers for /admin/, /agent/ and /assets/.
 * Authentication is intentionally not required: the Go build serves these files
 * publicly and the panels authenticate through the admin API.
 */
@RestController
@RequestMapping("/java/web")
public class WebAssetController {
    private final WebAssetResolver resolver;

    public WebAssetController(WebAssetResolver resolver) {
        this.resolver = resolver;
    }

    @GetMapping({"/admin", "/admin/**"})
    public ResponseEntity<Resource> admin(jakarta.servlet.http.HttpServletRequest request) {
        return serve("admin", suffixAfter(request, "/java/web/admin"), true);
    }

    @GetMapping({"/agent", "/agent/**"})
    public ResponseEntity<Resource> agent(jakarta.servlet.http.HttpServletRequest request) {
        return serve("agent", suffixAfter(request, "/java/web/agent"), true);
    }

    @GetMapping("/assets/**")
    public ResponseEntity<Resource> assets(jakarta.servlet.http.HttpServletRequest request) {
        return serve("assets", suffixAfter(request, "/java/web/assets"), false);
    }

    private ResponseEntity<Resource> serve(String panel, String relativePath, boolean spaFallback) {
        if (!resolver.available()) {
            throw new SidecarException(ErrorCode.FEATURE_DISABLED, "web assets unavailable");
        }
        WebAssetResolver.ResolvedAsset asset = resolver.resolve(panel, relativePath, spaFallback)
            .orElseThrow(() -> new SidecarException(ErrorCode.NOT_FOUND, "asset not found"));

        long length;
        try {
            length = Files.size(asset.path());
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "asset not found");
        }
        return ResponseEntity.ok()
            .header(HttpHeaders.CONTENT_TYPE, asset.contentType())
            .header(HttpHeaders.CACHE_CONTROL, cacheControlFor(panel))
            .contentLength(length)
            .body(new FileSystemResource(asset.path()));
    }

    private static String cacheControlFor(String panel) {
        return "assets".equals(panel) ? "public, max-age=31536000, immutable" : "no-cache";
    }

    private static String suffixAfter(jakarta.servlet.http.HttpServletRequest request, String prefix) {
        String uri = request.getRequestURI();
        return uri.length() > prefix.length() ? uri.substring(prefix.length()) : "";
    }
}
