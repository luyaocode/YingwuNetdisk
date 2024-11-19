package utils

import (
	"errors"
	"strconv"
	"yingwu/config"
)

// 封装函数：将 any 转为 int
func AnyToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case string:
		intValue, err := strconv.Atoi(v)
		if err != nil {
			if v == "test" {
				return config.Role_Test, err
			} else if v == "guest" {
				return config.Role_Guest, err
			}
			return config.Role_Other, err
		}
		return int64(intValue), nil
	default:
		return 0, errors.New("unsupported type: value must be string or int")
	}
}
