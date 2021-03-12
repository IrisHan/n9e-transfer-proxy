module n9e-transfer-proxy

go 1.15

require (
	github.com/didi/nightingale v1.4.1-0.20210310095038-3e426537c7bc
	github.com/gin-gonic/gin v1.6.3
	github.com/go-kit/kit v0.10.0
	github.com/oklog/run v1.1.0
	github.com/prometheus/common v0.18.0
	github.com/spf13/viper v1.7.1
	github.com/toolkits/pkg v1.1.3
	google.golang.org/grpc/examples v0.0.0-20210311051244-e8930beb0e04 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/m3db/m3 => github.com/m3db/m3 v1.1.0
