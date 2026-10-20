package services

import "time"

// timeNowMillis is the current time in milliseconds.
func timeNowMillis() int64 { return time.Now().UnixMilli() }
