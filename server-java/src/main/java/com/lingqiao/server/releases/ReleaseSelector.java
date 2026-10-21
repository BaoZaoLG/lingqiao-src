package com.lingqiao.server.releases;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;

/**
 * Release rollout selection mirroring Go releases.Selector helpers:
 * version comparison, targeting rules, and deterministic rollout bucketing.
 */
public final class ReleaseSelector {
    private ReleaseSelector() {
    }

    public static boolean versionGreater(String candidate, String current) {
        int[] c = versionParts(candidate);
        int[] v = versionParts(current);
        int length = Math.max(c.length, v.length);
        for (int i = 0; i < length; i++) {
            int cv = i < c.length ? c[i] : 0;
            int vv = i < v.length ? v[i] : 0;
            if (cv > vv) {
                return true;
            }
            if (cv < vv) {
                return false;
            }
        }
        return false;
    }

    public static boolean matchesTargeting(TargetingRules rules, ClientContext client) {
        TargetingRules effective = rules == null ? TargetingRules.empty() : rules;
        if (contains(effective.denyCards(), client.cardCode())
            || contains(effective.denyMachines(), client.machineId())
            || contains(effective.denyAgents(), client.agentId())) {
            return false;
        }
        boolean hasAllow = notEmpty(effective.allowCards())
            || notEmpty(effective.allowMachines())
            || notEmpty(effective.allowAgents());
        if (!hasAllow) {
            return true;
        }
        return contains(effective.allowCards(), client.cardCode())
            || contains(effective.allowMachines(), client.machineId())
            || contains(effective.allowAgents(), client.agentId());
    }

    public static boolean inRollout(String releaseId, int percent, ClientContext client) {
        if (percent <= 0) {
            return false;
        }
        if (percent >= 100) {
            return true;
        }
        String key = firstNonEmpty(client.machineId(), client.cardCode(), client.agentId(), client.version());
        byte[] digest = sha256((releaseId + "|" + key).getBytes(StandardCharsets.UTF_8));
        long high = 0;
        for (int i = 0; i < 8; i++) {
            high = (high << 8) | (digest[i] & 0xFFL);
        }
        long bucket = Long.remainderUnsigned(high, 100);
        return bucket < percent;
    }

    static String firstNonEmpty(String... values) {
        for (String value : values) {
            if (value != null && !value.isEmpty()) {
                return value;
            }
        }
        return "anonymous";
    }

    static boolean contains(List<String> values, String value) {
        if (value == null || value.isEmpty() || values == null) {
            return false;
        }
        for (String candidate : values) {
            if (candidate != null && candidate.trim().equalsIgnoreCase(value.trim())) {
                return true;
            }
        }
        return false;
    }

    static int[] versionParts(String version) {
        String[] chunks = (version == null ? "" : version).split("\\.", -1);
        int[] parts = new int[chunks.length];
        for (int i = 0; i < chunks.length; i++) {
            String chunk = chunks[i].trim();
            if (chunk.isEmpty()) {
                parts[i] = 0;
                continue;
            }
            try {
                parts[i] = Integer.parseInt(chunk);
            } catch (NumberFormatException ex) {
                parts[i] = 0;
            }
        }
        return parts;
    }

    private static boolean notEmpty(List<String> values) {
        return values != null && !values.isEmpty();
    }

    private static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    public record ClientContext(
        String version,
        String channel,
        String cardCode,
        String machineId,
        String agentId
    ) {
    }
}
