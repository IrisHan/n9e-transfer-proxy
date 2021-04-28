package http

import (
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/logger"
	"n9e-transfer-proxy/src/config"
	"net/http"
	"time"
)

func StartGin(listenAddr string, cg *config.Config) error {
	r := gin.New()

	//gin.SetMode(gin.ReleaseMode)
	r.Use(gin.Logger())
	m := make(map[string]*config.TransferConfig)
	for _, t := range cg.TransferConfigC {
		logger.Debugf("[get config info][region:%#v]", *t)
		m[t.RegionName] = t
	}

	r.Use(ConfigMiddleware(m))
	routeConfig(r)

	s := &http.Server{
		Addr:           listenAddr,
		Handler:        r,
		ReadTimeout:    time.Duration(10) * time.Second,
		WriteTimeout:   time.Duration(10) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	err := s.ListenAndServe()
	return err
}
