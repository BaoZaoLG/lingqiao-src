package com.lingqiao.server.agents;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;

final class AgentsCsv {
    private AgentsCsv() {
    }

    static byte[] export(List<Agent> agents) {
        StringBuilder csv = new StringBuilder();
        appendRecord(csv, List.of(
            "agent_id", "username", "disabled", "total_cards", "active_cards",
            "expired_cards", "bound_machines", "last_card_created_at", "created_at"));
        for (Agent agent : agents) {
            appendRecord(csv, List.of(
                spreadsheetSafe(agent.id()),
                spreadsheetSafe(agent.username()),
                Boolean.toString(agent.disabled()),
                Integer.toString(agent.totalCards()),
                Integer.toString(agent.activeCards()),
                Integer.toString(agent.expiredCards()),
                Integer.toString(agent.boundMachines()),
                rfc3339(agent.lastCardCreatedAt()),
                rfc3339(agent.createdAt())
            ));
        }
        return csv.toString().getBytes(StandardCharsets.UTF_8);
    }

    static String spreadsheetSafe(String value) {
        if (value != null && !value.isEmpty() && "=+-@".indexOf(value.charAt(0)) >= 0) {
            return "'" + value;
        }
        return value == null ? "" : value;
    }

    private static String rfc3339(Instant instant) {
        return instant == null ? "" : instant.toString();
    }

    private static void appendRecord(StringBuilder csv, List<String> cells) {
        for (int i = 0; i < cells.size(); i++) {
            if (i > 0) {
                csv.append(',');
            }
            String cell = cells.get(i);
            boolean quote = cell.indexOf(',') >= 0
                || cell.indexOf('"') >= 0
                || cell.indexOf('\r') >= 0
                || cell.indexOf('\n') >= 0;
            if (quote) {
                csv.append('"').append(cell.replace("\"", "\"\"")).append('"');
            } else {
                csv.append(cell);
            }
        }
        csv.append("\r\n");
    }
}
