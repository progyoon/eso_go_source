package logging

import (
	"fmt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"path/filepath"
	"same-parser/internal/config"
	"strings"
	"time"
)

type CustomFormatter struct{}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	now := entry.Time.In(time.FixedZone("KST", 9*60*60))
	timestamp := now.Format("20060102 15:04:05.000")
	caller := ""
	if entry.HasCaller() {
		caller = fmt.Sprintf("%s:%d", filepath.Base(entry.Caller.File), entry.Caller.Line)
	}
	line := fmt.Sprintf("[%s][%s][%s] : %s\n",
		timestamp, strings.ToUpper(entry.Level.String()), caller, entry.Message)
	return []byte(line), nil
}

func Setup(cfg *config.Config) (*logrus.Logger, error) {
	level, err := logrus.ParseLevel(strings.ToLower(cfg.Logging.LogLevel))
	if err != nil {
		level = logrus.InfoLevel
	}
	logger := logrus.New()
	logger.SetLevel(level)
	logger.SetFormatter(&CustomFormatter{})
	logger.SetReportCaller(true)

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return nil, fmt.Errorf("KST 타임존 로드 실패: %w", err)
	}
	time.Local = loc

	if err := os.MkdirAll(cfg.Logging.LogDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("로그 디렉토리 생성 실패: %w", err)
	}

	logpath := filepath.Join(cfg.Logging.LogDir, cfg.Logging.LogPrefix)
	writer, err := rotatelogs.New(
		fmt.Sprintf("%s_%d_%%Y%%m%%d.log", logpath, cfg.Logging.CollectionPeriod),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithClock(rotatelogs.Local),
		rotatelogs.WithMaxAge(time.Duration(cfg.Logging.RetentionDays)*24*time.Hour),
	)
	if err != nil {
		log.Fatalf("로그 로테이터 초기화 실패: %v", err)
	}
	logger.SetOutput(io.MultiWriter(os.Stdout, writer))

	return logger, nil
}
