package http

import (
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/logger"
	"n9e-transfer-proxy/src/config"
)

/*
下面对应都是transfer
- Request URL: http://10.178.27.152:8032/api/index/metrics 对应 QueryMetrics(recv dataobj.EndpointsRecv) *dataobj.MetricResp
- Request URL: http://10.178.27.152:8032/api/index/tagkv     对应 QueryTagPairs(recv dataobj.EndpointMetricRecv) []dataobj.IndexTagkvResp
- Request URL: http://10.178.27.152:8032/api/index/counter/fullmatch  对应 QueryIndexByFullTags(recv []dataobj.IndexByFullTagsRecv) ([]dataobj.IndexByFullTagsResp, int)
- Request URL: http://10.178.27.152:8032/api/transfer/data/ui  对应 QueryDataForUI(input dataobj.QueryDataForUI) []*dataobj.TsdbQueryResponse
- Request URL: http://xxx.com/api/mon/index/tagkv 对应 monapi  getTagkvs(c *gin.Context)
*/
func routeConfig(r *gin.Engine) {
	// executor apis

	// collector apis, compatible with open-falcon
	prefix := config.Conf().Router.RootPrefix
	if prefix == "" {
		logger.Info("prefix in config is null, change it in /api")
		prefix = "/api"
		config.Conf().Router.RootPrefix = prefix
	}

	index := r.Group(prefix + "/index")
	{
		index.POST("/metrics", GetMetrics)
		index.POST("/tagkv", GetTagPairs)
		index.POST("/counter/fullmatch", GetIndexByFullTags)
	}

	data := r.Group(prefix + "/transfer")
	{
		data.POST("/data/ui", QueryDataForUI)
	}

	mon := r.Group(prefix + "/mon")
	{
		mon.POST("/index/tagkv", GetTagkvs)
	}
}
