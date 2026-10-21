package com.lingqiao.server.admin;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.concurrent.atomic.AtomicLong;
import org.springframework.web.filter.OncePerRequestFilter;

/** Counts every sidecar request, matching Go's outer HTTP middleware counter. */
public class AdminRequestCounterFilter extends OncePerRequestFilter {
    private final AtomicLong requestCount;

    public AdminRequestCounterFilter(AtomicLong requestCount) {
        this.requestCount = requestCount;
    }

    @Override
    protected void doFilterInternal(
        HttpServletRequest request,
        HttpServletResponse response,
        FilterChain filterChain
    ) throws ServletException, IOException {
        requestCount.incrementAndGet();
        filterChain.doFilter(request, response);
    }
}
