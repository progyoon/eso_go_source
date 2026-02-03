FROM openjdk:17-jdk
LABEL authors="nana"

WORKDIR /app

COPY target/tibero-to-sqlite-shaded.jar app.jar
COPY lib/tibero6-jdbc.jar tibero6-jdbc.jar

RUN mkdir -p /app/logs

CMD ["java", "-Duser.timezone=Asia/Seoul", "-cp", "app.jar:tibero6-jdbc.jar:.", "org.nsone.TiberoToSQLite"]