/*
 * @Author: gitsrc
 * @Date: 2020-08-03 13:07:14
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-08-03 13:23:20
 * @FilePath: /redis-cluster-proxy/utils/loger/config_log.go
 */
package loger

import "github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"

var (
	logConfig *ConfigLog
)

type ConfigLog struct {
	Log *ConfigLogData
	App *ConfigAppData
}

type ConfigLogData struct {
	OutPut       string
	Debug        bool
	Key          string
	RedisCluster *rediscluster.Cluster
}

type ConfigAppData struct {
	AppName    string
	AppID      string
	AppVersion string
	AppKey     string
	Channel    string
	SubOrgKey  string
	Language   string
}

func ConfigLogInit(configLogData *ConfigLogData, configAppData *ConfigAppData) error {
	logConfig = &ConfigLog{
		Log: configLogData,
		App: configAppData,
	}
	if len(logConfig.App.AppName) == 0 {
		logConfig = configLogGetDefault()
	}
	return loggerInit()
}

func configLogGetDefault() *ConfigLog {
	c := new(ConfigLog)
	c.Log = &ConfigLogData{}
	c.App.AppName = "app_test"
	c.Log.Debug = true
	c.Log.OutPut = "stdout"
	return c
}
