package com.lingqiao.server.chat;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/chat")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "chat-write-enabled", havingValue = "true")
public class ChatWriteController {
    private final ChatWriteService service;
    private final AdminAuthService adminAuthService;

    public ChatWriteController(ChatWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/delete")
    public Map<String, Object> delete(HttpServletRequest request, @RequestBody DeleteRequest body) {
        requireWrite(request);
        service.deleteMessage(body.id());
        return CompatResponse.ok((Map<String, ?>) null);
    }

    @PostMapping("/system")
    public Map<String, Object> system(HttpServletRequest request, @RequestBody SystemRequest body) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("message", service.addSystemMessage(body.content())));
    }

    @PostMapping("/mute")
    public Map<String, Object> mute(HttpServletRequest request, @RequestBody MuteRequest body) {
        requireWrite(request);
        return CompatResponse.ok(Map.of(
            "mute", service.mute(body.author_id(), body.duration_minutes(), body.reason())));
    }

    private void requireWrite(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "chat.write");
    }

    public record DeleteRequest(long id) {
    }

    public record SystemRequest(String content) {
    }

    public record MuteRequest(String author_id, int duration_minutes, String reason) {
    }
}
