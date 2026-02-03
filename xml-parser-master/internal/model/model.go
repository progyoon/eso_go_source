package model

import "database/sql"

type Data struct {
	Result interface{} `json:"result"`
	Field  string      `json:"field"`
}

// ElasticDocument: 엘라스틱서치에 저장/전송되는 문서 모델
type ElasticDocument struct {
	EmsID       *string `json:"ems_id"`
	DuId        *string `json:"du_id"`
	CellId      *string `json:"cell_id"`
	CellNum     *string `json:"cell_num"`
	RuParam     *string `json:"ru_param"`
	Data        Data    `json:"data"`
	MeasDate    *string `json:"measdate"`
	EndTime     *string `json:"end_time"`
	MontypeName *string `json:"montype_name"`
	RUName      *string `json:"RU_NAME"`
	Timestamp   *string `json:"@timestamp"`
	EquipID     *string `json:"equip_id"`
	CollectDate *string `json:"collectDate"`
}

type RuMapping struct {
	EMS_Id   *string
	EMSName  *string
	DUId     *string
	RUId     *string
	DU_NAME  *string
	RU_NAME  *string
	CELL_ID  *string
	CELL_NUM *string
}

type RuMappingDAO struct {
	EMS_Id   sql.NullString `db:"ems_id"`
	EMSName  sql.NullString `db:"ems_name"`
	DUId     sql.NullString `db:"du_id"`
	RUId     sql.NullString `db:"ru_id"`
	DU_NAME  sql.NullString `db:"du_name"`
	RU_NAME  sql.NullString `db:"ru_name"`
	CELL_ID  sql.NullString `db:"cell_id"`
	CELL_NUM sql.NullString `db:"cell_num"`
}
