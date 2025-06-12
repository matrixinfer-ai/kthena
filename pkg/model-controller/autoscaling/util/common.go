package util

import "time"

func GetCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

func SecondToTimestamp(sec int64) int64 {
	return sec * 1000
}
