package org.nsone;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

public class SqlLoader {

    public static String loadSql(String vendor, String type){
            String fileName = String.format("/app/sql/%s_%s.sql", vendor.toLowerCase(), type.toLowerCase());
            Path path = Paths.get(fileName);
            try {
                return Files.readString(path);
            } catch (IOException e) {
                throw new IllegalArgumentException("SQL 파일을 읽을 수 없습니다: " + fileName, e);
            }
    }
}
