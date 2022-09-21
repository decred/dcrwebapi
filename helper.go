// Copyright (c) 2017-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"strings"
)

const (
	// semanticBuildAlphabet defines the allowed characters for the build
	// portion of a semantic version string.
	semanticBuildAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-.+"
)

// normalizeSemString returns the passed string stripped of all characters
// which are not valid according to the provided semantic versioning alphabet.
func normalizeSemString(str, alphabet string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(alphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NormalizeBuildString returns the passed string stripped of all characters
// which are not valid according to the semantic versioning guidelines for build
// metadata strings.  In particular they MUST only contain characters in
// semanticBuildAlphabet.
func NormalizeBuildString(str string) string {
	return normalizeSemString(str, semanticBuildAlphabet)
}

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

// round rounding func
func round(f float64, places uint) float64 {
	shift := math.Pow(10, float64(places))
	return math.Floor(f*shift+.5) / shift
}
