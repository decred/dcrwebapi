package main

import (
	"log"
	"net/http"
)

const (
	// the listening port
	defaultPort = ":8080"
)

func main() {
	service := NewService()
	log.Println("dcrwebapi starting on", defaultPort)
	log.Fatal(http.ListenAndServe(defaultPort, service.Router))
}
