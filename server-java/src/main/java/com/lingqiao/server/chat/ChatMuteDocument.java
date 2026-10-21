package com.lingqiao.server.chat;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ChatMuteDocument {
    @JsonProperty("author_id") private String authorId = "";
    private String reason = "";
    private Instant expires;

    public String getAuthorId() { return authorId == null ? "" : authorId; }
    public void setAuthorId(String authorId) { this.authorId = authorId; }
    public String getReason() { return reason == null ? "" : reason; }
    public void setReason(String reason) { this.reason = reason; }
    public Instant getExpires() { return expires; }
    public void setExpires(Instant expires) { this.expires = expires; }
}
