package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gitsrc/ipfs-nosql-frame/sc/alter"
	"github.com/gitsrc/ipfs-nosql-frame/sc/event"
	"github.com/gitsrc/ipfs-nosql-frame/sc/filter"
	"github.com/gitsrc/ipfs-nosql-frame/utils/influxdb"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	"github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"
	client "gitlab.oneitfarm.com/bifrost/influxdata/influxdb1-client/v2"
)

type ReporterUnit struct {
	LogKey        string     `json:"log_key"`
	RpushCount    uint64     `json:"rpush_count"`
	RpushLastTime *time.Time `json:"rpush_last_time"`
	LpopCount     uint64     `json:"lpop_count"`
	LpopLastTime  *time.Time `json:"lpop_last_time"`
	Length        int64      `json:"length"`
}

type Reporter struct {
	sync.RWMutex
	ReporterUnit map[string]ReporterUnit
}

func NewReporter() *Reporter {
	return &Reporter{
		ReporterUnit: make(map[string]ReporterUnit),
	}
}

func (report *Reporter) GetData() map[string]ReporterUnit {
	report.RLock()
	defer report.RUnlock()
	reporterUnitMap := make(map[string]ReporterUnit)
	for key, value := range report.ReporterUnit {
		reporterUnitMap[key] = value
	}
	return reporterUnitMap
}

func (report *Reporter) ReportData(redisCluster *rediscluster.Cluster, filter *filter.Filter) {
	defer func() {
		// Catch ReportData Panic
		if err := recover(); err != nil {
			loger.LogErrorw(loger.LogNameRedis, "ReportData catchPanic", fmt.Errorf("%v", err))
			return
		}
	}()
	for {
		time.Sleep(time.Second * 30)
		reporterUnit := report.GetData()
		if len(reporterUnit) == 0 {
			continue
		}
		filterRule := filter.GetData()
		var reportSli []ReporterUnit
		for key, value := range reporterUnit {
			value.LogKey = key
			// 长度判断，预警
			keyRule, ok := filterRule[key]
			if ok {
				value.Length = keyRule.Length
				warningLength := keyRule.WarningLength
				if warningLength == 0 {
					warningLength = 1000
				}
				if value.Length >= warningLength {
					// 触发长度预警
					alter.LogAlert(alter.LogAlertMsg{
						EventType:     event.LOG_KEY_LENGTH_WARNING,
						EventTime:     time.Now().UnixNano() / 1e6,
						LogKey:        key,
						WarningLength: warningLength,
						CurrentLength: value.Length,
					})
				}
			}
			reportSli = append(reportSli, value)
		}
		reportSliJSON, _ := json.Marshal(reportSli)
		// 这里使用LPUSH，因为RPUSH已经被改造
		go redisCluster.Do("LPUSH", "msp:log_metrics", reportSliJSON)
	}
}

func (report *Reporter) ReportMetrics(influxdb *influxdb.InfluxDBHTTP, filter *filter.Filter) {
	defer func() {
		// Catch ReportData Panic
		if err := recover(); err != nil {
			loger.LogErrorw(loger.LogNameRedis, "ReportMetrics catchPanic", fmt.Errorf("%v", err))
			return
		}
	}()
	bp, err := client.NewBatchPoints(influxdb.BatchPointsConfig)
	if err != nil {
		loger.LogErrorw(loger.LogNameDefault, "influxdb client.NewBatchPoints err", err)
		return
	}
	for {
		time.Sleep(time.Second * 10)
		reporterUnit := report.GetData()
		if len(reporterUnit) == 0 {
			continue
		}
		filterRule := filter.GetData()

		for key, value := range reporterUnit {
			var warningLength int64
			keyRule, ok := filterRule[key]
			if ok {
				value.Length = keyRule.Length
				warningLength = keyRule.WarningLength
				if warningLength == 0 {
					warningLength = 1000
				}
			}
			hostname := os.Getenv("HOSTNAME")
			if len(hostname) == 0 {
				hostname, _ = os.Hostname()
			}
			tags := map[string]string{
				"hostname": hostname,
				"key":      key,
			}
			fields := map[string]interface{}{
				"rpush":          value.RpushCount,
				"lpop":           value.LpopCount,
				"length":         value.Length,
				"warning_length": warningLength,
			}
			pt, err := client.NewPoint("log_key", tags, fields, time.Now())
			if err != nil {
				loger.LogErrorw(loger.LogNameDefault, "influxdb client.NewPoint err", err)
				continue
			}
			bp.AddPoint(pt)
			if len(bp.Points()) >= 10 {
				err = influxdb.Client.Write(bp)
				if err != nil {
					loger.LogErrorw(loger.LogNameDefault, "influxdb client.Write err", err)
				}
				// 关闭长连接
				err = influxdb.Client.Close()
				if err != nil {
					loger.LogErrorw(loger.LogNameDefault, "influxdb client.close err", err)
				}
				// 重新初始化bp，清空bp
				bp, err = client.NewBatchPoints(influxdb.BatchPointsConfig)
				if err != nil {
					loger.LogErrorw(loger.LogNameDefault, "influxdb client.NewBatchPoints err", err)
				}
			}
		}
	}
}

func (report *Reporter) AddRpushCommand(key string) {
	report.Lock()
	defer report.Unlock()
	reporterUnit := report.ReporterUnit[key]
	reporterUnit.RpushCount++
	timeNow := time.Now()
	reporterUnit.RpushLastTime = &timeNow
	report.ReporterUnit[key] = reporterUnit
}

func (report *Reporter) AddLpopCommand(key string) {
	report.Lock()
	defer report.Unlock()
	if strings.Contains(key, "sc_log") {
		key = "sc_log"
	}
	reporterUnit := report.ReporterUnit[key]
	reporterUnit.LpopCount++
	timeNow := time.Now()
	reporterUnit.LpopLastTime = &timeNow
	report.ReporterUnit[key] = reporterUnit
}

// ClearKey 同步最新key，删除不存在的key
func (report *Reporter) ClearKey(filter *filter.Filter) {
	for {
		time.Sleep(time.Second * 30)
		reporterUnit := report.GetData()
		if len(reporterUnit) == 0 {
			continue
		}
		// 聚合过期的key
		filterRule := filter.GetData()
		var keys []string
		for key := range reporterUnit {
			if _, ok := filterRule[key]; !ok {
				keys = append(keys, key)
			}
		}
		if len(keys) > 0 {
			// 执行删除
			report.Lock()
			for _, value := range keys {
				delete(report.ReporterUnit, value)
			}
			report.Unlock()
		}
	}
}
