package logs

import (
	"fmt"
	"github.com/toolkits/pkg/logger"
)

type Config struct {
	Dir       string `yaml:"dir"`
	Level     string `yaml:"level"`
	KeepHours uint   `yaml:"keepHours"`
}

// InitLogger init logger toolkit
func Init(c Config) error {
	lb, err := logger.NewFileBackend(c.Dir)
	if err != nil {
		fmt.Println("cannot init logger:", err)
		return err
	}

	lb.SetRotateByHour(true)
	lb.SetKeepHours(c.KeepHours)
	logger.SetLogging(c.Level, lb)
	return nil
}
