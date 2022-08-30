/*
 * @Author: gitsrc
 * @Date: 2020-07-10 13:10:30
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-27 13:20:04
 * @FilePath: /ServiceCar/utils/networker/httpproxy.go
 */

package networker

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	fasthttp "github.com/valyala/fasthttp"
)

// HTTPReverseProxyS is http reverse proxy data struct
type HTTPReverseProxyS struct {
	TargetAddress *url.URL
	TargetType    string // http 、 https

	ReverseProxy       *httputil.ReverseProxy
	FastHTTPHostCLient *fasthttp.HostClient
	FastHTTPCLient     *fasthttp.Client
}

// ReverseProxyOptions is reverse proxy options
type ReverseProxyOptions struct {
	ProtocolType              string //
	TargetAddress             string
	TargetDialTimeout         time.Duration //目标地址拨号超时时间
	TargetKeepAlive           time.Duration //目标地址keepalive超时
	TargetIdleConnTimeout     time.Duration //空闲连接多久过期
	TargetMaxIdleConnsPerHost int           //反向代理的每个gateway host 最大的连接数
}

// getNewHTTPReverseProxy is http reverse proxy creator : local function
func getNewHTTPReverseProxy(reverseProxyOptions *ReverseProxyOptions) (proxy *HTTPReverseProxyS, err error) {

	//进行目标地址变量的格式校验
	targetAddress := reverseProxyOptions.TargetAddress
	targetAddressParts := strings.Split(targetAddress, ":")
	if len(targetAddressParts) == 2 && strings.ToUpper(targetAddressParts[0]) == "SYSENV" {
		//读取环境变量、进行gateway地址赋值
		targetAddress = os.Getenv(targetAddressParts[1])
	}

	if targetAddress == "" {
		return nil, errors.New("Failed to obtain gateway address from system environment variables")
	}

	url, err := url.Parse(targetAddress) //解析targetAddress地址，进行合法性检测
	if err != nil {
		return nil, err
	}

	//构建HTTPReverseProxyS结构体对象、并注入关键参数
	proxy = &HTTPReverseProxyS{
		TargetAddress: url,
		TargetType:    reverseProxyOptions.ProtocolType,
	}

	//增加golang http util 标准 ReverseProxy
	proxy.ReverseProxy = &httputil.ReverseProxy{
		Director: newDirector(url),
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				//log.Println("CALLING DIAL")
				conn, err := (&net.Dialer{
					Timeout:   reverseProxyOptions.TargetDialTimeout,
					KeepAlive: reverseProxyOptions.TargetKeepAlive,
				}).Dial(network, addr)
				return conn, err
			},
			// DialContext: (&net.Dialer{ //dial context 写法
			// 	Timeout:   reverseProxyOptions.TargetDialTimeout,
			// 	KeepAlive: reverseProxyOptions.TargetKeepAlive,
			// 	DualStack: true,
			// }).DialContext,
			//ForceAttemptHTTP2: true,
			//MaxIdleConns:        100,
			IdleConnTimeout: reverseProxyOptions.TargetIdleConnTimeout,
			//TLSHandshakeTimeout: 5 * time.Second,
			MaxIdleConnsPerHost: reverseProxyOptions.TargetMaxIdleConnsPerHost,
		},
	}

	//增加Fasthttp hostclient对象，这个对象不能进行协议流转、比如HTTP to HTTPS
	proxy.FastHTTPHostCLient = &fasthttp.HostClient{
		Addr:                targetAddress,
		MaxConns:            reverseProxyOptions.TargetMaxIdleConnsPerHost,
		MaxConnDuration:     reverseProxyOptions.TargetKeepAlive,
		MaxIdleConnDuration: reverseProxyOptions.TargetIdleConnTimeout,
		ReadBufferSize:      1024,
		WriteBufferSize:     1024,
	}

	//增加Fasthttp 标准Client对象，支持HTTP->HTTPS 协议流转 ： 目前主力选用
	proxy.FastHTTPCLient = &fasthttp.Client{
		MaxConnsPerHost:           reverseProxyOptions.TargetMaxIdleConnsPerHost, //针对每个下游host控制的长连接数量
		MaxConnDuration:           reverseProxyOptions.TargetKeepAlive,
		MaxIdleConnDuration:       reverseProxyOptions.TargetIdleConnTimeout,
		ReadBufferSize:            1024,
		WriteBufferSize:           1024,
		MaxIdemponentCallAttempts: 0,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, reverseProxyOptions.TargetDialTimeout)
		},
	}

	return

}

//以下为实验代码：目前不选用

// ReverseFastHTTPClientHandler is http reverse by fasthttp Client Support TLS
// func (p *HTTPReverseProxyS) ReverseFastHTTPClientHandler(ctx *fasthttp.RequestCtx) (err error) {
// 	req := &ctx.Request
// 	res := &ctx.Response

// 	pc := p.FastHTTPCLient

// 	req.SetHost(p.targetAddress.String())

// 	req.URI().SetScheme("https")
// 	if err = pc.Do(req, res); err != nil {
// 		log.Printf("could not proxy: %v\n", err)
// 		res.SetStatusCode(http.StatusInternalServerError)
// 		res.SetBody([]byte(err.Error()))
// 		return
// 	}
// 	return

// }

// ReverseFastHTTPHandler is http reverse by fasthttp
func (p *HTTPReverseProxyS) ReverseFastHTTPHandler(ctx *fasthttp.RequestCtx) (err error) {
	req := &ctx.Request
	res := &ctx.Response

	pc := p.FastHTTPHostCLient
	for _, h := range hopHeaders {
		//清除client的请求header头部，避免HTTP连接状态影响
		req.Header.Del(h)
	}

	req.SetHost(pc.Addr)

	if err = pc.Do(req, res); err != nil {
		log.Printf("could not proxy: %v\n", err)
		res.SetStatusCode(http.StatusInternalServerError)
		res.SetBody([]byte(err.Error()))
		return
	}

	for _, h := range hopHeaders {
		//清除response的请求header头部，避免HTTP连接状态影响
		res.Header.Del(h)
	}

	return

}

// ReverseHandler is the route handler which must be bound to the routes
func (p *HTTPReverseProxyS) ReverseHandler(res http.ResponseWriter, req *http.Request) {

	// This launches a new Go routine under the hood and therefore it's non blocking
	p.ReverseProxy.ServeHTTP(res, req)
}

func newDirector(targetURL *url.URL) func(req *http.Request) {
	targetQuery := targetURL.RawQuery

	return func(req *http.Request) {
		// Update headers to support SSL redirection
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.URL.Path = singleJoiningSlash(targetURL.Path, req.URL.Path)
		req.Host = targetURL.Host
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

var hopHeaders = []string{
	"Connection",          // Connection
	"Proxy-Connection",    // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",          // Keep-Alive
	"Proxy-Authenticate",  // Proxy-Authenticate
	"Proxy-Authorization", // Proxy-Authorization
	"Te",                  // canonicalized version of "TE"
	"Trailer",             // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",   // Transfer-Encoding
	"Upgrade",             // Upgrade
}
