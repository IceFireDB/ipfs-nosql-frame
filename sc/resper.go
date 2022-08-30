/*
 * @Author: gitsrc
 * @Date: 2020-07-10 11:10:07
 * @LastEditors: gitsrc
 * @LastEditTime: 2021-06-22 19:32:49
 * @FilePath: /redis-cluster-proxy/sc/resper.go
 */

package sc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gitsrc/ipfs-nosql-frame/sc/alter"
	"github.com/gitsrc/ipfs-nosql-frame/sc/event"
	"github.com/gitsrc/ipfs-nosql-frame/utils/bareneter"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	"github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"
	"github.com/gitsrc/ipfs-nosql-frame/utils/resparse/credis"
	"github.com/gitsrc/ipfs-nosql-frame/utils/respecho/RedSHandle"

	"github.com/valyala/fastrand"
)

// RequestHandleRESP is Bare Server client request
func (sc *SC) RequestHandleRESP(conn bareneter.Conn) {
	defer func() {
		// Catch RequestHandle Panic
		if err := recover(); err != nil {
			loger.LogErrorw(loger.LogNameRedis, "Redis Cluster proxy loger RequestHandle catchPanic", fmt.Errorf("%v", err))
			_ = conn.Close() // 关键一行，避免假死的client Conn
			return
		}
	}()

	// 获取客户端conn对象，并在conn对象上构建RedSHandle对象
	localConn := conn.NetConn()
	localWriteHandle := RedSHandle.NewWriterHandle(localConn)
	decoder := credis.NewDecoderSize(localConn, 1024) // 内存不复用：创建解析器对象
	for {
		resp, err := decoder.Decode()
		// 如果RESP协议解码失败，则报错，返回客户端异常 并 上报日志
		if err != nil {
			_ = localWriteHandle.WriteError("use of failed decoder")
			_ = localWriteHandle.Flush()
			// 如果错误不是 io.EOF && 错误不是 "read: connection reset by peer"错误
			if err.Error() != io.EOF.Error() && (strings.Index(err.Error(), "read: connection reset by peer") == -1) {
				if !sc.Confer.Opts.CaConfig.Enable || err.Error() != "tls: first record does not look like a TLS handshake" {
					loger.LogErrorw(loger.LogNameRedis, "use of failed decoder", err)
				}
			}
			_ = conn.Close()
			return
		}

		// 如果resp类型不是数组类型
		if resp.Type != credis.TypeArray {
			errResp := "RESP COMMAND RESP TYPE NOT SUPPORT."
			_ = localWriteHandle.WriteError(errResp)
			_ = localWriteHandle.Flush()
			loger.LogErrorw(loger.LogNameRedis, errResp, errors.New(errResp))
			_ = conn.Close()
			return
		}
		respCount := len(resp.Array)
		// 如果客户端传过来的协议数据解析后的数据组过少，则证明客户端传过来的数据有问题，不进行后续处理
		if respCount < 1 {
			errResp := "COMMAND RESP COUNT TOO LESS."
			_ = localWriteHandle.WriteError(errResp)
			_ = localWriteHandle.Flush()
			loger.LogErrorw(loger.LogNameRedis, errResp, errors.New(errResp))
			_ = conn.Close()
			return
		}
		if resp.Array[0].Type != credis.TypeBulkBytes {
			errResp := "COMMAND CMD TYPE WRONG."
			_ = localWriteHandle.WriteError(errResp)
			_ = localWriteHandle.Flush()
			loger.LogErrorw(loger.LogNameRedis, errResp, errors.New(errResp))
			_ = conn.Close()
			return
		}
		cmdType := strings.ToUpper(string(resp.Array[0].Value))
		respDataArray := resp.Array
		scLogNum := sc.Confer.Opts.Log.ScLogNum
		switch cmdType {
		case "RPUSH":
			// 如果参数个数小于等于2 代表后面没有参数
			if respCount <= 2 {
				err = errors.New("the number of RPUSH command args is wrong. ")
				_ = localWriteHandle.WriteError(err.Error())
				_ = localWriteHandle.Flush()
				loger.LogErrorw(loger.LogNameRedis, "RPUSH COMMAND args number is  error.", err)
				_ = conn.Close()
				return
			}
			respKey := string(respDataArray[1].Value)

			// 进行filter过滤
			// 如果respKey不在Filter白名单里面，则进行日志发送行为拒绝
			if !sc.Filter.IsExistInFilterList(respKey) {
				errResp := "The log key is not in the filter whitelist"
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()

				alter.LogAlert(alter.LogAlertMsg{
					EventType:     event.LOG_KEY_NOTIN_WHITE_LIST,
					EventTime:     time.Now().UnixNano() / 1e6,
					LogKey:        respKey,
					RateThreshold: 0,
				})
				loger.LogWarnw(loger.LogNameRedis, errResp+" "+respKey)
				_ = conn.Close()
				return
			}

			rateLimiter, samplingRate, expire, lengthDrop := sc.Filter.GetFilterRule(respKey)
			if expire <= 0 {
				expire = 600
			}

			// 判断是否有采样，有的话执行相关逻辑
			if samplingRate > 0 {
				if fastrand.Uint32n(samplingRate) != 0 {
					// 代表采样失败，直接返回
					errResp := "this log has been discarded because of sampling rate"
					_ = localWriteHandle.WriteError(errResp)
					_ = localWriteHandle.Flush()
					_ = conn.Close()
					return
				}
			}

			// 判断长度是否超出阈值
			if lengthDrop {
				// 代表当前key的长度已经超出阈值，直接丢弃
				errResp := "this log has been discarded because of too long length"
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				_ = conn.Close()
				return
			}

			if scLogNum > 0 && respKey == "sc_log" {
				// 随机取一个
				randNum := fastrand.Uint32n(uint32(scLogNum + 1))
				if randNum != 0 {
					respKey = fmt.Sprintf("%s_%d", respKey, randNum)
				}
				// 当randNum为0时，走原来流程，key依旧为sc_log
			}
			// 进行限流判断
			// 当rateLimiter不为nil时进行限流查询。
			if rateLimiter != nil {
				ret, err := rateLimiter.Get(context.Background(), respKey)
				if err != nil {
					loger.LogErrorw(loger.LogNameRedis, "ratelimiter.Get(respKey)", err)
				}
				// 速率统计
				// 同时判断限流器无容量且真实的速率限制大于0 ：如果限流器被消耗完 并且 MSP配置限流阈值大于0 则进行限流
				if ret.Remaining < 0 && sc.Filter.GetMSPRateLimit(respKey) > 0 {
					// 激发限流
					errResp := "Key Speed ​​limited->" + respKey
					_ = localWriteHandle.WriteError(errResp)
					_ = localWriteHandle.Flush()
					// 采样率 0.1%
					if fastrand.Uint32n(1000) == 0 {
						loger.LogErrorf(loger.LogNameDefault, "log key : %s has been rate limited", respKey)
						// alter.LogAlert(event.LOG_RATE_LIMIT, respKey, ret.Total)
						alter.LogAlert(alter.LogAlertMsg{
							EventType:     event.LOG_RATE_LIMIT,
							EventTime:     time.Now().UnixNano() / 1e6,
							LogKey:        respKey,
							RateThreshold: ret.Total,
						})
					}
					_ = conn.Close()
					return
				}
			}
			// 统计
			sc.Reporter.AddRpushCommand(string(respDataArray[1].Value))
			commandArgs := make([]interface{}, 0)
			commandArgs = append(commandArgs, respKey, respDataArray[2].Value)
			// 加上过期时间,默认10分钟(600s)
			commandArgs = append(commandArgs, expire)
			// 返回列表长度
			reply, err := rediscluster.Int64(sc.RedisCluster.Do(cmdType, commandArgs...))
			if err != nil {
				errResp := "RPUSH COMMAMDN EXEC FAIL."
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				loger.LogErrorw(loger.LogNameRedis, errResp, err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.WriteInt(reply)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "RPUSH localWriteHandle write error.", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "RPUSH localWriteHandle flush error.", err)
				_ = conn.Close()
				return
			}
		case "LPOP":
			// 如果参数个数小于等于2 代表后面没有参数
			if respCount != 2 {
				err = errors.New("the number of LPOP command args is wrong. ")
				_ = localWriteHandle.WriteError(err.Error())
				_ = localWriteHandle.Flush()
				loger.LogErrorw(loger.LogNameRedis, "LPOP COMMAND args number is  error.", err)
				_ = conn.Close()
				return
			}
			logKey := string(respDataArray[1].Value)
			if strings.Contains(logKey, "sc_log") {
				sc.Reporter.AddLpopCommand("sc_log")
			} else {
				sc.Reporter.AddLpopCommand(logKey)
			}
			reply, err := rediscluster.Bytes(sc.RedisCluster.Do(cmdType, respDataArray[1].Value))
			// redis数据读取指令的特殊之处：nil回复判断
			// 如果执行结果出错，判断是否为空结果错误，如果为空错误，则给客户端返回nil，否则报错
			if err != nil && !errors.Is(err, rediscluster.ErrNil) {
				errResp := "LPOP COMMAMDN EXEC FAIL."
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				err = fmt.Errorf("LPOP %s", err.Error())
				loger.LogErrorw(loger.LogNameRedis, errResp, err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.WriteBulk(reply)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "LPOP localWriteHandle write error.", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "LPOP localWriteHandle flush error.", err)
				_ = conn.Close()
				return
			}
		case "BLPOP":
			// 目前限定BLPOP命令限制为一个key
			if respCount != 3 {
				err = errors.New("the number of BLPOP command args is wrong. ")
				_ = localWriteHandle.WriteError(err.Error())
				_ = localWriteHandle.Flush()
				loger.LogErrorw(loger.LogNameRedis, "BLPOP COMMAND args number is  error.", err)
				_ = conn.Close()
				return
			}
			logKey := string(respDataArray[1].Value)
			if strings.Contains(logKey, "sc_log") {
				sc.Reporter.AddLpopCommand("sc_log")
			} else {
				sc.Reporter.AddLpopCommand(logKey)
			}
			reply, err := rediscluster.Values(sc.RedisCluster.Do(cmdType, respDataArray[1].Value, respDataArray[2].Value))
			// redis数据读取指令的特殊之处：nil回复判断
			// 如果执行结果出错，判断是否为空结果错误，如果为空错误，则给客户端返回nil，否则报错
			if err != nil && !errors.Is(err, rediscluster.ErrNil) {
				errResp := "BLPOP COMMAMDN EXEC FAIL."
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				err = fmt.Errorf("BLPOP %s", err.Error())
				loger.LogErrorw(loger.LogNameRedis, errResp, err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.WriteObjects(reply...)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "BLPOP localWriteHandle write error.", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "BLPOP localWriteHandle flush error.", err)
				_ = conn.Close()
				return
			}
		case "LLEN":
			if respCount < 2 {
				errResp := "the number of LLEN command args is wrong."
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				loger.LogError(loger.LogNameRedis, "LLEN COMMAND args number is  error.")
				_ = conn.Close()
				return
			}
			reply, err := rediscluster.Int64(sc.RedisCluster.Do(cmdType, respDataArray[1].Value))
			if err != nil {
				errResp := "LLEN COMMAMDN EXEC FAIL."
				_ = localWriteHandle.WriteError(errResp)
				_ = localWriteHandle.Flush()
				err = fmt.Errorf("LLEN %s", err.Error())
				loger.LogErrorw(loger.LogNameRedis, errResp, err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.WriteInt(reply)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "LLEN command local write int err", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "LLEN command local flush int err", err)
				_ = conn.Close()
				return
			}
		case "PING":
			reply := "PONG"
			if respCount > 1 {
				reply = string(respDataArray[1].Value)
				if strings.ToUpper(reply) == "KEYSDATA" {
					data := sc.Filter.GetPingData(sc.RedisCluster, sc.Confer.Opts.Log.ScLogNum)
					jsonData := make(map[string]string)
					for _, item := range data {
						jsonData[item.LogKey] = item.ClusterNodeAddr
					}
					dataBytes, err := json.Marshal(jsonData)
					if err != nil {
						loger.LogErrorw(loger.LogNameRedis, "PING KEYSDATA json.Marshal", err)
					} else {
						reply = string(dataBytes)
					}
				}
			}
			err = localWriteHandle.WriteSimpleString(reply)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "PING localWriteHandle write error.", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "PING localWriteHandle flush error.", err)
				_ = conn.Close()
				return
			}
		case "CLUSTERKEYNODE":
			reply := "KEY is NULL"
			if respCount > 1 {
				key := string(respDataArray[1].Value)
				nodeStr, err := sc.RedisCluster.GetNodeAddrByKey(key)
				if err != nil {
					loger.LogErrorw(loger.LogNameRedis, "CLUSTERKEYNODE GetNodeAddrByKey error.", err)
					_ = conn.Close()
					return
				}
				reply = nodeStr
			}
			err = localWriteHandle.WriteSimpleString(reply)
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "CLUSTERKEYNODE localWriteHandle write error.", err)
				_ = conn.Close()
				return
			}
			err = localWriteHandle.Flush()
			if err != nil {
				loger.LogErrorw(loger.LogNameRedis, "CLUSTERKEYNODE localWriteHandle flush error.", err)
				_ = conn.Close()
				return
			}
		default:
			errResp := fmt.Sprintf("COMMAND(%s) NOT SUPPORT.", cmdType)
			_ = localWriteHandle.WriteError(errResp)
			_ = localWriteHandle.Flush()
			loger.LogErrorw(loger.LogNameRedis, errResp, errors.New(errResp))
			_ = conn.Close()
			return
		}
	}
}

// AcceptHandleRESP is Bare Server client accept
func (sc *SC) AcceptHandleRESP(bareneter.Conn) bool {
	defer func() {
		// Catch RequestHandle Panic
		if err := recover(); err != nil {
			log.Printf("Redis_Log_Proxy  AcceptHandleRESP (Redis_Log_Proxy.catchPanic) %v\n", err)
			loger.LogErrorw(loger.LogNameDefault, "github.com/gitsrc/ipfs-nosql-frame  AcceptHandle (ServiceCar.catchPanic) %v\n", fmt.Errorf("%v", err))
		}
	}()
	return true
}

// ClosedHandleRESP is Bare Server client close
func (sc *SC) ClosedHandleRESP(_ bareneter.Conn, err error) {
	if err != nil {
		loger.LogErrorw(loger.LogNameDefault, "client close err: %s", err)
	}
	defer func() {
		// Catch RequestHandle Panic
		if err := recover(); err != nil {
			log.Printf("Redis_Log_Proxy  ClosedHandleRESP (Redis_Log_Proxy.catchPanic) %v\n", err)
			loger.LogErrorw(loger.LogNameDefault, "Redis_Log_Proxy  ClosedHandleRESP (Redis_Log_Proxy.catchPanic) %v\n", fmt.Errorf("%v", err))
		}
	}()
}
