package calc

import (
	"github.com/didi/nightingale/src/common/dataobj"
	"math"
	"sort"
)

type AggrTsValue struct {
	Value dataobj.JsonFloat
	Count int
}

func AggregateResp(data []*dataobj.QueryDataForUIResp, opts dataobj.QueryDataForUI) []*dataobj.QueryDataForUIResp {
	// aggregateResp
	if len(data) < 2 || opts.AggrFunc == "" {
		return data
	}

	if len(opts.Comparisons) == 0 {
		return []*dataobj.QueryDataForUIResp{&dataobj.QueryDataForUIResp{
			Start:   opts.Start,
			End:     opts.End,
			Counter: opts.AggrFunc,
			Values:  Compute(opts.AggrFunc, data),
		}}
	}
	aggrDatas := make([]*dataobj.QueryDataForUIResp, 0)
	aggrCounter := make(map[int64][]*dataobj.QueryDataForUIResp)
	for _, v := range data {
		if _, loaded := aggrCounter[v.Comparison]; !loaded {
			aggrCounter[v.Comparison] = []*dataobj.QueryDataForUIResp{v}
		} else {
			aggrCounter[v.Comparison] = append(aggrCounter[v.Comparison], v)
		}
	}
	for k, v := range aggrCounter {
		aggrData := &dataobj.QueryDataForUIResp{
			Start:      opts.Start,
			End:        opts.End,
			Counter:    opts.AggrFunc,
			Values:     Compute(opts.AggrFunc, v),
			Comparison: k,
		}
		aggrDatas = append(aggrDatas, aggrData)
	}

	return aggrDatas

}

func Compute(f string, datas []*dataobj.QueryDataForUIResp) []*dataobj.RRDData {
	datasLen := len(datas)
	if datasLen < 1 {
		return nil
	}

	dataMap := make(map[int64]*AggrTsValue)
	switch f {
	case "sum":
		dataMap = sum(datas)
	case "avg":
		dataMap = avg(datas)
	case "max":
		dataMap = max(datas)
	case "min":
		dataMap = min(datas)
	default:
		return nil
	}

	var tmpValues dataobj.RRDValues
	for ts, v := range dataMap {
		d := &dataobj.RRDData{
			Timestamp: ts,
			Value:     v.Value,
		}
		tmpValues = append(tmpValues, d)
	}
	sort.Sort(tmpValues)
	return tmpValues
}

func sum(datas []*dataobj.QueryDataForUIResp) map[int64]*AggrTsValue {
	dataMap := make(map[int64]*AggrTsValue)
	datasLen := len(datas)
	for i := 0; i < datasLen; i++ {
		for j := 0; j < len(datas[i].Values); j++ {
			value := datas[i].Values[j].Value
			if math.IsNaN(float64(value)) {
				continue
			}
			if _, exists := dataMap[datas[i].Values[j].Timestamp]; exists {
				dataMap[datas[i].Values[j].Timestamp].Value += value
			} else {
				dataMap[datas[i].Values[j].Timestamp] = &AggrTsValue{Value: value}
			}
		}
	}
	return dataMap
}

func avg(datas []*dataobj.QueryDataForUIResp) map[int64]*AggrTsValue {
	dataMap := make(map[int64]*AggrTsValue)
	datasLen := len(datas)
	for i := 0; i < datasLen; i++ {
		for j := 0; j < len(datas[i].Values); j++ {
			value := datas[i].Values[j].Value
			if math.IsNaN(float64(value)) {
				continue
			}

			if _, exists := dataMap[datas[i].Values[j].Timestamp]; exists {
				dataMap[datas[i].Values[j].Timestamp].Count += 1
				dataMap[datas[i].Values[j].Timestamp].Value += (datas[i].Values[j].Value - dataMap[datas[i].Values[j].Timestamp].Value) /
					dataobj.JsonFloat(dataMap[datas[i].Values[j].Timestamp].Count)
			} else {
				dataMap[datas[i].Values[j].Timestamp] = &AggrTsValue{Value: value, Count: 1}
			}
		}
	}
	return dataMap
}

func minOrMax(datas []*dataobj.QueryDataForUIResp, fn func(a, b dataobj.JsonFloat) bool) map[int64]*AggrTsValue {
	dataMap := make(map[int64]*AggrTsValue)
	datasLen := len(datas)
	for i := 0; i < datasLen; i++ {
		for j := 0; j < len(datas[i].Values); j++ {
			value := datas[i].Values[j].Value
			if math.IsNaN(float64(value)) {
				continue
			}

			if _, exists := dataMap[datas[i].Values[j].Timestamp]; exists {
				if fn(value, dataMap[datas[i].Values[j].Timestamp].Value) {
					dataMap[datas[i].Values[j].Timestamp].Value = value
				}
			} else {
				dataMap[datas[i].Values[j].Timestamp] = &AggrTsValue{Value: value}
			}
		}
	}
	return dataMap
}

func max(datas []*dataobj.QueryDataForUIResp) map[int64]*AggrTsValue {
	return minOrMax(datas, func(a, b dataobj.JsonFloat) bool { return a > b })
}

func min(datas []*dataobj.QueryDataForUIResp) map[int64]*AggrTsValue {
	return minOrMax(datas, func(a, b dataobj.JsonFloat) bool { return a < b })
}
