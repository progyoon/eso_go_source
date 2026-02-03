package org.nsone;

import it.sauronsoftware.cron4j.Scheduler;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.sql.*;
import java.util.List;

public class TiberoToSQLite {

    private static final Logger logger = LoggerFactory.getLogger(TiberoToSQLite.class);

    private static final String TIBERO_JDBC_URL = Env.get("TIBERO_URL");
    private static final String TIBERO_USER = Env.get("TIBERO_USER");
    private static final String TIBERO_PASSWORD = Env.get("TIBERO_PASSWORD");
    private static final String SQLITE_DB = Env.get("SQLITE_PATH");

    public static void main(String[] args) {
        // 필수 환경 변수 검증
        EnvValidator.validateOrExit(List.of(
                "VENDOR",
                "TIBERO_URL",
                "TIBERO_USER",
                "TIBERO_PASSWORD",
                "SQLITE_PATH"
        ));


        syncData();
    }

    private static void syncData() {
        String vendor = Env.get("VENDOR");
        String mobileGen = Env.get("MOBILE_GEN");

        try {
            vendor = vendor.toUpperCase();
            logger.info("{} 사 동기화 시작", vendor);

            Class.forName("com.tmax.tibero.jdbc.TbDriver");

            try (
                    Connection tiberoConn = DriverManager.getConnection(TIBERO_JDBC_URL, TIBERO_USER, TIBERO_PASSWORD);
                    Statement stmt = tiberoConn.createStatement();
                    ResultSet rs = mobileGen.equals("LTE") ? stmt.executeQuery(getVendorSqlForLTE(vendor)) : stmt.executeQuery(getVendorSqlForNR(vendor));
                    Connection sqliteConn = DriverManager.getConnection("jdbc:sqlite:" + SQLITE_DB)
            ) {
                createTableIfNotExists(sqliteConn);
                sqliteConn.setAutoCommit(false);

                PreparedStatement pstmt = sqliteConn.prepareStatement("""
                            INSERT INTO ru_mapping (ru_param, ems_id, ems_name, du_id, ru_id, du_name, ru_name, cell_num, cell_id)
                            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                            ON CONFLICT(ru_param, ru_id) DO UPDATE SET
                                ems_id = excluded.ems_id,
                                ems_name = excluded.ems_name,
                                du_id = excluded.du_id,
                                du_name = excluded.du_name,
                                ru_name = excluded.ru_name,
                                cell_num = excluded.cell_num,
                                cell_id = excluded.cell_id
                                                
                        """);

                int rowCount = 0;
                while (rs.next()) {
                    pstmt.setString(1, rs.getString("RU_PARAM"));
                    pstmt.setString(2, rs.getString("EMS_ID"));
                    pstmt.setString(3, rs.getString("EMS_NAME"));
                    pstmt.setString(4, rs.getString("DU_ID"));
                    pstmt.setString(5, rs.getString("RU_ID"));
                    pstmt.setString(6, rs.getString("DU_NAME"));
                    pstmt.setString(7, rs.getString("RU_NAME"));
                    pstmt.setString(8, rs.getString("CELL_NUM"));
                    pstmt.setString(9, rs.getString("CELL_ID"));
                    pstmt.addBatch();
                    rowCount++;
                }

                pstmt.executeBatch();
                sqliteConn.commit();
                logger.info("SQLite 업데이트 완료: {}건 삽입/갱신", rowCount);

            } catch (SQLException e) {
                logger.error("DB 작업 중 오류 발생", e);
            }

        } catch (IllegalArgumentException | IllegalStateException e) {
            logger.error("환경 설정 오류: {}", e.getMessage());
            System.exit(1);
        } catch (Exception e) {
            logger.error("예기치 않은 오류", e);
            System.exit(1);
        }
    }

    private static void createTableIfNotExists(Connection conn) throws SQLException {
        String ddl = """
                    CREATE TABLE IF NOT EXISTS ru_mapping (
                        ru_param TEXT,
                        ems_id TEXT,
                        ems_name TEXT,
                        du_id TEXT,
                        ru_id TEXT,
                        du_name TEXT,
                        ru_name TEXT,
                        cell_num TEXT,
                        cell_id TEXT,
                       PRIMARY KEY (ru_id, ru_param)           
                    )
                """;
        try (Statement stmt = conn.createStatement()) {
            stmt.execute(ddl);
            logger.debug("SQLite 테이블 확인 또는 생성 완료");
        }
    }


    private static String getVendorSqlForNR(String vendor) {
        try {
            return SqlLoader.loadSql(vendor, "nr");
        } catch (IllegalArgumentException e) {
            throw new IllegalArgumentException("지원되지 않는 VENDOR 값입니다: " + vendor, e);
        }
    }

    private static String getVendorSqlForLTE(String vendor) {
        try {
            return SqlLoader.loadSql(vendor, "lte");
        } catch (IllegalArgumentException e) {
            throw new IllegalArgumentException("지원되지 않는 VENDOR 값입니다: " + vendor, e);
        }
    }


}
