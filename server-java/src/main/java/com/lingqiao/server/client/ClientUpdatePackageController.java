package com.lingqiao.server.client;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import com.lingqiao.server.releases.PackagePathResolver;
import com.lingqiao.server.releases.ReleasePackage;
import com.lingqiao.server.releases.ReleaseSelectionStore;
import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.core.io.FileSystemResource;
import org.springframework.core.io.Resource;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Update package download mirroring Go HandleUpdatePackageDownload:
 * HMAC over request path, bound session validation, package id validation,
 * and data-directory containment before streaming the file.
 */
@RestController
@RequestMapping("/java/api/v1/update/package")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-update-enabled", havingValue = "true")
public class ClientUpdatePackageController {
    private final ReleaseSelectionStore store;
    private final PackagePathResolver paths;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;

    public ClientUpdatePackageController(ReleaseSelectionStore store,
                                         PackagePathResolver paths,
                                         ClientSessionValidator sessions,
                                         HmacAuthService hmac) {
        this.store = store;
        this.paths = paths;
        this.sessions = sessions;
        this.hmac = hmac;
    }

    @GetMapping("/{packageId}")
    public ResponseEntity<Resource> download(HttpServletRequest request, @PathVariable String packageId) {
        String goPath = request.getRequestURI().replaceFirst("^/java", "");
        hmac.verify(
            request.getHeader("X-Client-ID"),
            goPath,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        String token = request.getHeader("X-Session-Token");
        if (token == null || token.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "session_token is required");
        }
        try {
            sessions.validateBound(
                token,
                request.getHeader("X-Machine-ID"),
                request.getHeader("X-Card-Code"));
        } catch (SidecarException ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, ex.getMessage());
        }

        PackagePathResolver.validatePackageId(packageId);
        ReleasePackage pkg = store.requirePackage(packageId);
        Path file = paths.resolve(pkg.path());
        if (!Files.isRegularFile(file)) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "package file not found");
        }

        long length;
        try {
            length = Files.size(file);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "package stat failed");
        }

        return ResponseEntity.ok()
            .contentType(MediaType.APPLICATION_OCTET_STREAM)
            .contentLength(length)
            .header(HttpHeaders.CONTENT_DISPOSITION,
                "attachment; filename=\"" + sanitizeFilename(pkg.filename()) + "\"")
            .header("X-Update-Package-ID", pkg.id())
            .header("X-Update-SHA256", pkg.sha256())
            .body(new FileSystemResource(file));
    }

    private static String sanitizeFilename(String filename) {
        String value = filename == null || filename.isBlank() ? "update.bin" : filename;
        return value.replace("\"", "").replace("\r", "").replace("\n", "");
    }
}
