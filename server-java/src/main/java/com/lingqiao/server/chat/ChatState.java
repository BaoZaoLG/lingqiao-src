package com.lingqiao.server.chat;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ChatState {
    @JsonProperty("next_id") private long nextId;
    @JsonProperty("messages") private List<ChatDocument> messages = new ArrayList<>();
    @JsonProperty("profiles") private Map<String, Object> profiles = new LinkedHashMap<>();
    @JsonProperty("mutes") private Map<String, ChatMuteDocument> mutes = new LinkedHashMap<>();

    public long getNextId() { return nextId; }
    public void setNextId(long nextId) { this.nextId = nextId; }

    public List<ChatDocument> getMessages() {
        return messages == null ? new ArrayList<>() : messages;
    }

    public void setMessages(List<ChatDocument> messages) { this.messages = messages; }

    public Map<String, Object> getProfiles() {
        return profiles == null ? new LinkedHashMap<>() : profiles;
    }

    public void setProfiles(Map<String, Object> profiles) { this.profiles = profiles; }

    public Map<String, ChatMuteDocument> getMutes() {
        return mutes == null ? new LinkedHashMap<>() : mutes;
    }

    public void setMutes(Map<String, ChatMuteDocument> mutes) { this.mutes = mutes; }
}
