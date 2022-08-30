package logrusrchook

import (
	"encoding/json"
	"fmt"
	"time"

	rediscluster "github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"

	"github.com/sirupsen/logrus"
)

// HookConfig stores configuration needed to setup the hook
type HookConfig struct {
	Key          string
	Format       string
	App          string
	Hostname     string
	RedisCluster *rediscluster.Cluster
}

// RedisHook to sends logs to Redis server
type RedisHook struct {
	RedisCluster   *rediscluster.Cluster
	RedisKey       string
	LogstashFormat string
	AppName        string
	Hostname       string
}

// NewHook creates a hook to be added to an instance of logger
func NewHook(config HookConfig) (*RedisHook, error) {

	if config.Format != "v0" && config.Format != "v1" && config.Format != "access" && config.Format != "origin" {
		return nil, fmt.Errorf("unknown message format")
	}

	return &RedisHook{
		RedisCluster:   config.RedisCluster,
		RedisKey:       config.Key,
		LogstashFormat: config.Format,
		AppName:        config.App,
		Hostname:       config.Hostname,
	}, nil

}

// Fire is called when a log event is fired.
func (hook *RedisHook) Fire(entry *logrus.Entry) error {
	var msg interface{}

	switch hook.LogstashFormat {
	case "v0":
		msg = createV0Message(entry, hook.AppName, hook.Hostname)
	case "v1":
		msg = createV1Message(entry, hook.AppName, hook.Hostname)
	case "access":
		msg = createAccessLogMessage(entry, hook.AppName, hook.Hostname)
	case "origin":
		msg = createOriginLogMessage(entry)
	default:
		fmt.Println("Invalid LogstashFormat")
	}

	// Marshal into json message
	js, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error creating message for REDIS: %s", err)
	}

	// send message to rediscluster
	_, err = hook.RedisCluster.Do("RPUSH", hook.RedisKey, js)
	if err != nil {
		return fmt.Errorf("error sending message to REDIS: %s", err)
	}

	return nil
}

// Levels returns the available logging levels.
func (hook *RedisHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.TraceLevel,
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func createV0Message(entry *logrus.Entry, appName, hostname string) map[string]interface{} {
	m := make(map[string]interface{})
	m["@timestamp"] = entry.Time.UTC().Format(time.RFC3339Nano)
	m["@source_host"] = hostname
	m["@message"] = entry.Message

	fields := make(map[string]interface{})
	fields["level"] = entry.Level.String()
	fields["application"] = appName

	for k, v := range entry.Data {
		fields[k] = v
	}
	m["@fields"] = fields

	return m
}

func createV1Message(entry *logrus.Entry, appName, hostname string) map[string]interface{} {
	m := make(map[string]interface{})
	m["@timestamp"] = entry.Time.UTC().Format(time.RFC3339Nano)
	m["host"] = hostname
	m["message"] = entry.Message
	m["level"] = entry.Level.String()
	m["application"] = appName
	for k, v := range entry.Data {
		m[k] = v
	}

	return m
}

func createAccessLogMessage(entry *logrus.Entry, appName, hostname string) map[string]interface{} {
	m := make(map[string]interface{})
	m["message"] = entry.Message
	m["@source_host"] = hostname

	fields := make(map[string]interface{})
	fields["application"] = appName

	for k, v := range entry.Data {
		fields[k] = v
	}
	m["@fields"] = fields

	return m
}

func createOriginLogMessage(entry *logrus.Entry) map[string]interface{} {
	fields := make(map[string]interface{})

	for k, v := range entry.Data {
		fields[k] = v
	}
	var level = entry.Level.String()
	if level == "ERROR" {
		level = "ERR"
	}
	fields["level"] = level
	fields["message"] = entry.Message

	return fields
}
