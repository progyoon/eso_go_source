package store

import (
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"same-parser/internal/model"
	"sync"
	"time"
)

const getRuMappingQuery = `
	SELECT 
		ru_param,
		ems_id, 
		ems_name, 
		du_id,
		ru_id,
		du_name,
		ru_name,
		cell_id,
		cell_num
	FROM 
		ru_mapping
`

type Store struct {
	mutex     sync.Mutex
	ruMapping map[string][]model.RuMapping
}

func NewStore() *Store {
	return &Store{
		ruMapping: make(map[string][]model.RuMapping),
	}
}

// Init: 애플리케이션 시작 시 DB에서 초기 로드. 메모리에 캐싱.
func (s *Store) Init(db *sql.DB) error {
	rows, err := db.Query(getRuMappingQuery)
	if err != nil {
		return fmt.Errorf("failed to query ru_mapping: %w", err)
	}
	defer rows.Close()

	temp := make(map[string][]model.RuMapping)
	for rows.Next() {
		var ruParam string
		var d model.RuMappingDAO
		if err = rows.Scan(&ruParam, &d.EMS_Id, &d.EMSName, &d.DUId, &d.RUId, &d.DU_NAME, &d.RU_NAME, &d.CELL_ID, &d.CELL_NUM); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		temp[ruParam] = append(temp[ruParam], model.RuMapping{
			EMS_Id:   nilIfInvalid(d.EMS_Id),
			EMSName:  nilIfInvalid(d.EMSName),
			DUId:     nilIfInvalid(d.DUId),
			RUId:     nilIfInvalid(d.RUId),
			DU_NAME:  nilIfInvalid(d.DU_NAME),
			RU_NAME:  nilIfInvalid(d.RU_NAME),
			CELL_ID:  nilIfInvalid(d.CELL_ID),
			CELL_NUM: nilIfInvalid(d.CELL_NUM),
		})
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	s.mutex.Lock()
	s.ruMapping = temp
	s.mutex.Unlock()

	return nil
}

func (s *Store) Get(key string) ([]model.RuMapping, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	val, ok := s.ruMapping[key]
	return val, ok
}

// Update: DB에서 재로드하여 맵을 새로 교체(Init과 동일한 동작)
func (s *Store) Update(db *sql.DB) error {
	rows, err := db.Query(getRuMappingQuery)
	if err != nil {
		return fmt.Errorf("failed to query ru_mapping: %w", err)
	}
	defer rows.Close()

	temp := make(map[string][]model.RuMapping)
	for rows.Next() {
		var ruParam string
		var d model.RuMappingDAO
		if err = rows.Scan(&ruParam, &d.EMS_Id, &d.EMSName, &d.DUId, &d.RUId, &d.DU_NAME, &d.RU_NAME, &d.CELL_ID, &d.CELL_NUM); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		temp[ruParam] = append(temp[ruParam], model.RuMapping{
			EMS_Id:   nilIfInvalid(d.EMS_Id),
			EMSName:  nilIfInvalid(d.EMSName),
			DUId:     nilIfInvalid(d.DUId),
			RUId:     nilIfInvalid(d.RUId),
			DU_NAME:  nilIfInvalid(d.DU_NAME),
			RU_NAME:  nilIfInvalid(d.RU_NAME),
			CELL_ID:  nilIfInvalid(d.CELL_ID),
			CELL_NUM: nilIfInvalid(d.CELL_NUM),
		})
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	s.mutex.Lock()
	s.ruMapping = temp
	s.mutex.Unlock()

	return nil
}

// StartPeriodicUpdate: 6시간 간격으로 백그라운드 업데이트 시작, 오류는 로거에 기록
func (s *Store) StartPeriodicUpdate(db *sql.DB, logger *logrus.Logger) {
	ticker := time.NewTicker(6 * time.Hour)
	go func() {
		for range ticker.C {
			if err := s.Update(db); err != nil {
				logger.Errorf("Error updating ruMappingMap: %v", err)
			} else {
				logger.Infof("ruMappingMap updated successfully.")
			}
		}
	}()
}

func nilIfInvalid(n sql.NullString) *string {
	if n.Valid {
		return &n.String
	}
	return nil
}
