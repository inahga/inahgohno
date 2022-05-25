package main

// #cgo LDFLAGS: -lpthread
// int run_thread();
import "C"
import "fmt"

//export gocallback
func gocallback() {
	fmt.Println("gocallback()")
}

func main() {
	if err := C.run_thread(); err != 0 {
		panic(err)
	}
	<-make(chan struct{})
}
