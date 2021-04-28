# 功能说明
- 该服务用于实现nightingales生产环境多机房架构，将nightingale在每个机房独立部署一套，然后部署该服务+修改nginx路由规则，对以下几个接口路由至本服务，nigthingale前端即可通过该服务实现多机房指标聚合。这里指的多套nightingale中，mysql必须共用一套，m3db和所有服务都各机房搭建即可。
- 该服务对接的nightingales 版本为`v 3.7.0`

# 产品效果
![image](https://github.com/IrisHan/n9e-transfer-proxy/blob/main/images/1.jpg)
![image](https://github.com/IrisHan/n9e-transfer-proxy/blob/main/images/2.jpg)
![image](https://github.com/IrisHan/n9e-transfer-proxy/blob/main/images/3.jpg)

# 架构说明
![image](https://github.com/ning1875/n9e-transfer-proxy/blob/main/images/n9e-transfer-proxy-arch.png)


- 分析前端请求发现在使用`m3db`作为后端时需要下面`5`个接口
```golang
/*
下面对应都是transfer
- Request URL: http://127.0.0.1:8032/api/index/metrics 对应 QueryMetrics(recv dataobj.EndpointsRecv) *dataobj.MetricResp
- Request URL: http://127.0.0.1:8032/api/index/tagkv     对应 QueryTagPairs(recv dataobj.EndpointMetricRecv) []dataobj.IndexTagkvResp
- Request URL: http://127.0.0.1:8032/api/index/counter/fullmatch  对应 QueryIndexByFullTags(recv []dataobj.IndexByFullTagsRecv) ([]dataobj.IndexByFullTagsResp, int)
- Request URL: http://127.0.0.1:8032/api/transfer/data/ui  对应 QueryDataForUI(input dataobj.QueryDataForUI) []*dataobj.TsdbQueryResponse
- Request URL: http://127.0.0.1:8032/api/monapi/index/tagkv 
*/
```
- 所以`proxy`只需要实现上述5个接口的代理即可
- 将原始请求并发打向后端所有`transfer`接口
- 将数据`merge`后在返回前端
- 在使用`m3db`作为存储时，有`布隆过滤器`顶在最前端，所以查询不存在该机房的数据所引发的资源开销不大
- 优化点可以在`proxy` hold一个map，将请求的endpoint精细打向执行的机房
- map的来源可以是长链接server端的数据

# 使用说明
> 编译 or 下载二进制
```shell script
# 编译
export GOPROXY=https://goproxy.io,direct 
export GO111MODULE=on
go build src/main.go -o n9e-transfer-proxy
# 下载二进制
wget https://github.com/ning1875/n9e-transfer-proxy/releases/download/v1.0/n9e-transfer-proxy-1.0.linux-amd64.tar.gz
```
> 修改配置文件
- 将所有分区`transfer`地址填入
> 启动proxy服务
```shell script
n9e-transfer-proxy --config.file="n9e-transfer-proxy.yml"
```
> 修改n9e前端指向的nginx.conf
- 将使用`m3db`作为后端时的/api/index  /api/transfer指到`proxy`即可
```shell script
upstream n9e.proxy {
    server proxy的地址如 localhost:9032;
    keepalive 60;
}

location /api/index {
    proxy_pass http://n9e.proxy;
}

location /api/transfer {
    proxy_pass http://n9e.proxy;
}

```
> transfer的路由中还有些path没实现，但是看起来和前端请求无关，可以自行实现下
> 或者在修改nginx配置时精细化 比如只配置以下4条和前端相关的指向proxy，其余的还是指向transfer
```shell script
/api/index/metrics          
/api/index/tagkv            
/api/index/counter/fullmatch
/api/transfer/data/ui    

upstream n9e.proxy {
    server proxy的地址如 localhost:9032;
    keepalive 60;
}

location /api/index/metrics {
    proxy_pass http://n9e.proxy;
}
location /api/index/tagkv  {
    proxy_pass http://n9e.proxy;
}
location /api/index/counter/fullmatch  {
    proxy_pass http://n9e.proxy;
}


location /api/transfer {
    proxy_pass http://n9e.proxy;
}

```
- transfer v3的路由
```golang

func Config(r *gin.Engine) {
	sys := r.Group("/api/transfer")
	{
		sys.GET("/ping", ping)
		sys.GET("/pid", pid)
		sys.GET("/addr", addr)
		sys.POST("/stra", getStra)
		sys.POST("/which-tsdb", tsdbInstance)
		sys.POST("/which-judge", judgeInstance)
		sys.GET("/alive-judges", judges)

		sys.POST("/push", PushData)
		sys.POST("/data", QueryData)
		sys.POST("/data/ui", QueryDataForUI)
	}

	index := r.Group("/api/index")
	{
		index.POST("/metrics", GetMetrics)
		index.POST("/tagkv", GetTagPairs)
		index.POST("/counter/clude", GetIndexByClude)
		index.POST("/counter/fullmatch", GetIndexByFullTags)
	}

	pprof.Register(r, "/api/transfer/debug/pprof")
}
```
