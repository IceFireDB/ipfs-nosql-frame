/*
 * @Author: gitsrc
 * @Date: 2020-07-15 18:16:03
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-28 09:41:42
 * @FilePath: /ServiceCar/sc/traffic_out_core.go
 */

package sc

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gitsrc/ipfs-nosql-frame/utils/rediscluster"

	"github.com/dgrijalva/jwt-go"
	"github.com/valyala/fasthttp"
)

const (
	serviceNotFound = "The microservice endpoint corresponding to the route was not found"
)

//HTTP 反向代理函数 ： 核心流程部分
func (sc *SC) trafficOutRequestHTTPHandle(ctx *fasthttp.RequestCtx) {

	//启动反向代理
	err := sc.trafficOutReverseFastHTTPClientHandler(ctx)
	if err != nil {
		log.Println(err)
	}
}

func (sc *SC) trafficOutReverseFastHTTPClientHandler(ctx *fasthttp.RequestCtx) (err error) {
	p := sc.TrafficOutHandleSever.Server.HTTPReverseServer //获取fasthttp Client
	req := &ctx.Request
	res := &ctx.Response

	for _, h := range hopHeaders {
		//清除client的请求header头部，避免HTTP连接状态影响
		req.Header.Del(h)
	}
	var proxyHost, proxyScheme, requestPath string
	header := map[string]string{}

	// 如果是网关承载下游流量 则 修改Req内部host字段为下游网关地址
	if sc.IsTrafficOutToGateWay {
		req.URI().SetHost(p.TargetAddress.String()) //设置反向代理目标host
		proxyHost = p.TargetAddress.String()
		proxyScheme = p.TargetType // http、https
	} else {
		// 根据sdk传递数据解析请求需要的参数
		proxyScheme, proxyHost, requestPath, header, err = sc.handleRequestRoute(req)
		if err != nil {
			res.SetStatusCode(http.StatusNotFound)
			res.SetBody([]byte(err.Error())) //注释要改
			return err
		}
	}
	log.Println(proxyScheme, proxyHost, requestPath, header)
	//此处可以针对请求进行过滤操作
	pc := p.FastHTTPCLient //获取fasthttp Client对象
	req.URI().SetHost(proxyHost)
	req.URI().SetScheme(proxyScheme)
	req.URI().SetPath(requestPath)

	//添加下游服务需要的header头部
	for k, v := range header {
		req.Header.Add(k, v)
	}

	if err = pc.Do(req, res); err != nil {
		/*响应失败：请求处理*/
		res.SetStatusCode(http.StatusInternalServerError)
		res.SetBody([]byte(err.Error())) //这一行代码不推荐这样处理，只是暂时用于错误响应
		return
	}

	for _, h := range hopHeaders {
		//清除response的请求header头部，避免HTTP连接状态影响
		res.Header.Del(h)
	}
	return
}

type topoDataS struct {
	Appid        string      `json:"appid"`
	AppKey       string      `json:"appkey"`
	Channel      string      `json:"channel"`
	Host         string      `json:"host"`
	Version      string      `json:"version"`
	Scheme       string      `json:"scheme"`
	ChannelAlias string      `json:"channel_alias"`
	Mode         string      `json:"mode"`
	ChildInfo    []topoDataS `json:"child_info"`
}

type targetBody struct {
	AppId        string `json:"appid"`
	AppKey       string `json:"appkey"`
	Channel      string `json:"channel"`
	Host         string `json:"host"`
	Scheme       string `json:"scheme"`
	ChannelAlias string `json:"channel_alias"`
	Mode         string `json:"mode"`
}
type callStack struct {
	AppId   string      `json:"appid"`
	AppKey  string      `json:"appkey"`
	Channel interface{} `json:"channel"`
	Alias   string      `json:"alias"`
}

type tokenBody struct {
	FromAppid      string      `json:"from_appid"`
	FromAppkey     string      `json:"from_appkey"`
	FromChannel    interface{} `json:"from_channel"`
	Appid          string      `json:"appid"`
	Appkey         string      `json:"appkey"`
	Channel        interface{} `json:"channel"`
	Alias          string      `json:"alias"`
	AccountId      string      `json:"account_id"`
	SuperAccountId string      `json:"super_account_id"`
	SubOrgKey      string      `json:"sub_org_key"`
	CallStack      []callStack `json:"call_stack"`
	jwt.StandardClaims
}

// 解析sdk传递的token
func getAppInfoByJwtToken(tokenString string) (*tokenBody, error) {
	if tokenString == "" {
		return nil, errors.New("token is empty")
	}
	tokenString = tokenString[7:]
	token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (i interface{}, e error) {
		return token, nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		b, err := json.Marshal(claims)
		if err != nil {
			return nil, err
		}
		tokenData := &tokenBody{}
		err = json.Unmarshal(b, tokenData)
		return tokenData, err
	}
	return nil, errors.New("token format claim error")
}

// 获取interface类型存储的string
func getInterfaceString(param interface{}) string {
	switch param.(type) {
	case string:
		return param.(string)
	case int:
		return strconv.Itoa(param.(int))
	case float64:
		return strconv.Itoa(int(param.(float64)))
	}
	return ""
}

func newMd5(str ...string) string {
	h := md5.New()
	for _, v := range str {
		h.Write([]byte(v))
	}
	return hex.EncodeToString(h.Sum(nil))
}

/**
 * 生成缓存key
 */
func makeRedisKey(k ...string) string {
	return strings.Join(k, "_")
}

/**
 * 查询redis后解析数据
 */
func (sc *SC) getChannelRelation(key, field string) (out targetBody, err error) {

	log.Println(key, field)
	channelRelation, err := rediscluster.Bytes(sc.RedisCluster.Do("HGET", key, field))
	log.Println(string(channelRelation))
	// 没有查询到redis 缓存，代表请求关系不成立
	if err == rediscluster.ErrNil {
		err = errors.New("The relation is invalid")
		return
	}

	targetData := targetBody{}
	err = json.Unmarshal(channelRelation, &targetData)
	if err != nil {
		// 解析格式错误
		err = errors.New("The relation is invalid")
		return
	}
	if targetData.AppKey == "" || targetData.Channel == "" {
		err = errors.New("The relation is invalid")
		return
	}
	return targetData, nil
}

// 验证调用链对不对
func checkChains(cs []callStack) error {
	for k, v := range cs {
		if k > 0 {
			if (v.AppId == "" || v.Alias == "") && (v.AppKey == "" || v.Channel == "" || v.Channel == "0") {
				return errors.New("The chain is invalid")
			}
		} else if v.AppId == "" || v.AppKey == "" || v.Channel == "" || v.Channel == "0" {
			return errors.New("The chain is invalid")
		}
	}
	return nil
}

/**
 * callByChain方式调用，获取field的的key
 */
func getChainFieldKey(appId, appKey, channel, alias string) string {
	if appId != "" && alias != "" {
		return makeRedisKey(appId, alias)
	} else {
		return makeRedisKey(appId, appKey, channel)
	}
}

// 处理请求，根据请求拿到目标服务地址
func (sc *SC) handleRequestRoute(req *fasthttp.Request) (scheme, host, path string, header map[string]string, err error) {
	reqUri := req.RequestURI()
	// 至少 /df911f0151f9ef021d410b4be5060972
	if len(reqUri) < 33 {
		err = errors.New("Request format error")
		return
	}
	// sdk 设置在header中的token
	headerAuthorizationData := req.Header.Peek("Authorization")
	if len(headerAuthorizationData) == 0 {
		err = errors.New("Request token invalid")
		return
	}
	// 获取router 和 request URI 部分
	//request URI 类似 /df911f0151f9ef021d410b4be5060972/user/login ，去除第一个"/"符号
	requestURIParts := bytes.Split(reqUri[1:], []byte("/"))
	// 32位路由 df911f0151f9ef021d410b4be5060972
	route := string(requestURIParts[0])
	// /user/login
	reqPath := bytes.NewBuffer(nil)
	for k, v := range requestURIParts {
		if k == 0 {
			continue
		}
		reqPath.WriteString("/")
		reqPath.Write(v)
	}
	// 请求的path
	path = reqPath.String()
	// sc 缓存中有则直接返回
	if res, ok := sc.Cache.Get(route); ok {
		targetData := res.(*targetBody)
		scheme = targetData.Scheme
		host = strings.Trim(targetData.Host, "/")
		header = map[string]string{
			"x-appid":   targetData.AppId,
			"x-appkey":  targetData.AppKey,
			"x-channel": targetData.Channel,
		}
		log.Println(targetData)
		return
	}
	// 解析token查询redis缓存
	tokenData, err := getAppInfoByJwtToken(string(headerAuthorizationData))
	if err != nil {
		return
	}
	log.Println(tokenData)
	isCallByChain, _ := strconv.Atoi(string(req.Header.Peek("x-is-chain")))
	log.Println(isCallByChain)
	// callByChain
	var targetData targetBody
	if isCallByChain > 0 {
		// 先验证整个链路格式对不对
		if err = checkChains(tokenData.CallStack); err != nil {
			return
		}
		// 遍历查询链
		var key, field string
		chainLength := len(tokenData.CallStack)
		for k, v := range tokenData.CallStack {
			if k > 0 {
				field = getChainFieldKey(v.AppId, v.AppKey, getInterfaceString(v.Channel), v.Alias)
			} else {
				key = makeRedisKey(v.AppId, v.AppKey, getInterfaceString(v.Channel))
				continue
			}
			// 查询上下级链路是否正确
			targetData, err = sc.getChannelRelation(key, field)
			if err != nil {
				return
			}
			// 有下一层则继续查询
			if k < chainLength-1 {
				key = makeRedisKey(targetData.AppId, targetData.AppKey, targetData.Channel)
			}
		}
	} else {
		// 最后一层链是目标服务
		lastNode := tokenData.CallStack[len(tokenData.CallStack)-1]
		topKey := makeRedisKey(tokenData.FromAppid, tokenData.FromAppkey, getInterfaceString(tokenData.FromChannel))
		fieldKey := getChainFieldKey(lastNode.AppId, lastNode.AppKey, getInterfaceString(lastNode.Channel), lastNode.Alias)
		targetData, err = sc.getChannelRelation(topKey, fieldKey)
		if err != nil {
			return
		}
	}
	if targetData.Host == "" || targetData.AppKey == "" {
		err = errors.New("The relation is invalid")
		return
	}
	// todo 查询到之后，缓存在高速缓存上
	sc.Cache.SetDefault(route, &targetData)
	log.Println(route, targetData)
	scheme = targetData.Scheme
	host = strings.Trim(targetData.Host, "/")
	header = map[string]string{
		"x-appid":   targetData.AppId,
		"x-appkey":  targetData.AppKey,
		"x-channel": targetData.Channel,
	}
	return
}
