# Tibero to SQLite Sync

Tibero DB의 데이터를 주기적으로 조회하여 필요한 컬럼만 추출하고,  
SQLite에 삽입/업데이트하는 Java 애플리케이션입니다.

이 SQLite 파일은 이후 다른 시스템(Golang 등)에서 ElasticSearch로 연동하거나 enrich 작업에 활용될 수 있습니다.

---

## ✨ 기능

- Tibero JDBC를 통한 벤더별 데이터 조회
- SQLite DB에 `UPSERT` 방식으로 삽입/갱신
- `.env` 기반 설정으로 유연한 환경 구성
- Log4j2 기반 로그 출력 (콘솔 & 파일)

---

## 🏗️ 프로젝트 구조

```
.
├── sql/                        # 벤더별 SQL 쿼리 파일  
├── src/main/java/org/nsone/
│   ├── TiberoToSQLite.java     # 메인 실행 로직
│   ├── Env.java                # 환경 변수 로딩
│   ├── SqlLoader.java          # SQL 쿼리 로딩 유틸
│   └── EnvValidator.java       # 필수 env 검증 유틸
├── lib/                        # JDBC 드라이버 등 의존 JAR
├── .env.xxx                    # 벤더용 설정
├── log4j2.xml                  # 로깅 설정
├── Dockerfile                  # Docker 이미지 빌드 파일
└── pom.xml                     # Maven 빌드 설정
```

---

## ⚙️ .env 예시

`.env` 파일은 루트 디렉토리에 위치해야 하며 다음과 같은 값을 포함합니다:

```
# Tibero 설정
TIBERO_URL=your.tibero.host:port
TIBERO_USER=username
TIBERO_PASSWORD=password




# SQLite DB 경로
SQLITE_PATH=ru_mapping_xxx.db



# 벤더 이름
VENDOR=XXX

#이동통신 세대 설정: LTE OR 5G
MOBILE_GEN=XXX
```


---

## 🐳 Docker로 실행하기

### 1. Fat JAR 빌드

```bash
mvn clean package
```

> `target/tibero-to-sqlite-shaded.jar` 생성됨

### 2. Docker 이미지 빌드

```bash
docker build -t tibero-sync .
```

### 3. 컨테이너 실행

```bash
docker run --env-file /data1/sub_proc/tiberoSync/config/.XXX_env -v /data1/sub_proc/tiberoSync/sqlite:/app/sqlite   -v /data1/sub_proc/tiberoSync/sql:/app/sql -v /data1/sub_proc/tiberoSync/logs:/app/logs -e vendor_name=XXXX --name  lsm-lte-tibero-sync-service tibero-sync:20250915

```
이때, 환경변수로 사용되는 vendor_name=XXXX는 로그파일의 Prefix로 사용됩니다.

또한, SQL 디렉토리는 vendor_lte.sql 또는 vendor_nr.sql로 표현되어야합니다. (lte또는 nr로 반드시 끝나야함)
sql 디렉토리 참고.

---

## 📦 JAR 직접 실행 (로컬 테스트용)
1. `.env.xxx`를 `.env`로 복사

```bash
cp .env.xxx .env
```

2. 실행

PowerShell 기준:
```bash
$OutputEncoding = [Console]::OutputEncoding = [Text.Encoding]::UTF8
java --% -Dfile.encoding=UTF-8 -cp "target/tibero-to-sqlite-shaded.jar;lib\tibero6-jdbc.jar" org.nsone.TiberoToSQLite
```
Bash (Linux/macOS) 기준:
```bash
java -Dfile.encoding=UTF-8 -cp "target/tibero-to-sqlite-shaded.jar;lib\tibero6-jdbc.jar" org.nsone.TiberoToSQLite
```

---

## 📝 참고 사항

- `log4j2.xml`은 JAR 안에 포함되어 있으며, 로그는 `/app/logs`에 남습니다
- SQLite 파일은 Docker 컨테이너 내에 생성되며, 필요 시 볼륨 마운트로 꺼낼 수 있습니다

