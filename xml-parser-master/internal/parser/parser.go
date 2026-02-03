package parser

import (
	"bufio"
	"github.com/sirupsen/logrus"
	"os"
	"same-parser/internal/config"
	"same-parser/internal/model"
	"same-parser/internal/store"
	"strconv"
	"time"

	"strings"

	xmlparser "github.com/tamerh/xml-stream-parser"
)

type MeasInfoData struct {
	EndTime           string     `json:"endTime"`
	ManagementElement string     `json:"ManagementElement"`
	MeasResult        []MeasInfo `json:"measResult"`
}

// MeasInfo: 수집 타입 이름과 key/value 값 맵의 슬라이스
type MeasInfo struct {
	MontypeName string              `json:"montypeName"`
	Values      []map[string]string `json:"values"`
}

// ProcessXML: XML 파일을 스트리밍으로 읽어 각 measInfo를 처리하고, 최종 지표를 조합해 시간 포맷 변환 후 문서 생성 및 docChan으로 전송
func ProcessXML(logger *logrus.Logger, cfg *config.Config, store *store.Store, filename string, docChan chan<- model.ElasticDocument) {
	file, err := os.Open(filename)
	if err != nil {
		logger.Errorf("파일 열기 오류: %v", err)
		return
	}
	defer file.Close()

	start := time.Now()

	parser := xmlparser.NewXMLParser(bufio.NewReader(file), "measInfo", "measCollec", "managedElement")

	var parsedResult MeasInfoData
	var parsed []MeasInfo

	rrcMap := make(map[string]map[string]float64) // RRC 합산용 버퍼

	for node := range parser.Stream() {
		if node.Err != nil {
			logger.Errorf("XML 파싱 오류: %v", node.Err)
			continue
		}

		switch node.Name {
		case "measCollec":
			if t, ok := node.Attrs["endTime"]; ok {
				parsedResult.EndTime = t
			}
		case "managedElement":
			if t, ok := node.Attrs["localDn"]; ok {
				parsedResult.ManagementElement = t // DU
			}
		case "measInfo":
			switch node.Attrs["measInfoId"] {
			case "Resource Management/RU Power Consumption":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				res := make([]map[string]string, 0, len(node.Childs["measValue"]))
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					m["RU"] = objLdn
					res = append(res, m)
				}
				parsed = append(parsed, MeasInfo{MontypeName: "POWER", Values: res})

			case "RRC/RRC Connection Number":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				res := make([]map[string]string, 0, len(node.Childs["measValue"]))
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					m["RU"] = objLdn
					res = append(res, m)
				}
				parsed = append(parsed, MeasInfo{MontypeName: "MAXUE", Values: res})

			case "Packet Statistics/Air MAC Packet":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				res := make([]map[string]string, 0, len(node.Childs["measValue"]))
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					m["RU"] = objLdn
					res = append(res, m)
				}
				parsed = append(parsed, MeasInfo{MontypeName: "MAC", Values: res})

			case "E-UTRA-NR Dual Connectivity/EN-DC Addition Information":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				res := make([]map[string]string, 0, len(node.Childs["measValue"]))
				aggregated := make(map[string]map[string]float64)

				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					parts := strings.Split(objLdn, "/")
					if len(parts) < 3 {
						continue
					}
					prefixRu := strings.Join(parts[:3], "/")
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					if _, ok := aggregated[prefixRu]; !ok {
						aggregated[prefixRu] = make(map[string]float64)
					}
					for k, v := range m {
						aggregated[prefixRu][k] += parseFloat(v)
					}
				}
				for prefix, vals := range aggregated {
					m := map[string]string{"RU": prefix}
					for k, v := range vals {
						m[k] = floatToString(v)
					}
					res = append(res, m)
				}
				parsed = append(parsed, MeasInfo{MontypeName: "ENDC", Values: res})

			case "RRU/Total PRB Usage":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				res := make([]map[string]string, 0, len(node.Childs["measValue"]))
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					m["RU"] = objLdn
					res = append(res, m)
				}
				parsed = append(parsed, MeasInfo{MontypeName: "PRB", Values: res})

			case "RRC/RRC Connection Establishment":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					parts := strings.Split(objLdn, "/")
					if len(parts) < 3 {
						continue
					}
					subRU := parts[1] + "/" + parts[2]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					if _, ok := rrcMap[subRU]; !ok {
						rrcMap[subRU] = make(map[string]float64)
					}
					rrcMap[subRU]["ConnEstabAtt"] += parseFloat(m["ConnEstabAtt"])
					rrcMap[subRU]["ConnEstabSucc"] += parseFloat(m["ConnEstabSucc"])
				}

			case "RRC/RRC Connection Re-establishment":
				typeText := firstOrEmpty(node.Childs["measTypes"])
				for _, mv := range node.Childs["measValue"] {
					objLdn := mv.Attrs["measObjLdn"]
					parts := strings.Split(objLdn, "/")
					if len(parts) < 3 {
						continue
					}
					subRU := parts[1] + "/" + parts[2]
					resText := firstOrEmpty(mv.Childs["measResults"])
					m := zipResults(typeText, resText)
					if _, ok := rrcMap[subRU]; !ok {
						rrcMap[subRU] = make(map[string]float64)
					}
					rrcMap[subRU]["ConnReEstabAtt"] += parseFloat(m["ConnReEstabAtt"])
					rrcMap[subRU]["ConnReEstabSucc"] += parseFloat(m["ConnReEstabSucc"])
				}
			}
		}
	}

	// RRC 합산
	rrcRes := make([]map[string]string, 0, len(rrcMap))
	for key, v := range rrcMap {
		rrcEntry := map[string]string{}
		rrcEntry["RU"] = "/" + key
		rrcAttempt := v["ConnEstabAtt"] + v["ConnReEstabAtt"]
		rrcEntry["RRCATTEMPT"] = floatToString(rrcAttempt)
		rate := 0.0
		if rrcAttempt > 0 {
			rate = (v["ConnEstabSucc"] + v["ConnReEstabSucc"]) / rrcAttempt * 100.0
		}
		rrcEntry["RRCSUCCRATE"] = floatToString(rate)
		rrcRes = append(rrcRes, rrcEntry)
	}
	parsed = append(parsed, MeasInfo{MontypeName: "RRC", Values: rrcRes})

	parsedResult.MeasResult = parsed

	// 시간 파싱/가공
	collectedDateTime := time.Now().Format("2006-01-02 15:04")
	parsedEndTime, err := time.Parse("2006-01-02T15:04:05.000-07:00", parsedResult.EndTime)
	if err != nil {
		logger.Errorf("시간 파싱 오류: %v", err)
		return
	}
	logger.Debugf("XML 처리 소요: %s", time.Since(start))

	formattedEndTime := parsedEndTime.Format("2006-01-02 15:04")
	measDate := parsedEndTime.Format("200601021504")
	formattedTimeStamp := parsedEndTime.UTC().Format("2006-01-02T15:04:05.000Z")

	// 메트릭 → ES 도큐먼트 전송
	for _, measResult := range parsedResult.MeasResult {
		mType := measResult.MontypeName

		switch {
		case mType == "POWER":
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				pm := roundToTwoDecimalPlaces(parseFloat(value["pmConsumedEnergy"]))
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "pmConsumedEnergy", pm, docChan)
			}

		case mType == "MAXUE" && cfg.Logging.CollectionPeriod != 60:
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "UEMax", int(parseFloat(value["UEMax"])), docChan)
			}

		case mType == "MAC" && cfg.Logging.CollectionPeriod != 60:
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				ul := roundToTwoDecimalPlaces(parseFloat(value["AirMacULKB"]) / 1024)
				dl := roundToTwoDecimalPlaces(parseFloat(value["AirMacDLKB"]) / 1024)
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "MACUL", ul, docChan)
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "MACDL", dl, docChan)
			}

		case mType == "ENDC" && cfg.Logging.CollectionPeriod != 60:
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				att := parseFloat(value["EnDc_AddAtt"])
				succ := parseFloat(value["EnDc_AddSucc"])
				rate := 0.0
				if att > 0 {
					rate = (succ / att) * 100.0
				}
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "ENDCATTEMPT", int(att), docChan)
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "ENDCSUCCRATE", roundToTwoDecimalPlaces(rate), docChan)
			}

		case mType == "PRB" && cfg.Logging.CollectionPeriod != 60:
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				dl := roundToTwoDecimalPlaces(parseFloat(value["PRBDownLinkAverage"]))
				ul := roundToTwoDecimalPlaces(parseFloat(value["PRBUpLinkAverage"]))
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "PRBDL", dl, docChan)
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "PRBUL", ul, docChan)
			}

		case mType == "RRC" && cfg.Logging.CollectionPeriod != 60:
			for _, value := range measResult.Values {
				ruParam := parsedResult.ManagementElement + value["RU"]
				attempt := int(parseFloat(value["RRCATTEMPT"]))
				rate := roundToTwoDecimalPlaces(parseFloat(value["RRCSUCCRATE"]))
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "RRCATTEMPT", attempt, docChan)
				emitDocs(logger, store, ruParam, &parsedResult, measDate, formattedEndTime, formattedTimeStamp, collectedDateTime, mType, "RRCSUCCRATE", rate, docChan)
			}
		}
	}
}

// emitDocs: store에서 ruParam에 해당하는 매핑이 있으면 매핑별로 문서를 생성하여 전송,
// 없으면 매핑 없이 기본 문서를 하나 생성해 전송
func emitDocs(
	logger *logrus.Logger,
	store *store.Store,
	ruParam string,
	parsedResult *MeasInfoData,
	measDate, endTime, ts, collected, mType, field string,
	val interface{},
	docChan chan<- model.ElasticDocument,
) {
	if params, ok := store.Get(ruParam); ok {
		for i := range params {
			doc := buildDoc(&params[i], ruParam, parsedResult.ManagementElement, measDate, endTime, ts, collected, mType, field, val)
			docChan <- doc
		}
	} else {
		logger.Debugf("ru_param not found: %s", ruParam)
		doc := buildDoc(nil, ruParam, parsedResult.ManagementElement, measDate, endTime, ts, collected, mType, field, val)
		docChan <- doc
	}
}

// buildDoc: RuMapping (있다면) 정보를 사용해 model.ElasticDocument 최종 도큐먼트 완성
func buildDoc(
	m *model.RuMapping,
	ruParam, equipID, measDate, endTime, ts, collected, mType, field string,
	val interface{},
) model.ElasticDocument {
	rp := ruParam
	md := measDate
	et := endTime
	mt := mType
	tsp := ts
	eq := equipID
	cd := collected
	unknown := "UNKNOWN"

	var emsID, duID, cellID, cellNum, ruName *string
	if m != nil {
		if m.EMSName != nil {
			emsID = m.EMSName // 원본 코드와 동일 매핑(ems_id ← EMSName)
		} else {
			emsID = &unknown
		}
		if m.DUId != nil {
			duID = m.DUId
		} else {
			duID = &unknown
		}
		if m.CELL_ID != nil {
			cellID = m.CELL_ID
		} else {
			cellID = &unknown
		}
		if m.CELL_NUM != nil {
			cellNum = m.CELL_NUM
		} else {
			cellNum = &unknown
		}
		if m.RUId != nil {
			ruName = m.RUId
		} else {
			ruName = &unknown
		}
	} else {
		emsID, duID, cellID, cellNum, ruName = &unknown, &unknown, &unknown, &unknown, &unknown
	}

	return model.ElasticDocument{
		EmsID:       emsID,
		DuId:        duID,
		CellId:      cellID,
		CellNum:     cellNum,
		RuParam:     &rp,
		Data:        model.Data{Result: val, Field: field},
		MeasDate:    &md,
		EndTime:     &et,
		MontypeName: &mt,
		RUName:      ruName,
		Timestamp:   &tsp,
		EquipID:     &eq,
		CollectDate: &cd,
	}
}

func firstOrEmpty(nodes []xmlparser.XMLElement) string {
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0].InnerText
}

// zipResults: measTypes와 measResults 문자열을 짝지어 매핑
func zipResults(typesStr, resultsStr string) map[string]string {
	types := strings.Fields(typesStr)
	values := strings.Fields(resultsStr)

	m := make(map[string]string, len(types))
	for i, t := range types {
		var val string
		if i < len(values) {
			val = values[i]
		}

		switch t {
		case "RuPowerAvg(W)":
			m["pmConsumedEnergy"] = val
		case "ConnNoMax(count)":
			m["UEMax"] = val
		case "AirMacULByte(Kbytes)":
			m["AirMacULKB"] = val
		case "AirMacDLByte(Kbytes)":
			m["AirMacDLKB"] = val
		case "EnDc_AddAtt(count)":
			m["EnDc_AddAtt"] = val
		case "EnDc_AddSucc(count)":
			m["EnDc_AddSucc"] = val
		case "TotPrbDLAvg(%)":
			m["PRBDownLinkAverage"] = val
		case "TotPrbULAvg(%)":
			m["PRBUpLinkAverage"] = val
		case "ConnEstabAtt(count)":
			m["ConnEstabAtt"] = val
		case "ConnEstabSucc(count)":
			m["ConnEstabSucc"] = val
		case "ConnReEstabAtt(count)":
			m["ConnReEstabAtt"] = val
		case "ConnReEstabSucc(count)":
			m["ConnReEstabSucc"] = val
		}
	}
	return m
}

func parseFloat(value string) float64 {
	val := strings.TrimSpace(value)
	if val == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0.0
	}
	return f
}

func floatToString(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

// 소수점 둘째자리까지 반올림
func roundToTwoDecimalPlaces(value float64) float64 {
	const factor = 100.0
	return float64(int64(value*factor+0.5)) / factor
}
