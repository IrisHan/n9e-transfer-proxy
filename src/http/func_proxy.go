package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/didi/nightingale/src/common/dataobj"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/errors"
	"github.com/toolkits/pkg/logger"
	"io/ioutil"
	src "n9e-transfer-proxy"
	"n9e-transfer-proxy/src/config"
	"net/http"
	"sync"
	"time"
)

type MetricsResp struct {
	Dat dataobj.MetricResp `json:"dat"`
	Err string             `json:"err"`
}

type TagkvResp struct {
	Dat []dataobj.IndexTagkvResp `json:"dat"`
	Err string                   `json:"err"`
}

type FullMatchResp struct {
	Dat struct {
		List  []dataobj.IndexByFullTagsResp `json:"list"`
		Count int                           `json:"count"`
	} `json:"dat"`
	Err string `json:"err"`
}

type DataForUIResp struct {
	Dat []dataobj.QueryDataForUIResp `json:"dat"`
	Err string                       `json:"err"`
}

func renderMessage(c *gin.Context, v interface{}) {
	if v == nil {
		c.JSON(200, gin.H{"err": ""})
		return
	}

	switch t := v.(type) {
	case string:
		c.JSON(200, gin.H{"err": t})
	case error:
		c.JSON(200, gin.H{"err": t.Error()})
	}
}

func renderData(c *gin.Context, data interface{}, err string) {
	c.JSON(200, gin.H{"dat": data, "err": err})
	return

}

func GetMetrics(c *gin.Context) {

	recv := dataobj.EndpointsRecv{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	transM := c.MustGet(src.CONFIG_TRANSFER).(map[string]*config.TransferConfig)
	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	mm := make(map[string]struct{})
	newMetrics := make([]string, 0)
	newErr := ""

	for _, resp := range fRes {
		if resp.HttpRes == nil {
			newErr += fmt.Sprintf(" [region:%s][err:requestError]", resp.RegionName)
			continue
		}

		if resp.HttpRes.StatusCode != http.StatusOK {
			logger.Error("[GetMetrics_http_error][region:%+v][rc:%+v]", resp.RegionName, resp.HttpRes.StatusCode)
			continue
		}
		respBytes, err := ioutil.ReadAll(resp.HttpRes.Body)
		if err != nil {
			logger.Error(err.Error())
		}
		defer resp.HttpRes.Body.Close()
		resV := MetricsResp{}
		err = json.Unmarshal(respBytes, &resV)
		if err != nil {
			logger.Error(err)
			continue
		}
		if resV.Err != "" {
			newErr += fmt.Sprintf(" [region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		for _, ms := range resV.Dat.Metrics {
			if _, loaded := mm[ms]; !loaded {
				newMetrics = append(newMetrics, ms)
				mm[ms] = struct{}{}
			}

		}

	}
	resp := dataobj.MetricResp{}

	resp.Metrics = newMetrics

	renderData(c, &resp, newErr)
}

func GetTagPairs(c *gin.Context) {
	recv := dataobj.EndpointMetricRecv{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	newErr := ""
	transM := c.MustGet(src.CONFIG_TRANSFER).(map[string]*config.TransferConfig)
	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	newDat := make([]dataobj.IndexTagkvResp, 0)
	metricTagM := make(map[string]dataobj.IndexTagkvResp)

	for _, resp := range fRes {
		if resp.HttpRes == nil {
			newErr += fmt.Sprintf(" [region:%s][err:requestError]", resp.RegionName)
			continue
		}

		if resp.HttpRes.StatusCode != http.StatusOK {
			logger.Error("[GetMetrics_http_error][region:%+v][rc:%+v]", resp.RegionName, resp.HttpRes.StatusCode)
			continue
		}
		respBytes, err := ioutil.ReadAll(resp.HttpRes.Body)
		if err != nil {
			logger.Error(err.Error())
		}
		defer resp.HttpRes.Body.Close()
		resV := TagkvResp{}
		err = json.Unmarshal(respBytes, &resV)
		if err != nil {
			logger.Error(err)
			continue
		}
		if resV.Err != "" {
			newErr += fmt.Sprintf(" [region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		for _, mm := range resV.Dat {
			if _, loaded := metricTagM[mm.Metric]; !loaded {
				metricTagM[mm.Metric] = mm
				newDat = append(newDat, mm)
			}

		}

	}
	renderData(c, newDat, newErr)
}

func GetIndexByFullTags(c *gin.Context) {
	recv := make([]dataobj.IndexByFullTagsRecv, 0)
	errors.Dangerous(c.ShouldBindJSON(&recv))

	newErr := ""
	transM := c.MustGet(src.CONFIG_TRANSFER).(map[string]*config.TransferConfig)
	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	newDat := make([]dataobj.IndexByFullTagsResp, 0)
	metricTagM := make(map[string]dataobj.IndexByFullTagsResp)
	count := 0
	for _, resp := range fRes {
		if resp.HttpRes == nil {
			newErr += fmt.Sprintf(" [region:%s][err:requestError]", resp.RegionName)
			continue
		}

		if resp.HttpRes.StatusCode != http.StatusOK {
			logger.Error("[GetMetrics_http_error][region:%+v][rc:%+v]", resp.RegionName, resp.HttpRes.StatusCode)
			continue
		}
		respBytes, err := ioutil.ReadAll(resp.HttpRes.Body)
		if err != nil {
			logger.Error(err.Error())
		}
		defer resp.HttpRes.Body.Close()
		resV := FullMatchResp{}
		err = json.Unmarshal(respBytes, &resV)
		if err != nil {
			logger.Error(err)
			continue
		}
		if resV.Err != "" {
			newErr += fmt.Sprintf(" [region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		for _, mm := range resV.Dat.List {
			if _, loaded := metricTagM[mm.Metric]; !loaded {
				metricTagM[mm.Metric] = mm
				newDat = append(newDat, mm)
				count += resV.Dat.Count
			}

		}

	}

	renderData(c, listResp{
		List:  newDat,
		Count: count,
	}, newErr)

}

func QueryDataForUI(c *gin.Context) {
	recv := dataobj.QueryDataForUI{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	newErr := ""
	transM := c.MustGet(src.CONFIG_TRANSFER).(map[string]*config.TransferConfig)
	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	newDat := make([]dataobj.QueryDataForUIResp, 0)
	metricTagM := make(map[string]dataobj.QueryDataForUIResp)

	for _, resp := range fRes {
		if resp.HttpRes == nil {
			newErr += fmt.Sprintf(" [region:%s][err:requestError]", resp.RegionName)
			continue
		}

		if resp.HttpRes.StatusCode != http.StatusOK {
			logger.Error("[GetMetrics_http_error][region:%+v][rc:%+v]", resp.RegionName, resp.HttpRes.StatusCode)
			continue
		}
		respBytes, err := ioutil.ReadAll(resp.HttpRes.Body)
		if err != nil {
			logger.Error(err.Error())
		}
		defer resp.HttpRes.Body.Close()
		resV := DataForUIResp{}
		err = json.Unmarshal(respBytes, &resV)
		if err != nil {
			logger.Error(err)
			continue
		}
		if resV.Err != "" {
			newErr += fmt.Sprintf(" [region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		for _, mm := range resV.Dat {
			if _, loaded := metricTagM[mm.Endpoint+mm.Counter]; !loaded {
				metricTagM[mm.Endpoint+mm.Counter] = mm
				newDat = append(newDat, mm)
			}

		}
	}
	renderData(c, newDat, newErr)
}

type listResp struct {
	List  interface{} `json:"list"`
	Count int         `json:"count"`
}

type RegionHttpRes struct {
	RegionName string
	HttpRes    *http.Response
}

func ConcurrencyReq(addrMap map[string]*config.TransferConfig, reqData interface{}, header http.Header, reqPath string) (fRes []*RegionHttpRes) {
	allRes := make(chan *RegionHttpRes, len(addrMap))
	wg := sync.WaitGroup{}
	for region, t := range addrMap {
		wg.Add(1)
		go func(t *config.TransferConfig, wg *sync.WaitGroup) {

			resI := BuildNewHttpPostReq(t, reqData, header, reqPath)
			allRes <- &RegionHttpRes{
				RegionName: region,
				HttpRes:    resI,
			}

			wg.Done()
		}(t, &wg)

	}

	wg.Wait()

	for i := 0; i < cap(allRes); i++ {
		res := <-allRes
		fRes = append(fRes, res)

	}

	return

}

func BuildNewHttpPostReq(t *config.TransferConfig, data interface{}, header http.Header, reqPath string) *http.Response {
	bytesData, err := json.Marshal(data)
	newAddr := fmt.Sprintf("%s%s", t.ApiAddr, reqPath)
	if err != nil {
		logger.Errorf("[reigon:%s][url:%s][BuildNewHttpPostReq.json.Marshal_dataError:%+v]", t.RegionName, newAddr, err)
		return nil
	}
	reader := bytes.NewReader(bytesData)
	request, err := http.NewRequest("POST", newAddr, reader)
	// http短连接
	//request.Close = true
	if err != nil {
		logger.Errorf("[reigon:%s][url:%s][BuildNewHttpPostReq_ERROR<1>:%+v]", t.RegionName, newAddr, err)
		return nil
	}
	request.Header = header
	client := http.Client{}
	client.Timeout = time.Second * time.Duration(t.TimeOutSeconds)
	resp, err := client.Do(request)
	if err != nil {
		logger.Errorf("[reigon:%s][url:%s][BuildNewHttpPostReq_ERROR<2>:%+v]", t.RegionName, newAddr, err)
		return nil
	}
	return resp

}
