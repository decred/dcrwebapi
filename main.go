// Copyright (c) 2017-2018 The Decred developers
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
	defaultPort = ":8089"
)

func main() {
	service := NewService()
	log.Println("dcrwebapi starting on", defaultPort)

	origins := handlers.AllowedOrigins([]string{"*"})
	methods := handlers.AllowedMethods([]string{"GET", "OPTIONS"})

	log.Fatal(http.ListenAndServe(defaultPort,
		handlers.CORS(origins, methods)(service.Router)))
}
