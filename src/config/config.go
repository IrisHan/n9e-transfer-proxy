package config

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"github.com/toolkits/pkg/file"
)

type Config struct {
	TransferConfigC []*TransferConfig `mapstructure:"transfer"`
	HttpC           *HttpConfig       `mapstructure:"http"`
}

type TransferConfig struct {
	RegionName     string `mapstructure:"region_name"`
	ApiAddr        string `mapstructure:"api_addr"`
	TimeOutSeconds uint64 `mapstructure:"time_out_second"`
}

type HttpConfig struct {
	HttpListenPort uint64 `mapstructure:"http_listen_port"`
}

func LoadFile(conf string) (*Config, error) {
	bs, err := file.ReadBytes(conf)
	if err != nil {
		return nil, fmt.Errorf("cannot read yml[%s]: %v", conf, err)
	}
	viper.SetConfigType("yaml")
	err = viper.ReadConfig(bytes.NewBuffer(bs))
	if err != nil {
		return nil, fmt.Errorf("cannot read yml[%s]: %v", conf, err)
	}

	viper.SetDefault("http", map[string]interface{}{
		"http_listen_port": 8032,
	})

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config error:%v", err)
	}
	return &cfg, nil

}
