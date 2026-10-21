package com.lingqiao.server.cards;

import java.util.Locale;
import java.util.Optional;

public final class CardCodes {
    public static final String ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";
    public static final int NORMALIZED_LENGTH = 18;

    private CardCodes() {
    }

    public static String normalize(String code) {
        if (code == null) {
            return "";
        }
        return code.toUpperCase(Locale.ROOT).replace("-", "").replace(" ", "");
    }

    public static boolean same(String a, String b) {
        return normalize(a).equals(normalize(b));
    }

    public static String format(String code) {
        String normalized = normalize(code);
        if (normalized.length() != NORMALIZED_LENGTH) {
            return normalized;
        }
        return normalized.substring(0, 6) + "-" + normalized.substring(6, 12) + "-" + normalized.substring(12, 18);
    }

    public static Optional<String> validateImport(String code) {
        String normalized = normalize(code);
        if (normalized.length() != NORMALIZED_LENGTH) {
            return Optional.empty();
        }
        for (int i = 0; i < normalized.length(); i++) {
            if (ALPHABET.indexOf(normalized.charAt(i)) < 0) {
                return Optional.empty();
            }
        }
        return Optional.of(normalized);
    }
}
