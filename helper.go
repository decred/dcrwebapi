package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

// writeJSONResponse convenience func for writing json responses
func writeJSONResponse(writer *http.ResponseWriter, code int,
	respJSON *[]byte) {
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).Header().Set("Strict-Transport-Security", "max-age=15552001")
	(*writer).Header().Set("Vary", "Accept-Encoding")
	(*writer).WriteHeader(code)
	(*writer).Write(*respJSON)
}

// writeSVGResponse convenience func for writing svg responses
func writeSVGResponse(writer *http.ResponseWriter, code int,
	svg *string) {
	(*writer).Header().Set("Strict-Transport-Security", "max-age=15552001")
	(*writer).Header().Set("Vary", "Accept-Encoding")
	(*writer).Header().Set("Content-Type", "image/svg+xml")
	(*writer).WriteHeader(code)
	fmt.Fprint(*writer, *svg)
}

// writeJSONErrorResponse convenience func for writing json error responses
func writeJSONErrorResponse(writer *http.ResponseWriter, code int, err error) {
	errorBody := map[string]interface{}{}
	errorBody["error"] = err.Error()
	errorJSON, _ := json.Marshal(errorBody)
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).Header().Set("Strict-Transport-Security", "max-age=15552001")
	(*writer).Header().Set("Vary", "Accept-Encoding")
	(*writer).WriteHeader(code)
	(*writer).Write(errorJSON)
}

// getFutureTime extends a base time to a future time using a duration
func getFutureTime(date *time.Time, days time.Duration, hours time.Duration,
	minutes time.Duration, seconds time.Duration) *time.Time {
	duration := ((time.Hour * 24) * days) + (time.Hour * hours) + (time.Minute * minutes) + (time.Second * seconds)
	futureTime := date.Add(duration)
	return &futureTime
}

// round rounding func
func round(f float64, places uint) float64 {
	shift := math.Pow(10, float64(places))
	return math.Floor(f*shift+.5) / shift
}
