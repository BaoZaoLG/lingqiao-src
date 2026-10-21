package com.lingqiao.server.admin;

import java.time.Duration;
import java.time.Instant;
import java.util.ArrayDeque;
import java.util.Deque;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Sliding-window login attempt limiter mirroring Go httpapi.RateLimiter usage
 * for the admin login endpoint.
 */
public class LoginRateLimiter {
    private final int limit;
    private final Duration window;
    private final Map<String, Deque<Instant>> attempts = new ConcurrentHashMap<>();

    public LoginRateLimiter(int limit, Duration window) {
        this.limit = limit;
        this.window = window;
    }

    public synchronized boolean allow(String key) {
        String bucket = key == null || key.isBlank() ? "unknown" : key;
        Instant now = Instant.now();
        Instant cutoff = now.minus(window);
        Deque<Instant> timestamps = attempts.computeIfAbsent(bucket, k -> new ArrayDeque<>());
        while (!timestamps.isEmpty() && timestamps.peekFirst().isBefore(cutoff)) {
            timestamps.pollFirst();
        }
        if (timestamps.size() >= limit) {
            return false;
        }
        timestamps.addLast(now);
        return true;
    }

    public synchronized void clear(String key) {
        attempts.remove(key == null || key.isBlank() ? "unknown" : key);
    }
}
