/*
 * @Author: gitsrc
 * @Date: 2020-07-17 14:20:11
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-28 09:40:54
 * @FilePath: /ServiceCar/sc/traffic_inflow_core.go
 */

package sc

import (
	"log"
	"net/http"

	"github.com/valyala/fasthttp"
)

//HTTP 反向代理函数 ： 核心流程部分
func (sc *SC) trafficInFlowRequestHTTPHandle(ctx *fasthttp.RequestCtx) {

	//启动反向代理
	err := sc.trafficInFlowReverseFastHTTPClientHandler(ctx)
	if err != nil {
		log.Println(err)
	}
}

//反向代理处理函数:使用fasthttp的Client对象
func (sc *SC) trafficInFlowReverseFastHTTPClientHandler(ctx *fasthttp.RequestCtx) (err error) {
	p := sc.TrafficInHandleSever.Server.HTTPReverseServer //获取fasthttp Client
	req := &ctx.Request
	res := &ctx.Response

	for _, h := range hopHeaders {
		//清除client的请求header头部，避免HTTP连接状态影响
		req.Header.Del(h)
	}

	//此处可以针对请求进行过滤操作

	pc := p.FastHTTPCLient //获取fasthttp Client对象

	req.SetHost(p.TargetAddress.String()) //设置反向代理目标host

	switch p.TargetType {
	case "https":
		req.URI().SetScheme("https")
	case "http":
		req.URI().SetScheme("http")
	}
	// t1 := time.Now()
	if err = pc.Do(req, res); err != nil {
		/*响应失败：请求处理
		 */
		res.SetStatusCode(http.StatusInternalServerError)
		res.SetBody([]byte(err.Error())) //这一行代码不推荐这样处理，只是暂时用于错误响应
		return
	}

	// t2 := time.Now()
	// log.Println("trffic in:", t2.Sub(t1).Milliseconds())
	for _, h := range hopHeaders {
		//清除response的请求header头部，避免HTTP连接状态影响
		res.Header.Del(h)
	}
	return
}
