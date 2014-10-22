package workers

import (
	"fmt"
	"time"
)

func Main(redisURL string) {
	for {
		fmt.Printf("zzz........\n")
		time.Sleep(10000 * time.Millisecond)
	}
}
