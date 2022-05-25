package main

// #cgo LDFLAGS: -lpthread
// #include <pthread.h>
//
// int create_thread(pthread_t *);
import "C"
import "fmt"

//export gocallback
func gocallback() {
	fmt.Println("gocallback()")
}

func main() {
	var thread C.pthread_t
	if err := C.create_thread(&thread); err != 0 {
		panic(err)
	}
	fmt.Println(thread)
	<-make(chan struct{})
}
