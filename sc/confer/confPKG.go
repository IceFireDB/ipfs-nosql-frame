/*
 * @Author: gitsrc
 * @Date: 2020-07-09 10:37:22
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-09-17 14:42:42
 * @FilePath: /log-redis-cluster-proxy/sc/confer/confPKG.go
 */

package confer

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// GetNewConfer is core function of Service Car : get a Confer entity.
func GetNewConfer(confFileURI string) (confer *Confer, err error) {
	confer = &Confer{}

	// Parse config from yaml file
	confer.Opts, err = parseYamlFromFile(confFileURI)

	// Disable master-slave percentage of redis read-write separation
	confer.Opts.RedisClusterConf.SlaveOperateRate = 0

	// Guarantee the lower limit of the heartbeat of the cluster: the minimum frequency is 5 seconds
	if confer.Opts.RedisClusterConf.ClusterUpdateHeartbeat < 5 {
		confer.Opts.RedisClusterConf.ClusterUpdateHeartbeat = 5
	}

	// Check for Debug parameters: If Debug monitoring is enabled
	if confer.Opts.DebugConf.Enable {
		// If the listening address is an empty string in the scenario where Enable is enabled, it indicates that the configuration is incorrect
		if len(confer.Opts.DebugConf.PprofURI) == 0 {
			err = errors.New("When the Enable parameter in Debug is True, the pprof_uri parameter cannot be empty, and a meaningful pprof listening address must be filled in")
			return
		}
	}

	// If the middleware has enabled the cache function
	if confer.Opts.MemoryCacheConf.Enable {
		// Cache related configuration parameter security verification: set the minimum number of items in the cache
		if confer.Opts.MemoryCacheConf.MaxItemsCount < 1024 {
			confer.Opts.MemoryCacheConf.MaxItemsCount = 1024
		}
	}

	// memory cache expires by default ms lower limit check
	if confer.Opts.MemoryCacheConf.DefaultExpiration <= 0 {
		confer.Opts.MemoryCacheConf.DefaultExpiration = 0
	}

	// memory cache expiry kv automatic cleaning cycle minimum cycle limit (minimum 10s)
	if confer.Opts.MemoryCacheConf.CleanupInterval <= 10 {
		confer.Opts.MemoryCacheConf.CleanupInterval = 10
	}

	// 环境变量参数调整
	if startNodes := os.Getenv(confer.Opts.RedisClusterConf.StartNodes); len(startNodes) > 0 {
		confer.Opts.RedisClusterConf.StartNodes = startNodes
	}

	if startNodes := os.Getenv(confer.Opts.Log.LogRedisClusterConf.StartNodes); len(startNodes) > 0 {
		confer.Opts.Log.LogRedisClusterConf.StartNodes = startNodes
	}

	if mspURI := os.Getenv(confer.Opts.MSP.BaseURL); len(mspURI) > 0 {
		confer.Opts.MSP.BaseURL = mspURI
	}

	if startNodes := os.Getenv(confer.Opts.RateLimiter.RedisClusterNodes); len(startNodes) > 0 {
		confer.Opts.RateLimiter.RedisClusterNodes = startNodes
	}

	// influxdb
	if confer.Opts.InfluxDBConf.Enable {
		if influxdbAddress := os.Getenv(confer.Opts.InfluxDBConf.Address); len(influxdbAddress) > 0 {
			confer.Opts.InfluxDBConf.Address = influxdbAddress
		}
		host, port, err := net.SplitHostPort(confer.Opts.InfluxDBConf.Address)
		if err != nil {
			log.Println("influxdb host port is wrong err: ", err)
		} else {
			confer.Opts.InfluxDBConf.Address = host
			portInt, _ := strconv.Atoi(port)
			confer.Opts.InfluxDBConf.Port = portInt
		}
		if influxdbUserName := os.Getenv(confer.Opts.InfluxDBConf.UserName); len(influxdbUserName) > 0 {
			confer.Opts.InfluxDBConf.UserName = influxdbUserName
		}
		if influxdbUserPwd := os.Getenv(confer.Opts.InfluxDBConf.Password); len(influxdbUserPwd) > 0 {
			confer.Opts.InfluxDBConf.Password = influxdbUserPwd
		}
		if influxdbUdpAddress := os.Getenv(confer.Opts.InfluxDBConf.UdpAddress); len(influxdbUdpAddress) > 0 {
			confer.Opts.InfluxDBConf.UdpAddress = influxdbUdpAddress
		}
		if influxdbUdpDatabase := os.Getenv(confer.Opts.InfluxDBConf.Database); len(influxdbUdpDatabase) > 0 {
			confer.Opts.InfluxDBConf.Database = influxdbUdpDatabase
		}
		if influxdbUdpPrecision := os.Getenv(confer.Opts.InfluxDBConf.Precision); len(influxdbUdpPrecision) > 0 {
			confer.Opts.InfluxDBConf.Precision = influxdbUdpPrecision
		}

		// 精度全小写n, u, ms, s, m or h
		confer.Opts.InfluxDBConf.Precision = strings.ToLower(confer.Opts.InfluxDBConf.Precision)

		// influxDB 精度调整：边界条件外统一为ms
		switch confer.Opts.InfluxDBConf.Precision {
		case "n", "u", "ms", "s", "m", "h":

		default:
			confer.Opts.InfluxDBConf.Precision = "ms" // 默认精度：毫秒
		}
	}
	// app_info
	confer.replaceByEnv(&confer.Opts.AppInfo.AppID)
	confer.replaceByEnv(&confer.Opts.AppInfo.SiteID)
	confer.replaceByEnv(&confer.Opts.AppInfo.ClusterID)
	// end app_info

	// ca start
	caEnable := os.Getenv("MSP_CA_ENABLE")
	if len(caEnable) > 0 && caEnable == "true" {
		confer.Opts.CaConfig.Enable = true
	}
	confer.replaceByEnv(&confer.Opts.CaConfig.Address)
	confer.replaceByEnv(&confer.Opts.CaConfig.AuthKey)
	// end ca
	return
}

func (*Confer) replaceByEnv(conf *string) {
	if s := os.Getenv(*conf); len(s) > 0 {
		*conf = s
	}
}
