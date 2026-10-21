package com.lingqiao.server.payloads;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

@JsonIgnoreProperties(ignoreUnknown = true)
public class PayloadDocument {
    @JsonProperty("payload_id") private String payloadId = "";
    @JsonProperty("exe_hash") private String exeHash = "";
    @JsonProperty("chunk_count") private int chunkCount;
    @JsonProperty("chunk_size") private int chunkSize;
    @JsonProperty("total_size") private int totalSize;
    @JsonProperty("created_at") private Instant createdAt;
    @JsonProperty("aes_key") private String aesKey = "";
    @JsonProperty("hmac_key") private String hmacKey = "";
    @JsonProperty("iv") private String iv = "";

    public String getPayloadId() { return payloadId == null ? "" : payloadId; }
    public void setPayloadId(String payloadId) { this.payloadId = payloadId; }
    public String getExeHash() { return exeHash == null ? "" : exeHash; }
    public void setExeHash(String exeHash) { this.exeHash = exeHash; }
    public int getChunkCount() { return chunkCount; }
    public void setChunkCount(int chunkCount) { this.chunkCount = chunkCount; }
    public int getChunkSize() { return chunkSize; }
    public void setChunkSize(int chunkSize) { this.chunkSize = chunkSize; }
    public int getTotalSize() { return totalSize; }
    public void setTotalSize(int totalSize) { this.totalSize = totalSize; }
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public String getAesKey() { return aesKey == null ? "" : aesKey; }
    public void setAesKey(String aesKey) { this.aesKey = aesKey; }
    public String getHmacKey() { return hmacKey == null ? "" : hmacKey; }
    public void setHmacKey(String hmacKey) { this.hmacKey = hmacKey; }
    public String getIv() { return iv == null ? "" : iv; }
    public void setIv(String iv) { this.iv = iv; }

    public Payload toSummary(boolean active) {
        return new Payload(
            getPayloadId(), getExeHash(), chunkCount, chunkSize, totalSize,
            createdAt == null ? Instant.EPOCH : createdAt, active
        );
    }
}
