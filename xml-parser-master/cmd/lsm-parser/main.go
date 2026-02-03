package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	_ "modernc.org/sqlite" // SQLite3 driver
	"os"
	"same-parser/internal/config"
	"same-parser/internal/es"
	"same-parser/internal/logging"
	"same-parser/internal/model"
	"same-parser/internal/parser"
	"same-parser/internal/store"
	"strings"
	"time"
)

func main() {
	time.Local = time.FixedZone("KST", 9*60*60)
	// 인자 파싱
	configFile := flag.String("c", "", "설정 파일 경로 (예: config.yml)")
	configFileAlias := flag.String("config", "", "설정 파일 경로 (예: config.yml)")
	flag.Parse()

	cfgPath := *configFile
	if cfgPath == "" {
		cfgPath = *configFileAlias
	}
	if cfgPath == "" {
		printUsage()
	}

	// 설정 로드
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fmt.Println("Failed to load configuration file:", cfgPath, "Exiting:", err)
		os.Exit(1)
	}

	// 로깅
	logger, err := logging.Setup(cfg)
	if err != nil {
		fmt.Println("Failed to setup logging:", err)
		os.Exit(1)
	}

	// --------------------------------------------------------------------------------
	// Elasticsearch 인덱서 초기화
	// - ES 클라이언트/인덱싱 관련 초기화.
	// - 실패 시 로그 남기고 종료.
	// --------------------------------------------------------------------------------
	indexer, err := es.NewIndexer(cfg)
	if err != nil {
		logger.Fatalf("Elasticsearch 초기화 실패: %v", err)
	}

	// --------------------------------------------------------------------------------
	// 채널 생성
	// - docChan: 파싱 후 Elasticsearch에 보낼 문서 버퍼
	// - jobChan: 감지된 파일 경로를 전송하는 작업 큐
	// - 채널 버퍼 크기는 설정이나 예상 트래픽에 따라 조정 가능
	// --------------------------------------------------------------------------------
	docChan := make(chan model.ElasticDocument, 50000)
	jobChan := make(chan string, 50000)

	// --------------------------------------------------------------------------------
	// SQLite DB 오픈 및 매핑 초기화(ENRICH 용)
	// - SQLite 파일 경로는 설정에서 가져옴.
	// - busy_timeout 파라미터를 통해 잠금 대기 시간 설정.
	// - store 초기화 및 주기적 업데이트 시작.
	// --------------------------------------------------------------------------------
	db, err := sql.Open("sqlite", "file:"+cfg.FileDir.SQLiteDBDir+"?_busy_timeout=5000")
	if err != nil {
		logger.Fatalf("SQLite 오픈 실패: %v", err)
	}
	defer db.Close()

	store := store.NewStore()
	if err := store.Init(db); err != nil {
		logger.Fatalf("ruMappingMap 초기화 실패: %v", err)
	}
	//주기적 갱신 시작.
	store.StartPeriodicUpdate(db, logger)

	// --------------------------------------------------------------------------------
	// 파일 감시자 설정 (fsnotify)
	// - 특정 디렉터리를 감시하여 파일 생성 이벤트를 수신.
	// - 이벤트에서 .xml 파일 생성만 jobChan으로 전달.
	// --------------------------------------------------------------------------------
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatalf("watcher 생성 실패: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(cfg.FileDir.ScanDir); err != nil {
		logger.Fatalf("감시 경로 추가 실패: %v", err)
	}

	// --------------------------------------------------------------------------------
	// 동시 처리 제한(세마포어)
	// - OpenFile 작업을 동시에 수행할 수 있는 최대 개수를 설정.
	// - 과도한 동시성으로 인한 리소스 고갈 방지 목적.
	// --------------------------------------------------------------------------------
	maxWorkers := cfg.Worker.OpenFileWorkerCount
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	sem := make(chan struct{}, maxWorkers)

	// --------------------------------------------------------------------------------
	// Elasticsearch 벌크 워커 시작
	// - docChan에서 문서를 가져와 ES로 벌크 업로드 수행.
	// - 내부적으로 별도의 고루틴(들)에서 동작.
	// --------------------------------------------------------------------------------
	es.StartBulkWorker(logger, indexer, cfg.Elasticsearch.IndexName, docChan)

	// --------------------------------------------------------------------------------
	// 파일 감시 루프 (goroutine)
	// - fsnotify 이벤트를 받아 .xml 파일 생성 이벤트를 감지하여 jobChan에 전달.
	// - watcher.Errors 채널을 모니터링하여 에러 로깅 수행.
	// - recover로 panic 방지 및 로그 기록.
	// --------------------------------------------------------------------------------
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("Watcher goroutine panic: %v", r)
			}
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 && strings.HasSuffix(strings.ToLower(event.Name), ".xml") {
					jobChan <- event.Name
				}
			case watcherErr, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Errorf("Watcher error: %v", watcherErr)
			}
		}
	}()

	// --------------------------------------------------------------------------------
	// 작업 소비 루프
	// - jobChan으로부터 파일 경로를 받아 비동기 처리.
	// - sem 채널을 통해 동시 실행 수 제한.
	// - waitStable로 파일이 완전히 업로드/작성되어 안정된 상태인지 검사 후 파서 실행.
	// --------------------------------------------------------------------------------
	for path := range jobChan {
		sem <- struct{}{}
		go func(p string) {
			defer func() { <-sem }()
			if stableErr := waitStable(p, 2*time.Second); stableErr != nil {
				logger.Errorf("안정화 오류: %v", stableErr)
				return
			}
			logger.Debugf("✅ 안정화 완료: %s", p)
			parser.ProcessXML(logger, cfg, store, p, docChan)
		}(path)
	}

	_ = context.Background()
}

func printUsage() {
	usage := `Usage: fetch-xml-files -c <config_file>
 -c, --config    설정 파일 경로 (예: config.yml)
`
	fmt.Print(usage)
	os.Exit(1)
}

// waitStable: 파일이 일정 시간 동안 크기 변동이 없을 때까지 대기.
func waitStable(name string, stableDur time.Duration) error {
	var prevSize int64 = -1
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(20 * time.Second)
	var stableStart time.Time

	for {
		select {
		case <-ticker.C:
			fi, err := os.Stat(name)
			if err != nil {
				return err
			}
			cur := fi.Size()
			if cur == prevSize {
				if stableStart.IsZero() {
					stableStart = time.Now()
				} else if time.Since(stableStart) >= stableDur {
					return nil
				}
			} else {
				prevSize = cur
				stableStart = time.Time{}
			}
		case <-timeout:
			return fmt.Errorf("파일 안정화 타임아웃: %s", name)
		}
	}
}
