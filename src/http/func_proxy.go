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
	"n9e-transfer-proxy/src/http/calc"
	"net/http"
	"strings"
	"sync"
	"time"
)

//type N9EResp struct {
//	Dat struct {
//		List []struct {
//			ID          int       `json:"id"`
//			UUID        string    `json:"uuid"`
//			Ident       string    `json:"ident"`
//			Name        string    `json:"name"`
//			Labels      string    `json:"labels"`
//			Note        string    `json:"note"`
//			Extend      string    `json:"extend"`
//			Cate        string    `json:"cate"`
//			Tenant      string    `json:"tenant"`
//			LastUpdated time.Time `json:"last_updated"`
//		} `json:"list"`
//		Total int `json:"total"`
//	} `json:"dat"`
//	Err string `json:"err"`
//}

type MetricsResp struct {
	Dat dataobj.MetricResp `json:"dat"`
	Err string             `json:"err"`
}

type TagkvResp struct {
	Dat []dataobj.IndexTagkvResp `json:"dat"`
	Err string                   `json:"err"`
}

type IndexTagkvRespM struct {
	Endpoints map[string]bool             `json:"endpoints"`
	Nids      map[string]bool             `json:"nids"`
	Tagkv     map[string]*dataobj.TagPair `json:"tagkv"`
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

type NidMetricRecv struct {
	Nids    []string `json:"nids"`
	Metrics []string `json:"metrics"`
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

func makeTransMap() map[string]*config.TransferConfig {
	m := make(map[string]*config.TransferConfig)
	for _, t := range config.Conf().TransferConfigC {
		m[t.RegionName] = t
	}
	return m
}

func GetMetrics(c *gin.Context) {

	recv := dataobj.EndpointsRecv{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	transM := makeTransMap()
	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	mm := make(map[string]struct{})
	newMetrics := make([]string, 0)
	newErr := ""

	for _, resp := range fRes {
		resV := MetricsResp{}
		if CheckResp(resp, &resV, "GetMetrics") {
			newErr += fmt.Sprintf("[GetMetrics_checkResp][region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		if resV.Err != "" || len(resV.Dat.Metrics) == 0 {
			logger.Debugf("[GetMetrics_respbody][region:%s][no data]", resp.RegionName)
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

//设备无关型数据，需要先遍历子节点再返回
func GetTagkvs(c *gin.Context) {
	//recv := NidMetricRecv{}
	recv := dataobj.EndpointMetricRecv{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	newErr := ""
	transM := makeTransMap()
	reqPath := c.Request.URL.Path

	//进行合并去重
	newDat := make([]dataobj.IndexTagkvResp, 0)

	//first pair
	fistPair := make([]*dataobj.TagPair, 0)
	var get bool
	var gotRegion string

	for k, v := range transM {
		//只请求一次monapi获得子节点列表即可
		tmpregion := map[string]*config.TransferConfig{k: v}
		fRes := ConcurrencyReq(tmpregion, recv, c.Request.Header, reqPath)
		for _, resp := range fRes {
			resV := TagkvResp{}
			if CheckResp(resp, &resV, "GetTagkvs") {
				newErr += fmt.Sprintf("[GetTagkvs_CheckResp][region:%s][err:%s]", resp.RegionName, resV.Err)
				delete(transM, resp.RegionName)
				continue
			}

			if resV.Err != "" || resV.Dat == nil {
				logger.Debugf("[GetTagkvs_respbody][region:%s][no data]", resp.RegionName)
				delete(transM, resp.RegionName)
				continue
			}

			for _, mm := range resV.Dat {
				if len(mm.Nids) == 0 {
					logger.Debugf("[GetTagkvs][region:%s][no nids;%#v]", resp.RegionName, mm)
					delete(transM, resp.RegionName)
					continue
				}
				logger.Debugf("[GetTagkvs_respbody][region:%s][get first data success:%#v]", resp.RegionName, mm)

				//第一个请求已经确定正确，确定所有子nids
				get = true
				recv.Nids = mm.Nids
				//保留一下第一个获得的tag
				fistPair = mm.Tagkv
				gotRegion = resp.RegionName
				delete(transM, resp.RegionName)
				goto OUT
			}
		}

	}

	OUT:
	if !get {
		logger.Errorf("[GetTagkvs][cannot find any tags，resq:%#v]", recv)
		renderData(c, newDat, newErr)
		return
	}

	if len(transM) == 0 {
		regionTag := &dataobj.TagPair{
			Key:    "region",
			Values: []string{gotRegion},
		}
		fistPair = append(fistPair, regionTag)

		result := dataobj.IndexTagkvResp{
			Nids: recv.Nids,
			Metric: recv.Metrics[0],
			Tagkv: fistPair,
		}
		logger.Debugf("[GetTagkvs][only one region have tags][result:%#v]", result)
		newDat = append(newDat, result)
		renderData(c, newDat, newErr)
		return
	}

	newDat, newErr = ProdcuceTagPairs(c.Request.Header, transM, recv, fistPair, gotRegion)
	renderData(c, newDat, newErr)
	return
}

func GetTagPairs(c *gin.Context) {
	recv := dataobj.EndpointMetricRecv{}
	errors.Dangerous(c.ShouldBindJSON(&recv))

	transM := makeTransMap()
	newDat, newErr := ProdcuceTagPairs(c.Request.Header, transM, recv, nil, "")
	renderData(c, newDat, newErr)
	return
}

func GetIndexByFullTags(c *gin.Context) {
	recv := make([]dataobj.IndexByFullTagsRecv, 0)
	errors.Dangerous(c.ShouldBindJSON(&recv))

	newErr := ""
	transM := makeTransMap()
	logger.Debugf("[GetIndexByFullTags_check_resq][transM:%#v]", transM)

	// 设置一个region区间，如果用户选择了按照机房筛选，则将这个带回给response供ui接口使用
	regionString := ""
	// 根据region进行请求筛选，并且去掉region tag对各集群的影响
	if len(recv[0].Tagkv) != 0 {
		logger.Debugf("[GetIndexByFullTags] check region struct:%+v", recv[0].Tagkv)

		for i := 0; i < len(recv[0].Tagkv); i++ {
			if recv[0].Tagkv[i].Key == "region" {
				//请求时只请求region机房
				if len(recv[0].Tagkv[i].Values) != 0 && len(recv[0].Tagkv[i].Values) != len(transM) {
					regionsM := map[string]bool{}
					for _, t := range recv[0].Tagkv[i].Values {
						if _, ok := regionsM[t]; !ok {
							regionsM[t] = true
							regionString = regionString + "," + t
						}
					}
					for k, _ := range transM {
						if _, ok := regionsM[k]; !ok {
							logger.Debugf("[GetIndexByFullTags] delete a request in region:%s", k)
							delete(transM, k)
						}
					}
					if len(transM) == 0 {
						logger.Errorf("[GetIndexByFullTags_http_error] cannot find region to resq, trigger all:%+v", recv[0].Tagkv)
						transM = c.MustGet(src.CONFIG_TRANSFER).(map[string]*config.TransferConfig)
					}

				}
				//清理tagKv的region
				logger.Debugf("[GetIndexByFullTags] check resq [tagkv:%+v][regionString:%s]", recv[0].Tagkv, regionString)
				if len(recv[0].Tagkv) == 1 {
					recv[0].Tagkv = nil
				} else {
					recv[0].Tagkv = append(recv[0].Tagkv[:i], recv[0].Tagkv[i+1:]...)
				}
				break
			}
		}
	}

	reqPath := c.Request.URL.Path
	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	newDat := make([]dataobj.IndexByFullTagsResp, 0)
	metricTagM := make(map[string]*dataobj.IndexByFullTagsResp)
	TagsM := map[string]map[string]bool{}
	count := 0
	for _, resp := range fRes {
		resV := FullMatchResp{}
		if CheckResp(resp, &resV, "GetIndexByFullTags") {
			newErr += fmt.Sprintf("[GetIndexByFullTags_checkResp][region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		if resV.Err != "" {
			logger.Debugf("[GetIndexByFullTags_respbody][region:%s][error:%s]", resp.RegionName, resV.Err)
			newErr += fmt.Sprintf("[region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}

		for i := 0; i < len(resV.Dat.List); i++ {
			if _, loaded := metricTagM[resV.Dat.List[i].Metric]; !loaded {
				metricTagM[resV.Dat.List[i].Metric] = &resV.Dat.List[i]

				//make my tags
				mytags := make(map[string]bool)
				if len(resV.Dat.List[i].Tags) != 0 {
					for _, v := range resV.Dat.List[i].Tags {
						mytags[v] = true
					}
				}
				TagsM[resV.Dat.List[i].Metric] = mytags

				continue
			}
			//现在针对已经存在的，需要将曲线合二为一
			metricTagM[resV.Dat.List[i].Metric].Count = resV.Dat.List[i].Count + metricTagM[resV.Dat.List[i].Metric].Count
			if len(resV.Dat.List[i].Tags) != 0 {
				for _, v := range resV.Dat.List[i].Tags {
					TagsM[resV.Dat.List[i].Metric][v] = true
				}
			}

		}
		count += resV.Dat.Count

	}
	if len(metricTagM) > 1 {
		logger.Errorf("[GetIndexByFullTags_response_error]get more than one metrics! bug! lets see :%#v", metricTagM)
	}

	for _, mm := range metricTagM {
		tags := []string{}
		if _, ok := TagsM[mm.Metric]; ok {
			for t, _ := range TagsM[mm.Metric] {
				tags = append(tags, t)
			}
		}
		//将组合好的regions连带返回
		if regionString != "" {
			regionString = "region=" + strings.TrimPrefix(regionString, ",")
			logger.Debugf("[GetIndexByFullTags] region struct is:%s", regionString)

			tags = append(tags, regionString)
		}

		mm.Tags = tags
		newDat = append(newDat, *mm)
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
	transM := makeTransMap()

	// 根据region进行请求筛选，并且去掉region tag对各集群的影响
	if len(recv.Tags) != 0 {
		for i := 0; i < len(recv.Tags); i++ {
			if strings.HasPrefix(recv.Tags[i], "region=") {
				regionsM := map[string]bool{}
				//请求时只请求region机房
				tmp := strings.Split(strings.TrimPrefix(recv.Tags[i], "region="), ",")
				if len(tmp) != 0 && len(tmp) != len(transM) {
					for _, t := range tmp {
						if _, ok := regionsM[t]; !ok {
							regionsM[t] = true
						}
					}
					for k, _ := range transM {
						if _, ok := regionsM[k]; !ok {
							delete(transM, k)
						}
					}
					if len(transM) == 0 {
						logger.Errorf("[QueryDataForUI_http_error] cannot find region to resq, trigger all:%+v", recv.Tags)
						transM = makeTransMap()
					}

				}

				//清理tag的region
				logger.Debugf("[QueryDataForUI] check resq tagkv:%+v", recv.Tags)
				if len(recv.Tags) == 1 {
					recv.Tags = []string{}
				} else {
					recv.Tags = append(recv.Tags[:i], recv.Tags[i+1:]...)
				}

				break
			}
		}
	}

	var aggrByRegion bool
	if len(recv.GroupKey) != 0 {
		for i := 0; i < len(recv.GroupKey); i++ {
			//按照region进行聚合的建议存在时，去掉请求各机房的影响
			if recv.GroupKey[i] == "region" {
				if len(recv.GroupKey) == 1 {
					recv.GroupKey = []string{}
				} else {
					recv.GroupKey = append(recv.GroupKey[:i], recv.GroupKey[i+1:]...)
				}

				aggrByRegion = true
				break
			}
		}
	}

	reqPath := c.Request.URL.Path
	newDat := make([]*dataobj.QueryDataForUIResp, 0)
	metricTagM := make(map[string]*dataobj.QueryDataForUIResp)

	fRes := ConcurrencyReq(transM, recv, c.Request.Header, reqPath)
	for _, resp := range fRes {
		resV := DataForUIResp{}
		if CheckResp(resp, &resV, "QueryDataForUI") {
			newErr += fmt.Sprintf("[QueryDataForUI_checkResp][region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}
		if resV.Err != "" {
			logger.Debugf("[QueryDataForUI_respbody][region:%s][error:%s]", resp.RegionName, resV.Err)
			newErr += fmt.Sprintf("[region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}

		if resV.Dat == nil {
			continue
		}

		//如果是非聚合请求则如下OK
		if recv.AggrFunc == "" {
			if len(recv.Comparisons) > 1 {
				if len(recv.Endpoints) != 0 {
					for i := 0; i < len(resV.Dat); i++ {
						transK := resV.Dat[i].Endpoint + resV.Dat[i].Counter + fmt.Sprintf("%d", resV.Dat[i].Comparison)
						if _, loaded := metricTagM[transK]; !loaded {
							logger.Debugf("[QueryDataForUI][machine metric][region:%s][key:%s][info:%+v]", resp.RegionName, transK, resV.Dat[i])
							metricTagM[transK] = &resV.Dat[i]
							newDat = append(newDat, &resV.Dat[i])
						}
					}
					continue
				}
				if len(recv.Nids) != 0 {
					for i := 0; i < len(resV.Dat); i++ {
						transK := resV.Dat[i].Nid + resV.Dat[i].Counter + fmt.Sprintf("%d", resV.Dat[i].Comparison)
						if _, loaded := metricTagM[transK]; !loaded {
							logger.Debugf("[QueryDataForUI][nomachine metric][region:%s][key:%s][info:%+v]", resp.RegionName, transK, resV.Dat[i])
							metricTagM[transK] = &resV.Dat[i]
							newDat = append(newDat, &resV.Dat[i])
						}
					}
					continue
				}
				continue

			} else {
				if len(recv.Endpoints) != 0 {
					for i := 0; i < len(resV.Dat); i++ {
						transK := resV.Dat[i].Endpoint + resV.Dat[i].Counter
						if _, loaded := metricTagM[transK]; !loaded {
							logger.Debugf("[QueryDataForUI][machine metric][region:%s][key:%s][info:%+v]", resp.RegionName, transK, resV.Dat[i])
							metricTagM[transK] = &resV.Dat[i]
							newDat = append(newDat, &resV.Dat[i])
						}
					}
					continue
				}
				if len(recv.Nids) != 0 {
					for i := 0; i < len(resV.Dat); i++ {
						transK := resV.Dat[i].Nid + resV.Dat[i].Counter
						if _, loaded := metricTagM[transK]; !loaded {
							logger.Debugf("[QueryDataForUI][nomachine metric][region:%s][key:%s][info:%+v]", resp.RegionName, transK, resV.Dat[i])
							metricTagM[transK] = &resV.Dat[i]
							newDat = append(newDat, &resV.Dat[i])
						}
					}
					continue
				}
				continue
			}

		}

		//如果是有聚合需求，则需要区分一下进行六机房整体聚合，还是独立聚合, 查看是否有独立聚合的tag
		for i := 0; i < len(resV.Dat); i++ {
			//前端界面需要对各机房聚合的曲线明确标识是哪个机房
			resV.Dat[i].Counter = resV.Dat[i].Counter + ",region=" + resp.RegionName
			//针对如果是设备相关请求，就按照设备处理
			if len(recv.Comparisons) > 1 {
				if len(recv.Endpoints) != 0 {
					transK := resV.Dat[i].Endpoint + resV.Dat[i].Counter + fmt.Sprintf("%d", resV.Dat[i].Comparison)
					if _, loaded := metricTagM[transK]; !loaded {
						metricTagM[transK] = &resV.Dat[i]
						newDat = append(newDat, &resV.Dat[i])
					}
					continue
				}
				//针对如果是设备无关请求，就按照设备处理
				if len(recv.Nids) != 0 {
					transK := resV.Dat[i].Nid + resV.Dat[i].Counter + fmt.Sprintf("%d", resV.Dat[i].Comparison)
					if _, loaded := metricTagM[transK]; !loaded {
						metricTagM[transK] = &resV.Dat[i]
						newDat = append(newDat, &resV.Dat[i])
					}
					continue
				}
				continue

			} else {
				if len(recv.Endpoints) != 0 {
					transK := resV.Dat[i].Endpoint + resV.Dat[i].Counter
					if _, loaded := metricTagM[transK]; !loaded {
						metricTagM[transK] = &resV.Dat[i]
						newDat = append(newDat, &resV.Dat[i])
					}
					continue
				}
				//针对如果是设备无关请求，就按照设备处理
				if len(recv.Nids) != 0 {
					transK := resV.Dat[i].Nid + resV.Dat[i].Counter
					if _, loaded := metricTagM[transK]; !loaded {
						metricTagM[transK] = &resV.Dat[i]
						newDat = append(newDat, &resV.Dat[i])
					}
					continue
				}
				continue

			}

		}
	}

	if aggrByRegion || recv.AggrFunc == "" {
		renderData(c, newDat, newErr)
		return
	}
	//如果用户没有选择根据机房聚合，则在机房的基础上进行二次聚合
	newDat = calc.AggregateResp(newDat, recv)
	renderData(c, newDat, newErr)
	return
}


func ProdcuceTagPairs(header http.Header, transM map[string]*config.TransferConfig, recv dataobj.EndpointMetricRecv, existPairs []*dataobj.TagPair, existRegion string) (newDat []dataobj.IndexTagkvResp, newErr string) {

	reqPath := config.Conf().Router.RootPrefix + "/index/tagkv"
	fRes := ConcurrencyReq(transM, recv, header, reqPath)

	//进行合并去重
	newDat = make([]dataobj.IndexTagkvResp, 0)
	itkm := make(map[string]*IndexTagkvRespM, 0)

	//新增一个区分机房的tag
	regionTag := &dataobj.TagPair{
		Key:    "region",
		Values: []string{},
	}

	if existRegion != "" {
		regionTag.Values = append(regionTag.Values, existRegion)
	}

	for _, resp := range fRes {
		resV := TagkvResp{}
		if CheckResp(resp, &resV, "ProdcuceTagPairs") {
			newErr += fmt.Sprintf("[ProdcuceTagPairs_CheckResp][region:%s][err:%s]", resp.RegionName, resV.Err)
			continue
		}

		if resV.Err != "" || resV.Dat == nil {
			logger.Debugf("[ProdcuceTagPairs_respbody][region:%s][no data]", resp.RegionName)
			continue
		}
		//对于来自多个机房的返回，需要先将endpoint合并，再nid合并，tagpairs中同属于一个key的vaues合并
		//这个data总是以数组形式展示，所以先range
		var haveData bool
		//组装region tags
		for _, mm := range resV.Dat {
			//itkm是去重map,相同指标的
			if len(mm.Endpoints) == 0 && len(mm.Nids) == 0 {
				continue
			}
			logger.Debugf("[ProdcuceTagPairs_respbody][region:%s][%#v]", resp.RegionName, mm)
			haveData = true

			if _, get := itkm[mm.Metric]; !get {
				itk := &IndexTagkvRespM{
					Endpoints: make(map[string]bool),
					Nids:      make(map[string]bool),
					Tagkv:     make(map[string]*dataobj.TagPair),
				}
				if len(mm.Endpoints) != 0 {
					for _, v := range mm.Endpoints {
						itk.Endpoints[v] = true
					}
				}

				if len(mm.Nids) != 0 {
					for _, t := range mm.Nids {
						itk.Nids[t] = true
					}
				}
				if mm.Tagkv != nil && len(mm.Tagkv) != 0 {
					for _, p := range mm.Tagkv {
						itk.Tagkv[p.Key] = p
					}
				}
				itkm[mm.Metric] = itk

			} else {
				//相同指标从其他机房merge
				if len(mm.Endpoints) != 0 {
					for _, v := range mm.Endpoints {
						itkm[mm.Metric].Endpoints[v] = true
					}
				}

				if len(mm.Nids) != 0 {
					for _, t := range mm.Nids {
						itkm[mm.Metric].Nids[t] = true
					}
				}

				if mm.Tagkv != nil && len(mm.Tagkv) != 0 {
					for _, p := range mm.Tagkv {
						//轮询所有pair
						if tpair, ok := itkm[mm.Metric].Tagkv[p.Key]; !ok {
							//没有这个pair，就直接添加
							itkm[mm.Metric].Tagkv[p.Key] = p
						} else {
							//merge 有这个pair，需要将values都添加进去，第一步合并
							tpair.Values = append(tpair.Values, p.Values...)

							//duplicate
							dup := map[string]bool{}
							for _, v1 := range tpair.Values {
								dup[v1] = true
							}
							values := []string{}
							for v2, _ := range dup {
								values = append(values, v2)
							}
							tpair.Values = values
							itkm[mm.Metric].Tagkv[p.Key] = tpair
						}
					}
				}
			}
		}
		if haveData {
			regionTag.Values = append(regionTag.Values, resp.RegionName)
		}
	}

	//对于设备无关型的处理，需要对第一次请求的数据进行
	if len(existPairs) != 0 && len(recv.Metrics) != 0 {
		if _, ok := itkm[recv.Metrics[0]]; ok {
			for i := 0; i < len(existPairs); i++ {
				if tpair, hit := itkm[recv.Metrics[0]].Tagkv[existPairs[i].Key]; !hit {
					itkm[recv.Metrics[0]].Tagkv[existPairs[i].Key] = existPairs[i]
				} else {
					//merge 有这个pair，需要将values都添加进去，第一步合并
					tpair.Values = append(tpair.Values, existPairs[i].Values...)

					//duplicate
					dup := map[string]bool{}
					for _, v1 := range tpair.Values {
						dup[v1] = true
					}
					values := []string{}
					for v2, _ := range dup {
						values = append(values, v2)
					}
					tpair.Values = values
					itkm[recv.Metrics[0]].Tagkv[existPairs[i].Key] = tpair
				}
			}
		} else {
			existPairsM := make(map[string]*dataobj.TagPair)
			existNids := make(map[string]bool)
			for _, k := range recv.Nids {
				existNids[k] = true
			}
			for _, v := range existPairs {
				existPairsM[v.Key] = v
			}
			fiststItk := &IndexTagkvRespM{
				Endpoints: make(map[string]bool),
				Nids:      existNids,
				Tagkv:     existPairsM,
			}
			itkm[recv.Metrics[0]] = fiststItk
		}

	}

	//将IndexTagkvRespM，转回IndexTagkvResp
	for k, v := range itkm {
		resd := dataobj.IndexTagkvResp{
			Endpoints: []string{},
		}
		for e, _ := range v.Endpoints {
			resd.Endpoints = append(resd.Endpoints, e)
		}
		if len(v.Nids) != 0 {
			resd.Nids = []string{}
			for n, _ := range v.Nids {
				resd.Nids = append(resd.Nids, n)
			}
		}
		resd.Metric = k
		if len(v.Tagkv) != 0 {
			resd.Tagkv = []*dataobj.TagPair{}
			for _, p := range v.Tagkv {
				resd.Tagkv = append(resd.Tagkv, p)
			}
		}
		resd.Tagkv = append(resd.Tagkv, regionTag)

		newDat = append(newDat, resd)
	}

	return
}

type listResp struct {
	List  interface{} `json:"list"`
	Count int         `json:"count"`
}

type RegionHttpRes struct {
	RegionName string
	HttpRes    *http.Response
}

func CheckResp(resp *RegionHttpRes, resV interface{}, tag string) bool {
	if resp.HttpRes == nil {
		logger.Errorf("%s[region:%s][err:requestError]", tag, resp.RegionName)
		return true
	}

	if resp.HttpRes.StatusCode != http.StatusOK {
		logger.Errorf("[%s_http_error][region:%s][rc:%+v]", tag, resp.RegionName, resp.HttpRes.StatusCode)
		return true
	}
	respBytes, err := ioutil.ReadAll(resp.HttpRes.Body)
	if err != nil {
		logger.Errorf("[%s_readbody_error][region:%s][error:%+v]", tag, resp.RegionName, err)
	}
	defer resp.HttpRes.Body.Close()
	logger.Debugf("[%s_check_resq][info:%#v]", tag, resp)

	err = json.Unmarshal(respBytes, resV)
	if err != nil {
		logger.Errorf("[%s_unmarshal_error][region:%s][source:%s][error:%+v]", tag, resp.RegionName, string(respBytes), err)
		return true
	}
	return false
}

func ConcurrencyReq(addrMap map[string]*config.TransferConfig, reqData interface{}, header http.Header, reqPath string) (fRes []*RegionHttpRes) {
	allRes := make(chan *RegionHttpRes, len(addrMap))
	reqPath = strings.Replace(reqPath, config.Conf().Router.SourcePrefix, config.Conf().Router.DstPrefix, 1)
	wg := sync.WaitGroup{}
	for region, t := range addrMap {
		wg.Add(1)
		go func(t *config.TransferConfig, region string, wg *sync.WaitGroup) {

			resI := BuildNewHttpPostReq(t, reqData, header, reqPath)

			logger.Debugf("[ConcurrencyReq][region:%s][resp:%#v]", region, resI)
			allRes <- &RegionHttpRes{
				RegionName: region,
				HttpRes:    resI,
			}

			wg.Done()
		}(t, region, &wg)

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
	logger.Infof("[reigon:%s][url:%s] trigger post!", t.RegionName, newAddr)
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
