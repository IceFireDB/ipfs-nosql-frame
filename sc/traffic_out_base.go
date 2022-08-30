/*
 * @Author: gitsrc
 * @Date: 2020-07-10 11:09:46
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-27 14:55:59
 * @FilePath: /ServiceCar/sc/traffic_out_base.go
 */

package sc

import (
	"os"

	"github.com/valyala/fasthttp"
)

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
	// "Content-Length",
}

// StartTrafficOutReverseHTTPServer is start  traffic out HTTP listener
func (sc *SC) StartTrafficOutReverseHTTPServer() (err error) {
	//sc.NetworkerL7.Server.HTTPServer.ReverseHandler()

	bindNetType := sc.TrafficOutHandleSever.GetBindNetWorkStr()
	bindNetAddress := sc.TrafficOutHandleSever.GetBindAddress()
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
		fasthttp.ListenAndServeUNIX(bindNetAddress, 0X777, sc.trafficOutRequestHTTPHandle)
	case "tcp":
		fasthttp.ListenAndServe(bindNetAddress, sc.trafficOutRequestHTTPHandle)
	}
	return
}
