package com.lingqiao.server.system;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.ArrayList;
import java.util.List;

@JsonIgnoreProperties(ignoreUnknown = true)
public class AnnouncementIndex {
    @JsonProperty("active_id") private String activeId = "";
    @JsonProperty("announcements") private List<AnnouncementDocument> announcements = new ArrayList<>();

    public String getActiveId() { return activeId == null ? "" : activeId; }
    public void setActiveId(String activeId) { this.activeId = activeId; }
    public List<AnnouncementDocument> getAnnouncements() { return announcements == null ? List.of() : announcements; }
    public void setAnnouncements(List<AnnouncementDocument> announcements) { this.announcements = announcements; }
}
