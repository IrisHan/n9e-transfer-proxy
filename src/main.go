package main

import (
	"flag"
	"fmt"
	"github.com/toolkits/pkg/logger"
	"n9e-transfer-proxy/src/config"
	"n9e-transfer-proxy/src/http"
	"n9e-transfer-proxy/src/logs"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configFile := flag.String("c", "n9e-transfer-proxy.yml", "configuration file")
	flag.Parse()

	sConfig, err := config.LoadFile(*configFile)
	if err != nil {
		fmt.Println("load config fatal:", err)
		os.Exit(1)
		return
	}
	err = logs.Init(sConfig.Logger)
	if err != nil {
		fmt.Println("parse log fatal:", err)
		os.Exit(1)
		return
	}
	defer logger.Close()

	signalReceive := make(chan os.Signal, 1)
	signal.Notify(signalReceive, os.Interrupt)
	signal.Notify(signalReceive, syscall.SIGTERM, syscall.SIGQUIT)

	addr := fmt.Sprintf("0.0.0.0:%d", sConfig.HttpC.HttpListenPort)
	logger.Info("start web service Listening on address:", addr)
	errchan := make(chan error)

	go func() {
		errchan <- http.StartGin(addr, sConfig)
	}()

	select {
	case err := <-errchan:
		logger.Error("Error starting HTTP server:", err)
		return
	case <-signalReceive:
		logger.Error("Signal stop!,start stop server!")
		return
	}

}
