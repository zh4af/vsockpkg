package utils

import (
	"fmt"
	"time"
)

func Retry(retryCnt int, interval time.Duration, module string, handle func() (bool, interface{}, error)) (interface{}, error) {
	var (
		success bool
		ret     interface{}
		err     error
	)
	for i := 0; i < retryCnt; i++ {
		success, ret, err = handle()
		if !success || err != nil {
			time.Sleep(interval)
			continue
		}
		return ret, nil
	}
	return nil, fmt.Errorf("retry %s too many times, err:%v", module, err)
}
