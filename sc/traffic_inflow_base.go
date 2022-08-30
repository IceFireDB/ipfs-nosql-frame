/*
 * @Author: gitsrc
 * @Date: 2020-07-17 13:24:46
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-17 14:22:06
 * @FilePath: /ServiceCar/sc/traffic_inflow_base.go
 */

package sc

import (
	"os"

	"github.com/valyala/fasthttp"
)

var trafficInflowHopHeaders = []string{
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

// StartTrafficInflowHTTPServer is start traffic inflow HTTP listener
func (sc *SC) StartTrafficInflowHTTPServer() (err error) {
	//sc.NetworkerL7.Server.HTTPServer.ReverseHandler()

	bindNetType := sc.TrafficInHandleSever.GetBindNetWorkStr()
	bindNetAddress := sc.TrafficInHandleSever.GetBindAddress()
	if bindNetType == "unix" {
		//If the file does not exist, do not operate
		if _, statErr := os.Stat(bindNetAddress); os.IsNotExist(statErr) {
			return
		}

		err = os.Remove(bindNetAddress)

		if err != nil {
			return err
		}
	}

	switch bindNetType {
	case "unix":
		fasthttp.ListenAndServeUNIX(bindNetAddress, 0X777, sc.trafficInFlowRequestHTTPHandle)
	case "tcp":
		fasthttp.ListenAndServe(bindNetAddress, sc.trafficInFlowRequestHTTPHandle)
	}
	return
}
