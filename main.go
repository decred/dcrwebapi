// Copyright (c) 2017-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

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
