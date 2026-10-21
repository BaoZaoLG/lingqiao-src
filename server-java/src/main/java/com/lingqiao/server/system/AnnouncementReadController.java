package com.lingqiao.server.system;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/announcement")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "announcement-enabled", havingValue = "true")
public class AnnouncementReadController {
    private final Path dataDir;
    private final AdminAuthService adminAuthService;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    public AnnouncementReadController(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir,
        AdminAuthService adminAuthService
    ) {
        this.dataDir = Path.of(dataDir).toAbsolutePath().normalize();
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> get(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "announcement.read");
        AnnouncementIndex index = readIndex();
        AnnouncementDocument active = index.getAnnouncements().stream()
            .filter(a -> index.getActiveId().equals(a.getId()))
            .findFirst()
            .orElse(null);
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("announcement", active);
        body.put("announcements", index.getAnnouncements());
        body.put("active_id", index.getActiveId());
        return CompatResponse.ok(body);
    }

    private AnnouncementIndex readIndex() {
        Path path = dataDir.resolve("announcements").resolve("index.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid announcement index path");
        }
        if (!Files.exists(path)) {
            return new AnnouncementIndex();
        }
        try {
            return mapper.readValue(path.toFile(), AnnouncementIndex.class);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read announcement index");
        }
    }
}
