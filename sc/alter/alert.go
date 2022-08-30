package alter

import (
	"fmt"
	"time"

	"github.com/gitsrc/ipfs-nosql-frame/sc/event"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
)

const (
	LOG_RATE_LIMIT           = 1
	LOG_KEY_NOTIN_WHITE_LIST = 2
	LOG_KEY_LENGTH_WARNING   = 3
)

// 限流告警
// {"event_time":1599466748690,"unique_id":"54b864603dfd9d7ab0ff6de7078f1b7a","type":2}
type LogAlertMsg struct {
	EventType     int    `json:"event_type"`     // 事件类型1限流2不在白名单3积压超出设定值
	EventTime     int64  `json:"event_time"`     // 告警时间
	LogKey        string `json:"log_key"`        // 日志key
	RateThreshold int    `json:"rate_threshold"` // 速率限制
	WarningLength int64  `json:"warning_length"` // 预警长度
	CurrentLength int64  `json:"current_length"` // 当前长度
}

//func LogAlert(eventType int, logKey string, total int) {
//	rs := logAlertMsg{
//		EventType:     eventType,
//		EventTime:     time.Now().UnixNano() / 1e6,
//		LogKey:        logKey,
//		RateThreshold: total,
//	}
//	recordLogAlertWarning(rs)
//	event.Client().Report(event.EVENT_TYPE_LOG, rs)
//}

func LogAlert(rs LogAlertMsg) {
	recordLogAlertWarning(rs)
	event.Client().Report(event.EVENT_TYPE_LOG, rs)
}

func recordLogAlertWarning(params LogAlertMsg) {
	var eventMsg string
	switch {
	case params.EventType == LOG_RATE_LIMIT:
		eventMsg = "触发日志限流"
	case params.EventType == LOG_KEY_NOTIN_WHITE_LIST:
		eventMsg = "日志key不在白名单"
	case params.EventType == LOG_KEY_LENGTH_WARNING:
		eventMsg = "日志key积压预警"
	}
	// 日志写入
	var textLog = fmt.Sprintf(`发生时间:%s;事件:%s;LOGKEY:%s`,
		time.Unix(0, params.EventTime*int64(time.Millisecond)).Format("2006-01-02 15:04:05.000"),
		eventMsg,
		params.LogKey,
	)
	loger.LogWarnw(loger.LogNameLogProxy, textLog)
}
