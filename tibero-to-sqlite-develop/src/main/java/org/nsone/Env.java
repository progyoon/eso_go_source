package org.nsone;

import io.github.cdimascio.dotenv.Dotenv;

/**
 * 환경 변수 로딩 유틸리티
 * - .env 우선 → 시스템 환경변수 → 기본값 순
 */
public class Env {
    private static final Dotenv dotenv = Dotenv.configure()
            .ignoreIfMalformed()
            .ignoreIfMissing()
            .load();

    public static String get(String key, String defaultValue) {
        String value = dotenv.get(key);
        if (value != null && !value.isBlank()) return value;

        value = System.getenv(key);
        return (value != null && !value.isBlank()) ? value : defaultValue;
    }

    public static String get(String key) {
        return get(key, null);
    }

    public static int getInt(String key, int defaultValue) {
        try {
            return Integer.parseInt(get(key, String.valueOf(defaultValue)));
        } catch (NumberFormatException e) {
            return defaultValue;
        }
    }
}
