package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gitsrc/ipfs-nosql-frame/sc"
	"github.com/gitsrc/ipfs-nosql-frame/utils/loger"
	"github.com/gitsrc/ipfs-nosql-frame/utils/networker"
)

const configFileDefaultName = "rcp_config.yaml"

// BuildDate: Binary file compilation time
// BuildVersion: Binary compiled GIT version
var (
	BuildDate    string
	BuildVersion string
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	showBanner()
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func main() {
	var configFileURI string
	parseCmdArgs(&configFileURI) // Parse binary exec-pipe args

	// init ServiceCar entity
	ServiceCar, err := sc.GetNewServiceCar(configFileURI)
	if err != nil {
		log.Fatalf("Redis_Log_Proxy GetNewServiceCar error: %s\n", err.Error())
	}

	// catch main-panic
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Redis_Log_Proxy (mainModules.catchPanic) %v\n", err)
			loger.LogErrorw(loger.LogNameDefault, "Redis_Log_Proxy (mainModules.catchPanic) %v\n", fmt.Errorf("%v", err))
		}
	}()

	// 探测配置文件，并决定是否开启pprof性能监控
	if ServiceCar.Confer.Opts.DebugConf.Enable {
		go func(pprofUri string) {
			var err error

			defer func() {
				if err != nil {
					log.Printf("Redis_Log_Proxy pprof listen error: %s\n", err.Error())
					loger.LogErrorw(loger.LogNameDefault, "Redis_Log_Proxy pprof listen error: %v\n", fmt.Errorf("%v", err))
				}
			}()

			log.Printf("github.com/gitsrc/ipfs-nosql-frame pprof listen on: %s\n", pprofUri)
			loger.LogInfof(loger.LogNameDefault, "github.com/gitsrc/ipfs-nosql-frame pprof listen on: %s\n", pprofUri)
			err = http.ListenAndServe(pprofUri, nil)

			if err != nil {
				return
			}
		}(ServiceCar.Confer.Opts.DebugConf.PprofURI)
	}

	// 开启 Traffic In Handle Server
	if ServiceCar.Confer.Opts.TrafficInflow.Enable {
		go func() {
			log.Println("start Traffic Inflow http Server.", ServiceCar.TrafficInHandleSever.GetBindNetWorkStr(), ServiceCar.TrafficInHandleSever.GetBindAddress(), "=>", ServiceCar.TrafficInHandleSever.GetGatewayTargetAddress())
			err = ServiceCar.StartTrafficInflowHTTPServer()

			if err != nil {
				log.Println(err)
			}
		}()
	}
	go func() {
		// 开启Traffic Out Handle Server
		bindServerType := ServiceCar.TrafficOutHandleSever.GetServerType()
		switch bindServerType {
		case networker.HTTPSERVER:
			log.Println("start http Server.", ServiceCar.TrafficOutHandleSever.GetBindNetWorkStr(), ServiceCar.TrafficOutHandleSever.GetBindAddress(), "=>", ServiceCar.TrafficOutHandleSever.GetGatewayTargetAddress())
			err = ServiceCar.StartTrafficOutReverseHTTPServer()
		case networker.RESPSERVER:
			log.Println("start RESP Server: ", ServiceCar.TrafficOutHandleSever.GetBindNetWorkStr(), ServiceCar.TrafficOutHandleSever.GetBindAddress())
			loger.LogInfow(loger.LogNameDefault, "start RESP Server: "+ServiceCar.TrafficOutHandleSever.GetBindNetWorkStr()+ServiceCar.TrafficOutHandleSever.GetBindAddress())
			err = ServiceCar.TrafficOutHandleSever.StartBareServer(ServiceCar.ServerTLSConfig, ServiceCar.RequestHandleRESP, ServiceCar.AcceptHandleRESP, ServiceCar.ClosedHandleRESP)
		default:
			panic("start Bare Server.")
		}
		if err != nil {
			loger.LogErrorw(loger.LogNameDefault, "Redis_Log_Proxy main error", err)
			panic(err)
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for sig := range sigs {
		switch sig {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			if ServiceCar.Confer.Opts.CaConfig.Enable {
				_ = ServiceCar.Exchanger.RevokeItSelf()
			}
			os.Exit(0)
		case syscall.SIGHUP:
			fmt.Println("+++++++++++++++++++++++++++++")
		}
	}
}

func parseCmdArgs(configURI *string) {
	dir, err := os.Getwd()
	if err != nil {
		dir = configFileDefaultName
	} else {
		dir += "/" + configFileDefaultName
	}

	// 设置解析参数 -c ,并设置默认配置文件路径为 os.Getwd()/CONFIG_FILE_DEFAULT_NAME
	flag.StringVar(configURI, "c", dir, "Config file path")

	flag.Parse()
}

func showBanner() {
	bannerData := `  +===============================+
  |    /$$$$$$         /$$$$$$    |
  |   /$$__  $$       /$$__  $$   |
  |  | $$  \__/      | $$  \__/   |
  |  |  $$$$$$       | $$         |
  |   \____  $$      | $$         |
  |   /$$  \ $$      | $$    $$   |
  |  |  $$$$$$/      |  $$$$$$/   |
  |   \______/        \______/    |				  
  +===============================+`
	fmt.Println(bannerData)
	fmt.Println("Build Version: ", BuildVersion, "  Date: ", BuildDate)
}
