/*
 * @Author: gitsrc
 * @Date: 2020-07-10 09:25:26
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-17 13:30:43
 * @FilePath: /ServiceCar/utils/networker/network.go
 */

package networker

import (
	"errors"
	"net"
	"strings"

	"github.com/gitsrc/ipfs-nosql-frame/utils/bareneter"
)

const (
	//NETWOEKLEVEL7 is level7 network
	NETWOEKLEVEL7 = 7
	//NETWOEKLEVEL5 is level7 network
	NETWOEKLEVEL5 = 5
)

const (
	//HTTPPROTOCOL is HTTP application layer protocol
	HTTPPROTOCOL = 1
	//RESPPROTOCOL is REDIS application layer protocol
	RESPPROTOCOL = 2
)

const (
	//UNIXBINDNETWORK is UNIX network monitoring category
	UNIXBINDNETWORK = 1
	//TCPBINDNETWORK is TCP network monitoring category
	TCPBINDNETWORK = 2
)

const (
	// BARESERVER represents a network server without any application layer protocol, and needs to handle network data and network status in each handle function.
	BARESERVER = 1
	// HTTPSERVER is HTTP protocol SERVER
	HTTPSERVER = 2
	// RESPSERVER is HTTP protocol SERVER
	RESPSERVER = 3
)

// NWS is short for network struct ： Core data structure
type NWS struct {
	netLevel             int      // Network level ： level7->7 level5->5
	protoclTypeStr       string   //eg: "HTTP" "RESP"
	protoclType          int      // Application layer protocol type ： eg HTTP、RESP
	bindNetWorkStr       string   //eg : unix,tcp
	bindNetWork          int      // Transport layer network monitoring type
	bindAddress          string   // Transport layer network monitoring routing
	bindIP               string   //
	bindPort             string   //
	gatewayTargetAddress string   //http proxy target address
	Server               *ServerS // NetWork bind Server
}

// ServerS is network server data and info
type ServerS struct {
	bareserver        *bareneter.Server  //裸协议服务器 暂时没用到、主要留作扩展
	HTTPReverseServer *HTTPReverseProxyS //HTTP协议服务器，支持HTTP模式的监听、用于HTTP SideCar Mode
	serverType        int
}

// GetNewNetWorker is Create a network management portal
func GetNewNetWorker(netlevel int, protoclTypeStr string, bindNetWorkStr string, bindAddressStr string, reverseProxyOptions *ReverseProxyOptions) (networker *NWS, error error) {
	if netlevel != NETWOEKLEVEL5 && netlevel != NETWOEKLEVEL7 {
		error = errors.New("Network layer is not supported")
		return
	}

	//create a networker entity
	networker = &NWS{
		netLevel:             netlevel,
		Server:               &ServerS{},
		gatewayTargetAddress: reverseProxyOptions.TargetAddress,
	}

	protoclTypeStr = strings.ToUpper(protoclTypeStr)

	protoclType := -1

	switch protoclTypeStr {
	case "HTTP":
		protoclType = HTTPPROTOCOL
	case "RESP":
		protoclType = RESPPROTOCOL
	}

	if protoclType == -1 {
		error = errors.New("Application layer protocol is not supported")
		return
	}
	networker.protoclType = protoclType
	networker.protoclTypeStr = protoclTypeStr

	bindNetWorkStr = strings.ToLower(bindNetWorkStr)

	bindNetWork := -1
	switch bindNetWorkStr {
	case "unix":
		bindNetWork = UNIXBINDNETWORK
	case "tcp":
		bindNetWork = TCPBINDNETWORK
	}

	if bindNetWork == -1 {
		error = errors.New("Network monitoring category is not supported")
		return
	}
	networker.bindNetWork = bindNetWork
	networker.bindNetWorkStr = bindNetWorkStr

	//According to the type of network monitoring, address related judgment and processing
	switch bindNetWork {

	case UNIXBINDNETWORK:
		//If it is a UNIX network
		networker.bindAddress = bindAddressStr

	default:
		//If it is a non-UNIX network, try to resolve the network listening address
		networker.bindIP, networker.bindPort, error = net.SplitHostPort(bindAddressStr)
		if error != nil {
			return
		}

		networker.bindAddress = bindAddressStr
	}

	//集中丰富NetHandle中Server类型信息，这部分和GetServerType函数挂钩，用于业务层分类运行不同的server
	switch networker.protoclTypeStr {
	case "HTTP":
		networker.Server.serverType = HTTPSERVER //如果为HTTP协议，则是HTTP服务器类型
		//创建HTTP反向代理对象
		if len(networker.gatewayTargetAddress) == 0 {
			error = errors.New("In http configuration, the upstream gateway address is not empty")
			return
		}
		networker.Server.HTTPReverseServer, error = getNewHTTPReverseProxy(reverseProxyOptions)
		if error != nil {
			return
		}
	case "RESP":
		networker.Server.serverType = RESPSERVER //如果为RESP协议，则是RESP服务器类型 ：按照BARESERVER模式进行回调
	default:
		networker.Server.serverType = BARESERVER //其他均为裸协议服务器:直接注册函数进行回调
	}

	return
}
