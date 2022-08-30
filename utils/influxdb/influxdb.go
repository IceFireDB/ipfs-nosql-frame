package influxdb

import (
	_ "gitlab.oneitfarm.com/bifrost/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "gitlab.oneitfarm.com/bifrost/influxdata/influxdb1-client/v2"
)

type InfluxDBHTTP struct {
	Client            client.Client
	BatchPointsConfig client.BatchPointsConfig
}
