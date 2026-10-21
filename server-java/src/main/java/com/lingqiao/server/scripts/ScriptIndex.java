package com.lingqiao.server.scripts;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.ArrayList;
import java.util.List;

@JsonIgnoreProperties(ignoreUnknown = true)
public class ScriptIndex {
    @JsonProperty("active_id")
    private String activeId = "";

    @JsonProperty("scripts")
    private List<ScriptModuleDocument> scripts = new ArrayList<>();

    public String getActiveId() {
        return activeId == null ? "" : activeId;
    }

    public void setActiveId(String activeId) {
        this.activeId = activeId;
    }

    public List<ScriptModuleDocument> getScripts() {
        return scripts == null ? List.of() : scripts;
    }

    public void setScripts(List<ScriptModuleDocument> scripts) {
        this.scripts = scripts;
    }
}
