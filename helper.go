// Copyright (c) 2017-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
)

// writeJSONResponse convenience func for writing json responses
func writeJSONResponse(writer *http.ResponseWriter, code int,
	respJSON *[]byte) {
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).Header().Set("Strict-Transport-Security", "max-age=15552001")
	(*writer).Header().Set("Vary", "Accept-Encoding")
	(*writer).Header().Set("X-Content-Type-Options", "nosniff")
	(*writer).WriteHeader(code)
	(*writer).Write(*respJSON)
}

// writeJSONErrorResponse convenience func for writing json error responses
func writeJSONErrorResponse(writer *http.ResponseWriter, err error) {
	errorBody := map[string]interface{}{}
	errorBody["error"] = err.Error()
	errorJSON, _ := json.Marshal(errorBody)
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).Header().Set("Strict-Transport-Security", "max-age=15552001")
	(*writer).Header().Set("Vary", "Accept-Encoding")
	(*writer).WriteHeader(http.StatusInternalServerError)
	(*writer).Write(errorJSON)
}
