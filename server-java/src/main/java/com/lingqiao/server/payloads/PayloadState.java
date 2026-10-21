package com.lingqiao.server.payloads;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.LinkedHashMap;
import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public class PayloadState {
    @JsonProperty("payloads") private Map<String, PayloadDocument> payloads = new LinkedHashMap<>();
    @JsonProperty("active_id") private String activeId = "";

    public Map<String, PayloadDocument> getPayloads() { return payloads == null ? Map.of() : payloads; }
    public void setPayloads(Map<String, PayloadDocument> payloads) { this.payloads = payloads; }
    public String getActiveId() { return activeId == null ? "" : activeId; }
    public void setActiveId(String activeId) { this.activeId = activeId; }
}
