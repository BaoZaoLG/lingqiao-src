package com.lingqiao.server.chat;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ChatDocument {
    private long id;
    private String type = "";
    @JsonProperty("author_id") private String authorId = "";
    private String author = "";
    private String content = "";
    @JsonProperty("created_at") private Instant createdAt;
    @JsonProperty("reply_to_id") private long replyToId;
    private Map<String, Integer> reactions = new LinkedHashMap<>();
    @JsonProperty("reaction_users") private Map<String, List<String>> reactionUsers = new LinkedHashMap<>();
    private boolean deleted;

    public long getId() { return id; }
    public void setId(long id) { this.id = id; }
    public String getType() { return type; }
    public void setType(String type) { this.type = type; }
    public String getAuthorId() { return authorId; }
    public void setAuthorId(String authorId) { this.authorId = authorId; }
    public String getAuthor() { return author; }
    public void setAuthor(String author) { this.author = author; }
    public String getContent() { return content; }
    public void setContent(String content) { this.content = content; }
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public long getReplyToId() { return replyToId; }
    public void setReplyToId(long replyToId) { this.replyToId = replyToId; }
    public Map<String, Integer> getReactions() { return reactions; }
    public void setReactions(Map<String, Integer> reactions) { this.reactions = reactions; }
    public Map<String, List<String>> getReactionUsers() {
        return reactionUsers == null ? new LinkedHashMap<>() : reactionUsers;
    }
    public void setReactionUsers(Map<String, List<String>> reactionUsers) {
        this.reactionUsers = reactionUsers;
    }
    public boolean isDeleted() { return deleted; }
    public void setDeleted(boolean deleted) { this.deleted = deleted; }
}
