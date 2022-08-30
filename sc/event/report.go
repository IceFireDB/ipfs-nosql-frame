package event

import (
	"encoding/json"
	"time"

	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	"github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"
)

const (
	MSP_EVENT                = "msp:event_msg"
	EVENT_TYPE_LOG           = "log"
	LOG_RATE_LIMIT           = 1
	LOG_KEY_NOTIN_WHITE_LIST = 2
	LOG_KEY_LENGTH_WARNING   = 3
)

type EventReport struct {
	redisCluster *rediscluster.Cluster
}

type MSPEvent struct {
	EventType string      `json:"event_type"` // logratelimit
	EventTime int64       `json:"event_time"`
	EventBody interface{} `json:"event_body"`
}

var _eventReport *EventReport

func InitEventClient(rc *rediscluster.Cluster) *EventReport {
	if _eventReport == nil {
		_eventReport = &EventReport{redisCluster: rc}
	}
	return _eventReport
}

func Client() *EventReport {
	return _eventReport
}

func (ev *EventReport) Report(eventType string, eventBody interface{}) {
	msg := MSPEvent{
		EventType: eventType,
		EventTime: time.Now().UnixNano() / 1e6,
		EventBody: eventBody,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		loger.LogErrorw(loger.LogNameDefault, "event report json.Marshal err", err)
		return
	}
	_, err = ev.redisCluster.Do("LPUSH", MSP_EVENT, b)

	if err != nil {
		loger.LogErrorw(loger.LogNameDefault, "EventReport report redis rpush", err)
		return
	}
}
