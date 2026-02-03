package org.nsone;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;

public class EnvValidator {

    private static final Logger logger = LoggerFactory.getLogger(EnvValidator.class);

    /**
     * 필수 환경 변수 체크
     *
     * @param requiredKeys 필수 키 목록
     */
    public static void validateOrExit(List<String> requiredKeys) {
        boolean hasError = false;

        for (String key : requiredKeys) {
            String value = Env.get(key);
            if (value == null || value.isBlank()) {
                logger.error("필수 환경 변수 '{}'가 누락되었습니다. .env 파일을 확인해주세요.", key);
                hasError = true;
            }
        }

        if (hasError) {
            logger.error("환경 변수 누락으로 인해 애플리케이션을 종료합니다.");
            System.exit(1);
        }
    }

}
