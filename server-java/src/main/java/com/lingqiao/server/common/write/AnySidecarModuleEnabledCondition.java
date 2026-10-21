package com.lingqiao.server.common.write;

import org.springframework.context.annotation.Condition;
import org.springframework.context.annotation.ConditionContext;
import org.springframework.core.env.ConfigurableEnvironment;
import org.springframework.core.env.EnumerablePropertySource;
import org.springframework.core.env.PropertySource;
import org.springframework.core.type.AnnotatedTypeMetadata;

/**
 * Activates the shared datasource when any {@code lingqiao.sidecar.*} module flag is
 * enabled, read or write.
 *
 * <p>Read modules need a {@code JdbcTemplate} too, and Spring Boot's
 * {@code DataSourceAutoConfiguration} is excluded, so gating the datasource on write
 * flags alone made a read-only sidecar fail to start. Enumerating flags by hand went
 * stale every time a module was added, so the condition scans the property namespace
 * instead.
 */
public class AnySidecarModuleEnabledCondition implements Condition {
    private static final String PREFIX = "lingqiao.sidecar.";

    @Override
    public boolean matches(ConditionContext context, AnnotatedTypeMetadata metadata) {
        if (!(context.getEnvironment() instanceof ConfigurableEnvironment environment)) {
            return false;
        }
        for (PropertySource<?> source : environment.getPropertySources()) {
            if (!(source instanceof EnumerablePropertySource<?> enumerable)) {
                continue;
            }
            for (String name : enumerable.getPropertyNames()) {
                if (!name.startsWith(PREFIX)) {
                    continue;
                }
                // Resolve through the environment so placeholders and overrides apply.
                if (Boolean.parseBoolean(environment.getProperty(name))) {
                    return true;
                }
            }
        }
        return false;
    }
}
