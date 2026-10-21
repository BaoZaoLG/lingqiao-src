package com.lingqiao.server.chat;

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
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/chat/messages")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "chat-enabled", havingValue = "true")
public class ChatReadController {
    private final Path dataDir;
    private final AdminAuthService adminAuthService;
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    public ChatReadController(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir,
        AdminAuthService adminAuthService
    ) {
        this.dataDir = Path.of(dataDir).toAbsolutePath().normalize();
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "chat.read");
        ChatState state = readState();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("messages", ChatWriteService.prune(
            new ArrayList<>(state.getMessages()), Instant.now()));
        return CompatResponse.ok(body);
    }

    private ChatState readState() {
        Path path = dataDir.resolve("chat.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid chat state path");
        }
        if (!Files.exists(path)) {
            return new ChatState();
        }
        try {
            return mapper.readValue(path.toFile(), ChatState.class);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read chat state");
        }
    }
}
