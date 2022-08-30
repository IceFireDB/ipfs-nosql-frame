/*
 * @Author: gitsrc
 * @Date: 2020-07-09 09:34:42
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-08-19 18:53:16
 * @FilePath: /redis-cluster-proxy/sc/confer/models.go
 */

package confer

import (
	"sync"
)

// Confer is top data struct of confer
type Confer struct {
	Mutex sync.RWMutex
	Opts  scConfS
}

// scConfS is Subordinate configuration
type scConfS struct {
	TrafficInflow    TrafficInflowS   `yaml:"traffic_inflow"` // TrafficInflow Carry upstream request traffic and undertake governance work
	GateWay          GatewayS         `yaml:"gateway"`        // GateWay configure block
	L5NetConf        L5NetConfS       `yaml:"net-L5"`         // level5 net configuration block
	L7NetConf        L7NetConfS       `yaml:"net-L7"`         // level7 net configuration block
	RedisClusterConf RedisClusterConf `yaml:"redis-cluster"`  // redis cluster
	MemoryCacheConf  MemoryCacheS     `yaml:"memory-cache"`   // Middleware memory cache configuration
	DebugConf        DebugConfS       `yaml:"debug"`          // Middleware debug configuration
	AppInfo          AppConfS         `yaml:"app_info"`       // Middleware appInfo configuration
	Log              LogConfS         `yaml:"log"`            // Middleware Log configuration
	MSP              MSPS             `yaml:"msp"`            // Middleware MSP configuration
	Filter           FilterS          `yaml:"filter"`         // Middleware filter configuration
	HTTPOption       HTTPOptS         `yaml:"http"`           // Middleware http configuration
	RateLimiter      RateLimiterS     `yaml:"rate_limiter"`   // Middleware rate_limiter configuration
	InfluxDBConf     InfluxDBConfig   `yaml:"influxdb"`       // influxdb config
	CaConfig         CaConfigS        `yaml:"ca_config"`      // ca config
}

/*
rate_limiter:
  redis_cluster_nodes: 192.168.2.80:9001,192.168.2.80:9002,192.168.2.80:9003,192.168.2.80:9004,192.168.2.80:9005,192.168.2.80:9006
*/
type RateLimiterS struct {
	RedisClusterNodes string `yaml:"redis_cluster_nodes"`
}

// HTTPOptS is http configure struct
type HTTPOptS struct {
	Enable    bool   `yaml:"enable"`     // Filter switch option
	ListenURI string `yaml:"listen_uri"` // Get the URL PATH address of the rule
}

// MSPS is msp data struct
type MSPS struct {
	BaseURL string `yaml:"base_url"` // URL base address of the MSP address
}

// FilterS is filter data struct
type FilterS struct {
	Enable                 bool   `yaml:"enable"`                    // Filter switch option
	RuleDataURLPath        string `yaml:"rule_data_url_path"`        // Get the URL PATH address of the rule
	RuleHTTPRequestTimeout int    `yaml:"rule_http_request_timeout"` // Filter data source http request timeout : second
	RuleUpdateHeartBeat    int    `yaml:"rule_update_heart_beat"`    // Rule data update heartbeat
}

// TrafficInflowS Carry upstream request traffic and undertake governance work
type TrafficInflowS struct {
	Enable                    bool   `yaml:"enable"` // 是否开启流量入口拦截
	BindProtocolType          string `yaml:"bind_protocol_type"`
	BindNetWork               string `yaml:"bind_network"`
	BindAddress               string `yaml:"bind_address"`
	TargetProtocolType        string `yaml:"target_protocol_type"`
	TargetAddress             string `yaml:"target_address"`
	TargetDialTimeout         int    `yaml:"target_dial_timeout"`            // 到gateway的tcp拨号超时时间、单位：秒
	TargetKeepAlive           int    `yaml:"target_keep_alive"`              // 到gateway的keepalive超时
	TargetIdleConnTimeout     int    `yaml:"target_idle_conn_timeout"`       // 到gateway的tcp连接的空闲超时时间
	TargetMaxIdleConnsPerHost int    `yaml:"target_max_idle_conns_per_host"` // 关键参数，到gateway的连接池最大容量可以为多少
}

// GatewayS is gateway configure options
type GatewayS struct {
	Enable                    bool   `yaml:"enable"` // 是否开启网关转发、如果不开启 则 进行指定路由redis查询
	ProtocolType              string `yaml:"protocol_type"`
	TargetAddress             string `yaml:"target_address"`
	TargetDialTimeout         int    `yaml:"target_dial_timeout"`            // 到gateway的tcp拨号超时时间、单位：秒
	TargetKeepAlive           int    `yaml:"target_keep_alive"`              // 到gateway的keepalive超时
	TargetIdleConnTimeout     int    `yaml:"target_idle_conn_timeout"`       // 到gateway的tcp连接的空闲超时时间
	TargetMaxIdleConnsPerHost int    `yaml:"target_max_idle_conns_per_host"` // 关键参数，到gateway的连接池最大容量可以为多少
}

// L5NetConfS is level5 net configure options
type L5NetConfS struct {
	ListenURI string `yaml:"listen_uri"` // Network entry for middleware monitoring
}

// L7NetConfS is level7 net configure options
type L7NetConfS struct {
	ProtocolType string `yaml:"protocol_type"` // Application layer protocol type
	BindNetwork  string `yaml:"bind_network"`  // Transport layer network monitoring type
	BindAddress  string `yaml:"bind_address"`  // Transport layer network monitoring routing
}

// RedisClusterConf is redis cluster configure options
type RedisClusterConf struct {
	Enable bool `yaml:"enable"`
	// Cluster start node string, which can contain multiple nodes, separated by commas, multiple nodes are found for high availability
	StartNodes string `yaml:"start_nodes"`

	ConnTimeOut      int `yaml:"conn_timeout"`       // Connection timeout parameter of cluster nodes Unit: ms
	ConnReadTimeOut  int `yaml:"conn_read_timeout"`  // Cluster node read timeout parameter Unit: ms
	ConnWriteTimeOut int `yaml:"conn_write_timeout"` // Cluster node write timeout parameter Unit: ms
	ConnAliveTimeOut int `yaml:"conn_alive_timeout"` // Cluster node TCP idle survival time Unit: seconds
	ConnPoolSize     int `yaml:"conn_pool_size"`     // The size of the TCP connection pool for each node in the cluster

	// Cluster read-write separation function, the percentage of slave node carrying read traffic: 0-> slave node does not carry any read traffic
	SlaveOperateRate int `yaml:"slave_operate_rate"`

	// Redis cluster status update heartbeat interval: only effective for scenarios where read-write separation is enabled
	ClusterUpdateHeartbeat int `yaml:"cluster_update_heartbeat"`
}

// MemoryCacheS is memory cache configure options
type MemoryCacheS struct {
	Enable            bool `yaml:"enable"`
	MaxItemsCount     int  `yaml:"max_items_count"`    // The maximum number of items in the cache
	DefaultExpiration int  `yaml:"default_expiration"` // cache kv default expiration time (unit: ms)
	CleanupInterval   int  `yaml:"cleanup_interval"`   // cache expiry kv cleanup period (unit: seconds)
}

// DebugConfS is debug configure options
type DebugConfS struct {
	Enable   bool   `yaml:"enable"`
	PprofURI string `yaml:"pprof_uri"` // Middleware performance analysis listening address
}

// AppConfS is App configure options
type AppConfS struct {
	AppName    string `yaml:"app_name"`
	AppID      string `yaml:"app_id"`
	AppVersion string `yaml:"app_version"`
	AppKey     string `yaml:"app_key"`
	Channel    string `yaml:"channel"`
	SubOrgKey  string `yaml:"sub_org_key"`
	Language   string `yaml:"language"`
	SiteID     string `yaml:"site_id"`
	ClusterID  string `yaml:"cluster_id"`
}

// LogConfS is log configure options
type LogConfS struct {
	OutPut              string           `yaml:"output"`
	Debug               bool             `yaml:"debug"`
	LogKey              string           `yaml:"key"`
	ScLogNum            int              `yaml:"sc_log_num"`
	LogRedisClusterConf RedisClusterConf `yaml:"redis-cluster"` // redis cluster
}

// InfluxDBConfig
type InfluxDBConfig struct {
	Enable              bool   `yaml:"enable"` // 服务开关
	Address             string `yaml:"address"`
	Port                int    `yaml:"port"`
	UdpAddress          string `yaml:"udp_address"` // influxdb 数据库的udp地址，ip:port
	Database            string `yaml:"database"`    // 数据库名称
	Precision           string `yaml:"precision"`   // 精度 n, u, ms, s, m or h
	UserName            string `yaml:"username"`
	Password            string `yaml:"password"`
	MaxIdleConns        int    `yaml:"max-idle-conns"`
	MaxIdleConnsPerHost int    `yaml:"max-idle-conns-per-host"`
	IdleConnTimeout     int    `yaml:"idle-conn-timeout"`
}

type CaConfigS struct {
	Enable  bool   `yaml:"enable"`
	AuthKey string `yaml:"auth_key"`
	Address string `yaml:"address"`
}
