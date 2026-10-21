package com.lingqiao.server.scripts;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ScriptModuleDocument {
    private String id = "";
    private String version = "";
    private String sha256 = "";
    private String content = "";
    private int size;
    private String note = "";
    private String status = "";

    @JsonProperty("created_at")
    private Instant createdAt;

    @JsonProperty("updated_at")
    private Instant updatedAt;

    @JsonProperty("activated_at")
    private Instant activatedAt;

    public String getId() { return id == null ? "" : id; }
    public void setId(String id) { this.id = id; }
    public String getVersion() { return version == null ? "" : version; }
    public void setVersion(String version) { this.version = version; }
    public String getSha256() { return sha256 == null ? "" : sha256; }
    public void setSha256(String sha256) { this.sha256 = sha256; }
    public String getContent() { return content == null ? "" : content; }
    public void setContent(String content) { this.content = content; }
    public int getSize() { return size; }
    public void setSize(int size) { this.size = size; }
    public String getNote() { return note == null ? "" : note; }
    public void setNote(String note) { this.note = note; }
    public String getStatus() { return status == null ? "" : status; }
    public void setStatus(String status) { this.status = status; }
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
    public Instant getActivatedAt() { return activatedAt; }
    public void setActivatedAt(Instant activatedAt) { this.activatedAt = activatedAt; }

    public ScriptModule toModule() {
        return new ScriptModule(
            getId(), getVersion(), getSha256(), getContent(), getSize(), getNote(), getStatus(),
            createdAt == null ? Instant.EPOCH : createdAt,
            updatedAt == null ? Instant.EPOCH : updatedAt,
            activatedAt
        );
    }
}
