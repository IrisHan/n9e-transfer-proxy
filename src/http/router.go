package http

import (
	"github.com/gin-gonic/gin"
)

/*
下面对应都是transfer
- Request URL: http://127.0.0.1:8032/api/index/metrics 对应 QueryMetrics(recv dataobj.EndpointsRecv) *dataobj.MetricResp
- Request URL: http://127.0.0.1:8032/api/index/tagkv     对应 QueryTagPairs(recv dataobj.EndpointMetricRecv) []dataobj.IndexTagkvResp
- Request URL: http://127.0.0.1:8032/api/index/counter/fullmatch  对应 QueryIndexByFullTags(recv []dataobj.IndexByFullTagsRecv) ([]dataobj.IndexByFullTagsResp, int)
- Request URL: http://127.0.0.1:8032/api/transfer/data/ui  对应 QueryDataForUI(input dataobj.QueryDataForUI) []*dataobj.TsdbQueryResponse

*/
func routeConfig(r *gin.Engine) {
	// executor apis

	// collector apis, compatible with open-falcon
	index := r.Group("/api/index")
	{
		index.POST("/metrics", GetMetrics)
		index.POST("/tagkv", GetTagPairs)
		index.POST("/counter/fullmatch", GetIndexByFullTags)
	}

	data := r.Group("/api/transfer")
	{
		data.POST("/data/ui", QueryDataForUI)
	}
}
