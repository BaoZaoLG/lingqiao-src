package com.lingqiao.server.system;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.chat.ChatState;
import org.junit.jupiter.api.Test;

class ReadOnlyModuleDocumentsTest {
    private final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    @Test
    void parsesAnnouncementAndChatCompatibilityDocuments() throws Exception {
        AnnouncementIndex announcement = mapper.readValue("""
            {"active_id":"a1","announcements":[{"id":"a1","content":"hello","status":"published","updated_at":"2026-07-01T00:00:00Z"}]}
            """, AnnouncementIndex.class);
        ChatState chat = mapper.readValue("""
            {"next_id":2,"messages":[{"id":1,"type":"user","author":"U","content":"hello","created_at":"2026-07-01T00:00:00Z"}]}
            """, ChatState.class);

        assertThat(announcement.getActiveId()).isEqualTo("a1");
        assertThat(announcement.getAnnouncements()).hasSize(1);
        assertThat(chat.getMessages()).hasSize(1);
        assertThat(chat.getMessages().get(0).getContent()).isEqualTo("hello");
    }
}
