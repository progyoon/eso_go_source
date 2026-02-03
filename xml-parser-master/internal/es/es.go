package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esutil"
	"github.com/sirupsen/logrus"
	"log"
	"same-parser/internal/config"
	"same-parser/internal/model"
	"time"
)

func NewIndexer(cfg *config.Config) (esutil.BulkIndexer, error) {
	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:           []string{cfg.Elasticsearch.Host},
		Username:            cfg.Elasticsearch.Username,
		Password:            cfg.Elasticsearch.Password,
		CompressRequestBody: true,
		RetryOnStatus:       []int{429, 502, 503, 504},
		RetryBackoff:        func(i int) time.Duration { return time.Duration(i) * 500 * time.Millisecond },
		MaxRetries:          5,
	})
	if err != nil {
		return nil, fmt.Errorf("Elasticsearch 초기화 실패: %w", err)
	}

	indexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        esClient,
		Index:         cfg.Elasticsearch.IndexName, // 실제 인덱스는 아이템에서 덮어씀(날짜 suffixed)
		NumWorkers:    2,
		FlushBytes:    5 << 20, // 5MB
		FlushInterval: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("Bulk indexer initialization failed: %w", err)
	}
	return indexer, nil
}

// StartBulkWorker: docChan에서 ElasticDocument를 읽어 ES 벌크 인덱서에 추가하는 고루틴 실행.
func StartBulkWorker(logger *logrus.Logger, indexer esutil.BulkIndexer, indexName string, docChan <-chan model.ElasticDocument) {
	ctx := context.Background()

	go func() {
		for doc := range docChan {
			// 문서를 JSON으로 마샬(직렬화)
			b, err := json.Marshal(doc)
			if err != nil {
				logger.Errorf("marshal error: %v", err)
				continue
			}

			measDate := safeStr(doc.MeasDate)
			idx := indexName + "-" + indexDateSuffix(measDate)
			// 문서 ID 구성 (중복 방지 목적)
			id := safeStr(doc.RuParam) + "-" +
				safeStr(doc.CellNum) + "-" +
				safeStr(doc.RUName) + "-" +
				doc.Data.Field + "-" +
				measDate
			// Bulk 항목 생성: 인덱스, ID, 본문과 성공/실패 콜백 포함
			item := esutil.BulkIndexerItem{
				Action:     "index",
				DocumentID: id,
				Index:      idx,
				Body:       bytes.NewReader(b),

				OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
					if res.Status > 201 {
						logger.Infof("bulk partial success status=%d id=%s idx=%s", res.Status, item.DocumentID, item.Index)
					}
				},
				OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
					if err != nil {
						log.Printf("ERROR: %s", err)
					} else {
						log.Printf("ERROR: %s: %s", res.Error.Type, res.Error.Reason)
					}
				},
			}

			if err := indexer.Add(ctx, item); err != nil {
				logger.Errorf("indexer.Add failed: id=%s idx=%s err=%v", id, idx, err)
			}
		}
	}()
}

func safeStr(p *string) string {
	if p == nil || *p == "" {
		return "NULL"
	}
	return *p
}

// 200601021504 → YYYY.MM.DD
func indexDateSuffix(measDate string) string {
	if len(measDate) >= 8 {
		return measDate[:4] + "." + measDate[4:6] + "." + measDate[6:8]
	}
	return time.Now().Format("2006.01.02")
}
