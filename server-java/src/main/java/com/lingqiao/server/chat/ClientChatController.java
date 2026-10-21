package com.lingqiao.server.chat;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.client.ClientSessionValidator;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.HmacAuthService;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Client chat endpoints mirroring Go /api/v1/chat/{messages,send,profile,presence,react}.
 * Every request is HMAC signed over the raw body and bound to an active session.
 */
@RestController
@RequestMapping("/java/api/v1/chat")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-chat-enabled", havingValue = "true")
public class ClientChatController {
    private final ClientChatService chat;
    private final ClientSessionValidator sessions;
    private final HmacAuthService hmac;
    private final ObjectMapper mapper = new ObjectMapper();

    public ClientChatController(ClientChatService chat,
                                ClientSessionValidator sessions,
                                HmacAuthService hmac) {
        this.chat = chat;
        this.sessions = sessions;
        this.hmac = hmac;
    }

    @PostMapping(path = "/messages", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> messages(HttpServletRequest request, @RequestBody String rawBody) {
        ChatRequest req = authorize(request, rawBody);
        List<ClientChatService.ChatMessageView> messages = chat.list(identity(req), req.after_id());
        return CompatResponse.ok(Map.of("messages", messages));
    }

    @PostMapping(path = "/send", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> send(HttpServletRequest request, @RequestBody String rawBody) {
        ChatRequest req = authorize(request, rawBody);
        ClientChatService.ChatMessageView message =
            chat.add(identity(req), req.content(), req.reply_to_id());
        return CompatResponse.ok(Map.of("message", message));
    }

    @PostMapping(path = "/profile", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> profile(HttpServletRequest request, @RequestBody String rawBody) {
        ChatRequest req = authorize(request, rawBody);
        ClientChatService.ClientIdentity identity = identity(req);
        ClientChatService.ChatProfileView profile = req.nickname() == null || req.nickname().isBlank()
            ? chat.profile(identity)
            : chat.setProfile(identity, req.nickname());

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("author_id", profile.authorId());
        body.put("nickname", profile.nickname());
        return CompatResponse.ok(Map.of("profile", body));
    }

    @PostMapping(path = "/presence", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> presence(HttpServletRequest request, @RequestBody String rawBody) {
        ChatRequest req = authorize(request, rawBody);
        return CompatResponse.ok(Map.of("online", chat.touchPresence(identity(req))));
    }

    @PostMapping(path = "/react", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> react(HttpServletRequest request, @RequestBody String rawBody) {
        ChatRequest req = authorize(request, rawBody);
        ClientChatService.ChatMessageView message =
            chat.react(identity(req), req.message_id(), req.reaction());
        return CompatResponse.ok(Map.of("message", message));
    }

    private ChatRequest authorize(HttpServletRequest request, String rawBody) {
        hmac.verify(
            request.getHeader("X-Client-ID"),
            rawBody,
            request.getHeader("X-HMAC-Signature"),
            request.getHeader("X-Timestamp"),
            request.getHeader("X-Nonce"));

        ChatRequest req = parse(rawBody);
        ClientSessionValidator.ValidatedSession session =
            sessions.validateBound(req.session_token(), req.machine_id(), req.card());
        return req.withResolvedIdentity(session.cardCode(), session.machineId());
    }

    private static ClientChatService.ClientIdentity identity(ChatRequest req) {
        return new ClientChatService.ClientIdentity(
            req.session_token(), ChatIdentity.authorId(req.card(), req.machine_id()));
    }

    private ChatRequest parse(String rawBody) {
        try {
            ChatRequest parsed = mapper.readValue(rawBody, ChatRequest.class);
            if (parsed == null) {
                throw new SidecarException(ErrorCode.INVALID_JSON, "invalid json");
            }
            return parsed;
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "invalid json");
        }
    }

    /**
     * Unknown fields are ignored, matching Go's encoding/json. The client sends
     * client_id in every chat body and the Go handler simply drops it; without
     * this, Jackson's default FAIL_ON_UNKNOWN_PROPERTIES=true rejected the same
     * payload and the generic catch in parse() reported it as INVALID_JSON — so
     * presence/profile/messages all failed with a misleading "invalid json"
     * once the cutover routed chat here.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public record ChatRequest(
        String session_token,
        String machine_id,
        String card,
        String content,
        String nickname,
        String reaction,
        long message_id,
        long reply_to_id,
        long after_id
    ) {
        /**
         * The chat author identity must come from the server-side session, never
         * from client-supplied fields, matching Go's use of the resolved session.
         */
        ChatRequest withResolvedIdentity(String cardCode, String machineId) {
            return new ChatRequest(session_token, machineId, cardCode, content, nickname,
                reaction, message_id, reply_to_id, after_id);
        }
    }
}
