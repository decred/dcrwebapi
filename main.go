// Copyright (c) 2017-2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"

	"github.com/gorilla/handlers"
)

const (
	// the listening port
	defaultPort = ":8080"
)

func main() {
	service := NewService()
	log.Println("dcrwebapi starting on", defaultPort)

	origins := handlers.AllowedOrigins([]string{"*"})
	methods := handlers.AllowedMethods([]string{http.MethodGet, http.MethodOptions})

	log.Fatal(http.ListenAndServe(defaultPort,
		handlers.CORS(origins, methods)(service.Router)))
}
