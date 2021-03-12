package http

import (
	"github.com/gin-gonic/gin"
	src "n9e-transfer-proxy"
	"n9e-transfer-proxy/src/config"
)

// ApiMiddleware will add the db connection to the context
func ConfigMiddleware(m map[string]*config.TransferConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(src.CONFIG_TRANSFER, m)
		c.Next()
	}
}
