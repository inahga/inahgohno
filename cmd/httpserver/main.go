package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	fmt.Printf("serving %s on :8080\n", dir)
	http.ListenAndServe(":8080", http.FileServer(http.Dir(dir)))
}
