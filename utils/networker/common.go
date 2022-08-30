/*
 * @Author: gitsrc
 * @Date: 2020-07-10 12:44:47
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-17 13:30:32
 * @FilePath: /ServiceCar/utils/networker/common.go
 */

package networker

/*
const (
	// BARESERVER represents a network server without any application layer protocol, and needs to handle network data and network status in each handle function.
	BARESERVER = 1
	// HTTPSERVER is HTTP protocol SERVER
	HTTPSERVER = 2
)
*/

// GetServerType is get NetWorker server type
func (networker *NWS) GetServerType() int {
	return networker.Server.serverType
}

// GetBindAddress is get NetWorker bind address
func (networker *NWS) GetBindAddress() string {
	return networker.bindAddress
}

// GetBindNetWorkStr is get NetWorker bind NetWork string
func (networker *NWS) GetBindNetWorkStr() string {
	return networker.bindNetWorkStr
}

// GetGatewayTargetAddress is get NetWorker gatewayTargetAddress string
func (networker *NWS) GetGatewayTargetAddress() string {
	return networker.gatewayTargetAddress
}
