package com.lingqiao.server.scripts;

import java.util.List;

public record ScriptListResult(
    List<ScriptModule> scripts,
    String activeId,
    ScriptModule active
) {
}
