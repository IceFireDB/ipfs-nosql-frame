/*
 * @Author: gitsrc
 * @Date: 2020-08-05 11:24:39
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-10-16 15:09:39
 * @FilePath: /log-redis-cluster-proxy/sc/filter/filter.go
 */

package filter

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gitsrc/ipfs-nosql-frame/sc/ratelimit"
	"github.com/gitsrc/ipfs-nosql-frame/utils/influxdb"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	"github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"

	client "gitlab.oneitfarm.com/bifrost/influxdata/influxdb1-client/v2"

	"github.com/go-redis/redis/v8"
	"gitlab.oneitfarm.com/bifrost/ratelimiter"
)

//
//var count = &Count{}
//
//type Count struct {
//	Pass int32
//	Dis  int32
//}
//
//func AddPass() {
//	count.AddPass()
//}
//
//func AddDis() {
//	count.AddDis()
//}
//
//func PrintNum() {
//	if atomic.LoadInt32(&count.Pass) > 0 {
//		fmt.Println(atomic.LoadInt32(&count.Dis)/atomic.LoadInt32(&count.Pass), atomic.LoadInt32(&count.Pass), atomic.LoadInt32(&count.Dis))
//	}
//}
//
//func (c *Count) AddPass() {
//	atomic.AddInt32(&c.Pass, 1)
//}
//
//func (c *Count) AddDis() {
//	atomic.AddInt32(&c.Dis, 1)
//}

// Filter is core data struct
type Filter struct {
	enable                    bool            // 开关选项
	ruleDataURL               string          // 规则数据源接口URL
	ruleDataUpdateHeartBeat   int             // 规则数据更新心跳
	ruleHTTPRequestTimeout    int             // 规则数据源 HTTP请求超时时间
	rateLimitRedisClusterAddr string          // 限流器的redis地址
	FilterRuleData            filterRuleDataS // 规则数据存储单元
}

// RuleMapType is filter rule map data struct
type RuleMapType map[string]logRuleUnitS

type filterRuleDataS struct {
	RuleDataMAP RuleMapType // 规则数据存储单元
	dataHash    [md5.Size]byte
	sync.RWMutex
}

type logRuleUnitS struct {
	LogKey          string               `json:"log_key"`
	RateLimit       int                  `json:"rate_limit"`
	SamplingRate    uint32               `json:"sampling_rate"`  // 采样率，默认为0，当大于0时，计算规则为 1/(SamplingRate-1)
	Expire          int                  `json:"expire"`         // 过期时间，每次rpush都会执行，单位s，默认600
	Length          int64                `json:"length"`         // 当前长度
	WarningLength   int64                `json:"warning_length"` // 预警长度，默认1000
	ClusterNodeAddr string               // logkey 在集群的节点分布
	RateLimiter     *ratelimiter.Limiter // 限流器
	SilenceTime     time.Duration        // 事件上报静默时间周期 TODO
}

type httpRespDataS struct {
	Code        int            `json:"code"`
	LogKeyRules []logRuleUnitS `json:"data"`
	Message     string         `json:"message"`
}

// GetNewFilter is create a filter
func GetNewFilter(enable bool, ruleDataURL string, ruleDataUpdateHeartBeat int, ruleHTTPRequestTimeout int, rateLimitRedisClusterAddr string) (filter *Filter) {
	filter = &Filter{
		enable:                    enable,
		ruleDataURL:               ruleDataURL,
		ruleDataUpdateHeartBeat:   ruleDataUpdateHeartBeat,
		ruleHTTPRequestTimeout:    ruleHTTPRequestTimeout,
		rateLimitRedisClusterAddr: rateLimitRedisClusterAddr,
	}
	return
}

// StartRuleMonitor is Rule data source monitoring
func (filter *Filter) StartRuleMonitor(redisCluster *rediscluster.Cluster, logProxyRedisKey string, scLogNum int) (err error) {
	defer func() {
		// Catch StartRuleMonitor Panic
		if err := recover(); err != nil {
			loger.LogErrorw(loger.LogNameRedis, "StartRuleMonitor catchPanic", fmt.Errorf("%v", err))
			return
		}
	}()

	// 如果没有开启filter功能，则直接返回
	if !filter.enable {
		return
	}

	loger.LogInfow(loger.LogNameDefault, "Start filter data source monitoring")
	isFirstLoop := true
	respData := &httpRespDataS{}
OUTER:
	for {
		respDataIsRenew := false // logkey slice 重构标识：当为true代表重构内存
		if isFirstLoop {         // 如果是第一次循环，则禁用sleep
			isFirstLoop = false
		} else {
			time.Sleep(time.Second * time.Duration(filter.ruleDataUpdateHeartBeat))
		}
		// 获取远程Rule数据
		respOriginData, err := getRuleDataFromHTTP(filter.ruleDataURL, time.Duration(filter.ruleHTTPRequestTimeout)*time.Second)

		if err != nil {
			loger.LogErrorw(loger.LogNameApi, "Filter RuleMonitor: Failed to request MSP remote data source.", err)
			// continue 网络IO即使错误，也进入更新流程，因为需要构建日志代理本身的日志key
			// 如果长度为0，则证明内部没有数据,增加rcp_log值
			if len(respData.LogKeyRules) == 0 {
				respData.LogKeyRules = append(respData.LogKeyRules, logRuleUnitS{LogKey: logProxyRedisKey, RateLimit: -1})
			} else {
				continue // 网络IO错误，并且有数据，则忽略后续操作，避免空数据覆盖
			}
		} else {
			// 如果成功，则证明网络IO正常，重置内存
			respDataTemp := &httpRespDataS{}
			err = json.Unmarshal(respOriginData, respDataTemp)
			// json解析失败
			if err != nil {
				loger.LogErrorw(loger.LogNameDefault, "Filter RuleMonitor: json decoding failed.", err)
				if len(respData.LogKeyRules) == 0 {
					respData.LogKeyRules = append(respData.LogKeyRules, logRuleUnitS{LogKey: logProxyRedisKey, RateLimit: -1})
				} else {
					continue // 网络IO正常 但json解析失败，并且有数据，则忽略后续操作，避免空数据覆盖
				}
			} else {
				// json解析成功，更新respData
				respData = respDataTemp
				respDataIsRenew = true
			}
		}
		// 此处正常流程，进行数据更新操作，更新前进行hash计算，避免无谓的内存并发同步损耗
		respDataHash := md5.Sum(respOriginData) //nolint:gosec    // 计算当前HTTP-body的Hash校验和，用来进行匹配
		filterDataHash := filter.GetDataHash()  // 获取源数据的hash值，判断http响应数据是否发生变化
		// 如果HTTP解析成功 并且 新数据的hash数据和旧filter数据hash相同，则忽略此次更新操作，
		if filterDataHash != respDataHash {
			loger.LogInfow(loger.LogNameDefault, "感应到日志代理规则变化："+string(respOriginData))
		}
		// respDataHTTP解析成功，追加logProxy,并且接口数据有变，追加日志key
		if respDataIsRenew {
			respData.LogKeyRules = append(respData.LogKeyRules, logRuleUnitS{LogKey: logProxyRedisKey, RateLimit: -1})
		}
		//if scLogNum > 0 {
		//	// 判断是否存在sc_log
		//	for _, value := range respData.LogKeyRules {
		//		if value.LogKey == "sc_log" {
		//			for i := 1; i <= scLogNum; i++ {
		//				logRuleUnitS := logRuleUnitS{
		//					RateLimit:       value.RateLimit,
		//					LogKey:          fmt.Sprintf("sc_log_%d", i),
		//					ClusterNodeAddr: value.ClusterNodeAddr,
		//				}
		//				respData.LogKeyRules = append(respData.LogKeyRules, logRuleUnitS)
		//			}
		//		}
		//	}
		//}
		// 数据发生变化、进行新的数据构建
		NewFilterRuleData := make(RuleMapType)
		rateLimitRedisClusterAddr := filter.rateLimitRedisClusterAddr

		rateLimitRedisClusterAddrArr := strings.Split(rateLimitRedisClusterAddr, ",")
		var redisOptions = redis.ClusterOptions{Addrs: rateLimitRedisClusterAddrArr}
		for _, data := range respData.LogKeyRules {
			nodeAddr, err := redisCluster.GetNodeAddrByKey(data.LogKey)
			if err != nil {
				loger.LogErrorf(loger.LogNameDefault, "Filter RuleMonitor: rediscluster.GetNodeAddrByKey failed. (%s) %v", data.LogKey, err)
				continue OUTER // 跳出当前循环 忽略此次更新。
			}
			var limiter *ratelimiter.Limiter
			rateLimit := data.RateLimit
			if rateLimit >= 0 {
				// 仅仅针对开启限流的key才初始化限流器
				clusterClient := redis.NewClusterClient(&redisOptions)
				limiterTemp, err := ratelimiter.New(context.Background(), ratelimiter.Options{
					Client:   &ratelimit.ClusterClient{ClusterClient: clusterClient},
					Duration: time.Second, // 修改限流周期为1s
					Max:      rateLimit})
				if err != nil {
					loger.LogErrorf(loger.LogNameDefault, "Filter RuleMonitor: ratelimiter.New failed. (%s) %v", data.LogKey, err)
					limiter = nil
					continue OUTER // 跳出当前循环 忽略此次更新。 ：创建限流器失败
				}
				limiter = limiterTemp // 无错成功赋值
			}
			// 查询每个key当前的长度
			var keyLen int64
			keyLen, err = rediscluster.Int64(redisCluster.Do("LLEN", data.LogKey))
			if err != nil {
				loger.LogErrorw(loger.LogNameApi, "redisCluster.Do(llen, "+data.LogKey+") err", err)
			}
			if data.LogKey == "sc_log" {
				for i := 1; i <= scLogNum; i++ {
					respKey := fmt.Sprintf("%s_%d", data.LogKey, i)
					length, err := rediscluster.Int64(redisCluster.Do("LLEN", respKey))
					if err != nil {
						length = 0
						loger.LogErrorw(loger.LogNameApi, "redisCluster.Do(llen, "+respKey+") err", err)
					}
					keyLen += length
				}
			}
			NewFilterRuleData[data.LogKey] = logRuleUnitS{
				RateLimit:       data.RateLimit,
				LogKey:          data.LogKey,
				SamplingRate:    data.SamplingRate,
				Expire:          data.Expire,
				Length:          keyLen,
				WarningLength:   data.WarningLength,
				RateLimiter:     limiter,
				ClusterNodeAddr: nodeAddr,
			}
		}
		// 进行数据内存同步，让写锁的粒度最小
		filter.FilterRuleData.Lock()
		filter.FilterRuleData.RuleDataMAP = NewFilterRuleData
		filter.FilterRuleData.dataHash = respDataHash
		filter.FilterRuleData.Unlock()
	}
}

// GetData 获取内存数据
func (filter *Filter) GetData() (data RuleMapType) {
	filter.FilterRuleData.RLock()
	defer filter.FilterRuleData.RUnlock()

	//filter.FilterRuleData.RuleDataMAP 是个MAP结构，如果直接上报，则会造成Data race，因此需要深度拷贝
	data = make(RuleMapType)
	for k, v := range filter.FilterRuleData.RuleDataMAP {
		data[k] = v
	}
	return
}

// GetPingData 获取内存数据
func (filter *Filter) GetPingData(redisCluster *rediscluster.Cluster, scLogNum int) (data RuleMapType) {
	filter.FilterRuleData.RLock()
	defer filter.FilterRuleData.RUnlock()

	//filter.FilterRuleData.RuleDataMAP 是个MAP结构，如果直接上报，则会造成Data race，因此需要深度拷贝
	data = make(RuleMapType)
	for k, v := range filter.FilterRuleData.RuleDataMAP {
		if v.LogKey == "sc_log" {
			for i := 1; i <= scLogNum; i++ {
				logKey := fmt.Sprintf("sc_log_%d", i)
				nodeAddr, err := redisCluster.GetNodeAddrByKey(logKey)
				if err != nil {
					loger.LogErrorf(loger.LogNameDefault, "Filter RuleMonitor: rediscluster.GetNodeAddrByKey failed. (%s) %v", logKey, err)
					continue
				}
				logRuleUnitS := logRuleUnitS{
					LogKey:          logKey,
					ClusterNodeAddr: nodeAddr,
				}
				data[logKey] = logRuleUnitS
			}
		}
		data[k] = v
	}
	return
}

// GetDataHash is get data hash content
func (filter *Filter) GetDataHash() [md5.Size]byte {
	filter.FilterRuleData.RLock()
	defer filter.FilterRuleData.RUnlock()

	return filter.FilterRuleData.dataHash
}

func (filter *Filter) GetMSPRateLimit(id string) int {
	if len(strings.TrimSpace(id)) > 0 {
		data := filter.GetData()
		if len(data) > 0 {
			if logRuleUnitS, ok := data[id]; ok {
				return logRuleUnitS.RateLimit
			}
		}
	}
	return -1
}

func (filter *Filter) ReportKeyRateAndLen(redisCluster *rediscluster.Cluster, influxdb *influxdb.InfluxDBHTTP) {
	defer func() {
		// Catch ReportKeyRate Panic
		if err := recover(); err != nil {
			loger.LogErrorw(loger.LogNameRedis, "Start ReportKeyRate catchPanic", fmt.Errorf("%v", err))
			return
		}
	}()
	// 如果没有开启filter功能，则直接返回
	if !filter.enable {
		return
	}
	loger.LogInfow(loger.LogNameDefault, "Start ReportKeyRate data source monitoring")
	bp, err := client.NewBatchPoints(influxdb.BatchPointsConfig)
	if err != nil {
		loger.LogErrorw(loger.LogNameDefault, "influxdb client.NewBatchPoints err", err)
		return
	}
	for {
		// 10个points上传一次,1s统计一次
		time.Sleep(time.Second)
		// 获取最新的key列表
		RuleMapType := filter.GetData()
		if len(RuleMapType) > 0 {
			for k := range RuleMapType {
				// 获取k对应的速率
				var rate int
				result, err := rediscluster.Values(redisCluster.Do("hmget", "LIMIT:"+k, "kr"))
				if err != nil {
					loger.LogErrorw(loger.LogNameRedis, "rediscluster.Values err", err)
					continue
				}
				if len(result) > 0 && result[0] != nil {
					data, err := rediscluster.String(result[0], nil)
					if err != nil {
						fmt.Println(data)
						loger.LogErrorw(loger.LogNameDefault, "rediscluster.String err", err)
						continue
					}
					rateSli := strings.Split(data, ".")
					if len(rateSli) > 0 {
						rateStr := rateSli[0]
						rateInt, err := strconv.Atoi(rateStr)
						if err != nil {
							loger.LogErrorw(loger.LogNameDefault, "strconv.Atoi err", err)
							continue
						}
						rate = rateInt
					}
				}
				// 获取key的长度
				keyLen, err := rediscluster.Int64(redisCluster.Do("LLEN", k))
				if rate == 0 && keyLen == 0 {
					continue
				}
				tags := map[string]string{
					"log_key": k,
				}
				fields := map[string]interface{}{
					"rate": rate,
					"llen": keyLen,
				}
				pt, err := client.NewPoint("log_key_rate", tags, fields, time.Now())
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
					bp, err = client.NewBatchPoints(influxdb.BatchPointsConfig)
					if err != nil {
						loger.LogErrorw(loger.LogNameDefault, "influxdb client.NewBatchPoints err", err)
					}
				}
			}
		}
	}
}

func getRuleDataFromHTTP(url string, timeout time.Duration) (respData []byte, err error) {
	httpClient := http.Client{
		Timeout: timeout,
	}
	// Will throw error as it's not quick enough
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("resp is nil")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP request response is not 200")
	}
	respData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	return
}
