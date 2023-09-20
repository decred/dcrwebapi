// Copyright (c) 2017-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Vsp contains information about a single Voting Service Provider. Includes
// info hard-coded in dcrwebapi and info retrieved from the VSPs /vspinfo
// endpoint.
type Vsp struct {
	// Hard-coded in dcrwebapi.
	Network  string `json:"network"`
	Launched int64  `json:"launched"`
	// Set by dcrwebapi each time info is successfully updated.
	LastUpdated int64 `json:"lastupdated"`
	// Retrieved from the /api/vspinfo.
	APIVersions                []int64 `json:"apiversions"`
	FeePercentage              float64 `json:"feepercentage"`
	Closed                     bool    `json:"closed"`
	Voting                     int64   `json:"voting"`
	Voted                      int64   `json:"voted"`
	Revoked                    int64   `json:"revoked"`
	VspdVersion                string  `json:"vspdversion"`
	BlockHeight                uint64  `json:"blockheight"`
	EstimatedNetworkProportion float64 `json:"estimatednetworkproportion"`
}
type vspSet map[string]Vsp

// Service represents a dcrweb service.
type Service struct {
	// the http client
	HTTPClient *http.Client
	// the http router
	Router *http.ServeMux
	Vsps   vspSet
	// the pool update mutex
	Mutex sync.RWMutex
}

// NewService creates a new dcrwebapi service.
func NewService() *Service {
	service := Service{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 2,
			},
			Timeout: time.Second * 10,
		},
		Router: http.NewServeMux(),
		Mutex:  sync.RWMutex{},

		Vsps: vspSet{
			"teststakepool.decred.org": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2020, 6, 1),
			},
			"testnet-vsp.jholdstock.uk": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2021, 1, 20),
			},
			"dcrvsp.ubiqsmart.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 25),
			},
			"stakey.net": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 10, 22),
			},
			"vsp.stakeminer.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 9),
			},
			"vsp.decredcommunity.org": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 05),
			},
			"vspd.99split.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 17),
			},
			"vspd.decredbrasil.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 11, 22),
			},
			"ultravsp.uk": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 1),
			},
			"vsp.dcr.farm": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2020, 12, 9),
			},
			"decredvoting.com": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2021, 2, 1),
			},
			"decred.stake.fun": Vsp{
				Network:  "mainnet",
				Launched: getUnixTime(2021, 1, 28),
			},
			"123.dcr.rocks": {
				Network:  "mainnet",
				Launched: getUnixTime(2021, 4, 28),
			},
			"big.decred.energy": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 5, 1),
			},
			"dcrhive.com": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 6, 23),
			},
			"vspd.bass.cf": {
				Network:  "mainnet",
				Launched: getUnixTime(2022, 5, 1),
			},
			"vote.dcr-swiss.ch": {
				Network:  "mainnet",
				Launched: getUnixTime(2023, 6, 30),
			},
		},
	}

	// Fetch initial VSP data.
	vspData(&service)

	// Start update ticker.
	go func() {
		for {
			<-time.After(time.Minute * 5)
			vspData(&service)
		}
	}()

	// setup route
	service.Router.HandleFunc("/", service.HandleRoutes)
	return &service
}

// getHTTP will use the services HTTP client to send a GET request to the
// provided URL. Returns the response body, or an error.
func (service *Service) getHTTP(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to create request: %v",
			url, err)
	}

	req.Header.Set("User-Agent", "decred/dcrweb bot")
	poolResp, err := service.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to send request: %v",
			url, err)
	}
	defer poolResp.Body.Close()

	if poolResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v: non-success status: %d",
			url, poolResp.StatusCode)
	}

	respBody, err := io.ReadAll(poolResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to read body: %v",
			url, err)
	}

	return respBody, nil
}

func vspStats(service *Service, url string) error {
	var vsp Vsp

	service.Mutex.RLock()
	vsp = service.Vsps[url]
	service.Mutex.RUnlock()
	infoURL := fmt.Sprintf("https://%s/api/v3/vspinfo", url)

	infoResp, err := service.getHTTP(infoURL)
	if err != nil {
		return err
	}

	var info map[string]interface{}
	err = json.Unmarshal(infoResp, &info)
	if err != nil {
		return fmt.Errorf("%v: unmarshal failed: %v",
			infoURL, err)
	}

	apiversions, hasAPIVersions := info["apiversions"]
	feepercentage, hasFeePercentage := info["feepercentage"]
	vspclosed, hasClosed := info["vspclosed"]
	voting, hasVoting := info["voting"]
	voted, hasVoted := info["voted"]
	revoked, hasRevoked := info["revoked"]
	version, hasVersion := info["vspdversion"]
	blockheight, hasBlockHeight := info["blockheight"]
	networkproportion, hasnetworkproportion := info["estimatednetworkproportion"]

	hasRequiredFields := hasAPIVersions && hasFeePercentage &&
		hasClosed && hasVoting && hasVoted && hasRevoked && hasVersion &&
		hasBlockHeight && hasnetworkproportion

	if !hasRequiredFields {
		return fmt.Errorf("%v: missing required fields: %+v", infoURL, info)
	}

	vsp.APIVersions = make([]int64, 0)
	for _, i := range apiversions.([]interface{}) {
		vsp.APIVersions = append(vsp.APIVersions, int64(i.(float64)))
	}

	vsp.FeePercentage = feepercentage.(float64)
	vsp.Closed = vspclosed.(bool)
	vsp.Voting = int64(voting.(float64))
	vsp.Voted = int64(voted.(float64))
	vsp.Revoked = int64(revoked.(float64))
	vsp.VspdVersion = version.(string)
	vsp.BlockHeight = uint64(blockheight.(float64))
	vsp.EstimatedNetworkProportion = networkproportion.(float64)

	vsp.LastUpdated = time.Now().Unix()

	service.Mutex.Lock()
	service.Vsps[url] = vsp
	service.Mutex.Unlock()

	return nil
}

func vspData(service *Service) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(service.Vsps))
	for url := range service.Vsps {
		go func(url string) {
			defer waitGroup.Done()
			err := vspStats(service, url)
			if err != nil {
				log.Println(err)
			}
		}(url)
	}
	waitGroup.Wait()
}

func getUnixTime(year int, month time.Month, day int) int64 {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()
}

// HandleRoutes is the handler func for all endpoints exposed by the service
func (service *Service) HandleRoutes(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		writeJSONErrorResponse(&writer, err)
		return
	}

	route := request.FormValue("c")
	switch route {

	case "vsp":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.Vsps)
		service.Mutex.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return

	default:
		writer.WriteHeader(http.StatusNotFound)
		return
	}
}
