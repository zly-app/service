package mqtt_consume

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWorkers(t *testing.T) {
	workers := NewWorkers(10)
	workers.Start()

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			workers.Go(func() {
				time.Sleep(1e9)
				fmt.Println(i)
			})
			wg.Done()
		}(i)
	}
	wg.Wait() // 保证都写入了
	fmt.Println("等待结束")
	workers.Stop()
}
