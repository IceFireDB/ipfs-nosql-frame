/*
 * @Author: gitsrc
 * @Date: 2020-07-10 13:10:26
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-10 13:54:17
 * @FilePath: /ServiceCar/utils/networker/bareserver.go
 */

package networker

import (
	"crypto/tls"

	"github.com/gitsrc/ipfs-nosql-frame/utils/bareneter"
)

// StartBareServer is start a Bare protocol server.
func (networker *NWS) StartBareServer(
	tlsc *tls.Config,
	handler func(conn bareneter.Conn),
	accept func(conn bareneter.Conn) bool,
	closed func(conn bareneter.Conn, err error),
) (err error) {
	err = bareneter.ListenAndServe(networker.bindNetWorkStr, networker.bindAddress, tlsc, handler, accept, closed)
	return
}
