package main

// #cgo LDFLAGS: -lpthread
// #include <pthread.h>
//
// int create_threads();
import "C"
import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var counter uint64

//export gocallback
func gocallback() {
	atomic.AddUint64(&counter, 1)
}

func main() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := C.create_threads(); err != 0 {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			go func() {
				for i := 0; i < 5; i++ {
					gocallback()
					<-time.After(time.Second * 1)
				}
			}()
		}
	}()
	wg.Wait()
	fmt.Println(counter)
}
