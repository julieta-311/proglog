package main

import (
	"fmt"
	"os"

	"github.com/julieta-311/proglog/internal/server"
)

func main() {
	srv := server.NewHTTPServer("0.0.0.0:8080")
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}
