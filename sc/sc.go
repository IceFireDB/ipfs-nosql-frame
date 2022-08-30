/*
 * @Author: gitsrc
 * @Date: 2020-07-09 09:05:47
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-09-07 16:09:12
 * @FilePath: /redis-cluster-proxy/sc/sc.go
 */

package sc

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	confer "github.com/gitsrc/ipfs-nosql-frame/sc/confer"
	sconfer "github.com/gitsrc/ipfs-nosql-frame/sc/confer"
	"github.com/gitsrc/ipfs-nosql-frame/sc/event"
	"github.com/gitsrc/ipfs-nosql-frame/sc/filter"
	"github.com/gitsrc/ipfs-nosql-frame/sc/metrics"
	"github.com/gitsrc/ipfs-nosql-frame/utils/influxdb"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	cache "github.com/gitsrc/ipfs-nosql-frame/utils/memorycacher"
	networker "github.com/gitsrc/ipfs-nosql-frame/utils/networker"
	rediscluster "github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"

	"gitlab.oneitfarm.com/bifrost/capitalizone/pkg/keygen"

	"gitlab.oneitfarm.com/bifrost/capitalizone/pkg/caclient"
	"gitlab.oneitfarm.com/bifrost/capitalizone/pkg/spiffe"
	cv2 "gitlab.oneitfarm.com/bifrost/cilog/v2"
	client "gitlab.oneitfarm.com/bifrost/influxdata/influxdb1-client/v2"
)

const ServerName = "log-proxy"

// SC is ServiceCar core data struct.
type SC struct {
	Confer                *confer.Confer
	RedisCluster          *rediscluster.Cluster
	LogRedisCluster       *rediscluster.Cluster // log redis cluster
	Cache                 *cache.Cache
	TrafficInHandleSever  *networker.NWS         // Layer 7 networker
	TrafficOutHandleSever *networker.NWS         // Layer 7 networker
	IsTrafficOutToGateWay bool                   // 出口网络是不是出口到GateWay ： true代表由中心网关承载流量，false代表基于服务发现机制
	Filter                *filter.Filter         // 日志过滤器
	Reporter              *metrics.Reporter      // 监控相关
	InfluxDBHTTP          *influxdb.InfluxDBHTTP // influxdb HTTP client
	ServerTLSConfig       *tls.Config
	Exchanger             *caclient.Exchanger // ca证书中心
	sync.RWMutex
}

// GetNewServiceCar is ServiceCar creator.
func GetNewServiceCar(confFileURI string) (sc *SC, err error) {
	sc = &SC{}
	// Get a confer
	sc.Confer, err = sconfer.GetNewConfer(confFileURI)
	if err != nil {
		return
	}
	// 如果开启了RedisCluster配置选项，则进行RedisCluster实例的初始化
	// Create Redis cluster entity
	if sc.Confer.Opts.RedisClusterConf.Enable {
		sc.RedisCluster, err = rediscluster.NewCluster(
			&rediscluster.Options{
				StartNodes:             strings.Split(sc.Confer.Opts.RedisClusterConf.StartNodes, ","),
				ConnTimeout:            time.Duration(sc.Confer.Opts.RedisClusterConf.ConnTimeOut) * time.Millisecond,
				ReadTimeout:            time.Duration(sc.Confer.Opts.RedisClusterConf.ConnReadTimeOut) * time.Millisecond,
				WriteTimeout:           time.Duration(sc.Confer.Opts.RedisClusterConf.ConnWriteTimeOut) * time.Millisecond,
				KeepAlive:              sc.Confer.Opts.RedisClusterConf.ConnPoolSize,
				AliveTime:              time.Duration(sc.Confer.Opts.RedisClusterConf.ConnAliveTimeOut) * time.Second,
				SlaveOperateRate:       sc.Confer.Opts.RedisClusterConf.SlaveOperateRate,
				ClusterUpdateHeartbeat: sc.Confer.Opts.RedisClusterConf.ClusterUpdateHeartbeat,
			})

		if err != nil {
			return
		}
	}

	// Force open middleware cache option
	sc.Confer.Opts.MemoryCacheConf.Enable = true

	// Create a middleware cache entity: If the Cache option is enabled
	if sc.Confer.Opts.MemoryCacheConf.Enable {
		sc.Cache = cache.New(
			time.Millisecond*time.Duration(sc.Confer.Opts.MemoryCacheConf.DefaultExpiration),
			time.Second*time.Duration(sc.Confer.Opts.MemoryCacheConf.CleanupInterval),
			sc.Confer.Opts.MemoryCacheConf.MaxItemsCount,
		)
	}

	// 根据配置创建Traffic Out networker
	trafficOutReverseProxyOptions := &networker.ReverseProxyOptions{
		ProtocolType:              strings.ToLower(sc.Confer.Opts.GateWay.ProtocolType),
		TargetAddress:             sc.Confer.Opts.GateWay.TargetAddress,
		TargetDialTimeout:         time.Duration(sc.Confer.Opts.GateWay.TargetDialTimeout) * time.Millisecond,
		TargetKeepAlive:           time.Duration(sc.Confer.Opts.GateWay.TargetKeepAlive) * time.Second,
		TargetIdleConnTimeout:     time.Duration(sc.Confer.Opts.GateWay.TargetIdleConnTimeout) * time.Second,
		TargetMaxIdleConnsPerHost: sc.Confer.Opts.GateWay.TargetMaxIdleConnsPerHost,
	}

	sc.TrafficOutHandleSever, err = networker.GetNewNetWorker(
		networker.NETWOEKLEVEL7,
		sc.Confer.Opts.L7NetConf.ProtocolType,
		sc.Confer.Opts.L7NetConf.BindNetwork,
		sc.Confer.Opts.L7NetConf.BindAddress,
		trafficOutReverseProxyOptions)

	if err != nil {
		return
	}

	// 根据配置创建Traffic in networker
	trafficInflowReverseProxyOptions := &networker.ReverseProxyOptions{
		ProtocolType:              strings.ToLower(sc.Confer.Opts.TrafficInflow.TargetProtocolType),
		TargetAddress:             sc.Confer.Opts.TrafficInflow.TargetAddress,
		TargetDialTimeout:         time.Duration(sc.Confer.Opts.TrafficInflow.TargetDialTimeout) * time.Millisecond,
		TargetKeepAlive:           time.Duration(sc.Confer.Opts.TrafficInflow.TargetKeepAlive) * time.Second,
		TargetIdleConnTimeout:     time.Duration(sc.Confer.Opts.TrafficInflow.TargetIdleConnTimeout) * time.Second,
		TargetMaxIdleConnsPerHost: sc.Confer.Opts.TrafficInflow.TargetMaxIdleConnsPerHost,
	}

	sc.TrafficInHandleSever, err = networker.GetNewNetWorker(
		networker.NETWOEKLEVEL7,
		sc.Confer.Opts.TrafficInflow.BindProtocolType,
		sc.Confer.Opts.TrafficInflow.BindNetWork,
		sc.Confer.Opts.TrafficInflow.BindAddress,
		trafficInflowReverseProxyOptions)

	if err != nil {
		return
	}

	sc.IsTrafficOutToGateWay = sc.Confer.Opts.GateWay.Enable

	// log redis cluster object
	sc.LogRedisCluster, err = rediscluster.NewCluster(
		&rediscluster.Options{
			StartNodes:             strings.Split(sc.Confer.Opts.Log.LogRedisClusterConf.StartNodes, ","),
			ConnTimeout:            time.Duration(sc.Confer.Opts.Log.LogRedisClusterConf.ConnTimeOut) * time.Millisecond,
			ReadTimeout:            time.Duration(sc.Confer.Opts.Log.LogRedisClusterConf.ConnReadTimeOut) * time.Millisecond,
			WriteTimeout:           time.Duration(sc.Confer.Opts.Log.LogRedisClusterConf.ConnWriteTimeOut) * time.Millisecond,
			KeepAlive:              sc.Confer.Opts.Log.LogRedisClusterConf.ConnPoolSize,
			AliveTime:              time.Duration(sc.Confer.Opts.Log.LogRedisClusterConf.ConnAliveTimeOut) * time.Second,
			SlaveOperateRate:       sc.Confer.Opts.Log.LogRedisClusterConf.SlaveOperateRate,
			ClusterUpdateHeartbeat: sc.Confer.Opts.Log.LogRedisClusterConf.ClusterUpdateHeartbeat,
		})

	if err != nil {
		return
	}
	// 创建日志对象
	configLogData := &loger.ConfigLogData{
		OutPut:       sc.Confer.Opts.Log.OutPut, // redis 输出到日志中心redis ，stdout 输出到终端
		Debug:        sc.Confer.Opts.Log.Debug,  // 目前无用，后期增加输出stack功能
		Key:          sc.Confer.Opts.Log.LogKey,
		RedisCluster: sc.LogRedisCluster,
	}
	configAppData := &loger.ConfigAppData{
		AppName:    sc.Confer.Opts.AppInfo.AppName,
		AppID:      sc.Confer.Opts.AppInfo.AppID,
		AppVersion: sc.Confer.Opts.AppInfo.AppVersion,
		AppKey:     sc.Confer.Opts.AppInfo.AppKey,
		Channel:    sc.Confer.Opts.AppInfo.Channel,
		SubOrgKey:  sc.Confer.Opts.AppInfo.SubOrgKey,
		Language:   sc.Confer.Opts.AppInfo.Language,
	}
	// 初始化推送事件客户端
	event.InitEventClient(sc.RedisCluster)
	err = loger.ConfigLogInit(configLogData, configAppData)

	if err != nil {
		return
	}

	// 创建日志过滤器
	sc.Filter = filter.GetNewFilter(sc.Confer.Opts.Filter.Enable,
		sc.Confer.Opts.MSP.BaseURL+sc.Confer.Opts.Filter.RuleDataURLPath,
		sc.Confer.Opts.Filter.RuleUpdateHeartBeat,
		sc.Confer.Opts.Filter.RuleHTTPRequestTimeout,
		sc.Confer.Opts.RateLimiter.RedisClusterNodes,
	)

	// 启动日志规则远程数据监听
	go sc.Filter.StartRuleMonitor(sc.RedisCluster, sc.Confer.Opts.Log.LogKey, sc.Confer.Opts.Log.ScLogNum)

	// 创建监控相关
	sc.Reporter = metrics.NewReporter()
	// 上报数据到msp
	go sc.Reporter.ReportData(sc.LogRedisCluster, sc.Filter)

	// 初始化influxdb udp client 开始
	if sc.Confer.Opts.InfluxDBConf.Enable {
		httpClient, err := client.NewHTTPClient(client.HTTPConfig{
			Addr:                fmt.Sprintf("http://%s:%d", sc.Confer.Opts.InfluxDBConf.Address, sc.Confer.Opts.InfluxDBConf.Port),
			Username:            sc.Confer.Opts.InfluxDBConf.UserName,
			Password:            sc.Confer.Opts.InfluxDBConf.Password,
			MaxIdleConns:        sc.Confer.Opts.InfluxDBConf.MaxIdleConns,
			MaxIdleConnsPerHost: sc.Confer.Opts.InfluxDBConf.MaxIdleConnsPerHost,
			IdleConnTimeout:     90 * time.Second,
			Timeout:             10 * time.Second,
		})
		if err != nil {
			loger.LogErrorw(loger.LogNameDefault, "redis-log-proxy NewUDPClient error", err)
			return nil, err
		}
		batchPointsConfig := client.BatchPointsConfig{
			Precision: sc.Confer.Opts.InfluxDBConf.Precision,
			Database:  sc.Confer.Opts.InfluxDBConf.Database,
		}
		sc.InfluxDBHTTP = &influxdb.InfluxDBHTTP{Client: httpClient, BatchPointsConfig: batchPointsConfig}
		// 上报数据到时序
		go sc.Reporter.ReportMetrics(sc.InfluxDBHTTP, sc.Filter)
		go sc.Reporter.ClearKey(sc.Filter)
		// 启动速率统计上报
		// go sc.Filter.ReportKeyRateAndLen(sc.RedisCluster, sc.InfluxDBHTTP)
	}
	// 启动mtls
	if sc.Confer.Opts.CaConfig.Enable {
		err = sc.StartTLSConn()
		if err != nil {
			return nil, err
		}
		sc.ServerTLSConfig, err = sc.NewSidecarTLSServer()
	}
	return
}

func (sc *SC) StartTLSConn() error {
	// tls相关功能控制
	if !sc.Confer.Opts.CaConfig.Enable {
		return nil
	}
	// 重试机制
	retry := 0 // 当前第多少次重试
	for {
		if sc.Exchanger != nil {
			break
		}
		err := sc.newMTLS()
		if err != nil {
			retry++
			sleepSecond := retry * 5
			if sleepSecond > 120 {
				sleepSecond = 120
			}
			loger.LogWarnf(loger.LogNameDefault, "第%d次初始化ca失败：%s, %d秒后重试", retry, err.Error(), sleepSecond)
			time.Sleep(time.Second * time.Duration(sleepSecond))
			continue
		}
		break
	}
	return nil
}

func (sc *SC) newMTLS() (err error) {
	if !sc.Confer.Opts.CaConfig.Enable {
		return nil
	}
	l, _ := cv2.NewZapLogger(&cv2.Conf{
		Level: 2,
	})
	c := caclient.NewCAI(caclient.WithCAServer(caclient.RoleSidecar, sc.Confer.Opts.CaConfig.Address),
		caclient.WithAuthKey(sc.Confer.Opts.CaConfig.AuthKey), caclient.WithLogger(l),
		caclient.WithCSRConf(keygen.CSRConf{
			SNIHostnames: []string{ServerName},
		}))
	exchanger, err := c.NewExchanger(&spiffe.IDGIdentity{
		SiteID:    sc.Confer.Opts.AppInfo.SiteID,
		ClusterID: sc.Confer.Opts.AppInfo.ClusterID,
		UniqueID:  sc.Confer.Opts.AppInfo.AppID,
	})
	if err != nil {
		return err
	}
	_, err = exchanger.Transport.GetCertificate()
	if err != nil {
		return err
	}
	// 启动证书轮换
	go exchanger.RotateController().Run()

	sc.Exchanger = exchanger
	return nil
}

// mTLS Client 使用示例
// func (tp *TrafficProxy) NewSidecarMTLSClient(validatorRemote func(remoteUniqueId string) error) (*tls.Config, error) {
func (sc *SC) NewSidecarMTLSClient() (*tls.Config, error) {
	cfger, err := sc.Exchanger.ClientTLSConfig("")
	if err != nil {
		return nil, err
	}
	return cfger.TLSConfig(), nil
}

func (sc *SC) NewSidecarTLSServer() (*tls.Config, error) {
	var tlsCfg *caclient.TLSGenerator
	var err error
	// mtls
	tlsCfg, err = sc.Exchanger.ServerTLSConfig()
	if err != nil {
		return nil, err
	}

	return tlsCfg.TLSConfig(), nil
}
