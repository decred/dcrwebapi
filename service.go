// Copyright (c) 2017-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	// StakepoolAPIInitialVersion is the initial stakepool API version.
	StakepoolAPIInitialVersion = 1

	// StakepoolAPICurrentVersion is the current stakepool API version.
	StakepoolAPICurrentVersion = 2
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

// Stakepool represents a decred stakepool solely for voting delegation.
type Stakepool struct {
	// APIEnabled defines if the api is enabled.
	APIEnabled bool `json:"APIEnabled"`

	// APIVersionsSupported contains the collection of API versions supported.
	APIVersionsSupported []interface{} `json:"APIVersionsSupported"`

	// Network defines the active network.
	Network string `json:"Network"`

	// URL contains the URL.
	URL string `json:"URL"`

	// Launched defines the timestamp of when
	// the stakepool was added.
	Launched int64 `json:"Launched"`

	// LastUpdated is the last updated time.
	LastUpdated int64 `json:"LastUpdated"`

	// Immature is the number of immature tickets.
	Immature int `json:"Immature"`

	// Live is the number of live tickets.
	Live int `json:"Live"`

	// Voted is the number of voted tickets.
	Voted int `json:"Voted"`

	// Missed is the number of missed votes.
	Missed int `json:"Missed"`

	// PoolFees is the pool fees.
	PoolFees float64 `json:"PoolFees"`

	// ProportionLive is the proportion of live tickets.
	ProportionLive float64 `json:"ProportionLive"`

	// ProportionMissed is the proportion of tickets missed.
	ProportionMissed float64 `json:"ProportionMissed"`

	// UserCount is the number of users.
	UserCount int `json:"UserCount"`

	// UserCountActive is the number of active users.
	UserCountActive int `json:"UserCountActive"`

	// Version is the software version advertised.
	Version string `json:"Version"`

	// BlockHeight is the height of the latest block processed by the VSP.
	BlockHeight int `json:"BlockHeight"`
}

// StakepoolSet represents a collection of stakepools.
type StakepoolSet map[string]Stakepool

// MarshalJSON custom marshaler for StakepoolSet, preserves data set
// randomness.
func (set StakepoolSet) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	length := len(set)
	count := 0

	for key, value := range set {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		buffer.WriteString(fmt.Sprintf("\"%s\":%s",
			key, string(jsonValue)))
		count++
		if count < length {
			buffer.WriteString(",")
		}
	}

	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

// Service represents a dcrweb service.
type Service struct {
	// the http client
	HTTPClient *http.Client
	// the http router
	Router *http.ServeMux
	// the stakepools
	Stakepools StakepoolSet
	Vsps       vspSet
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
			"vsp.coinmine.pl": Vsp{
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
		},

		// Historical launch dates have been collected from these sources:
		//   - https://github.com/decred/dcrwebapi/commit/09113670a5b411c9c0c988e5a8ea627ee00ac007
		//   - https://forum.decred.org/threads/ultrapool-eu-new-stakepool.5276/#post-25188
		//   - https://decred.slack.com/
		//   - https://github.com/decred/dcrwebapi/commit/9374b388624ad2b3f587d3effef39fc752d892ec
		//   - https://github.com/decred/dcrwebapi/commit/e76f621d33050a506ab733ff2bc2f47f9366726c
		Stakepools: StakepoolSet{
			"Delta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.stakeminer.com",
				Launched:             getUnixTime(2016, 5, 19),
			},
			"Hotel": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stake.decredbrasil.com",
				Launched:             getUnixTime(2016, 5, 28),
			},
			"Zeta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrstake.coinmine.pl",
				Launched:             getUnixTime(2018, 10, 22),
			},
		},
	}

	// fetch initial stakepool data
	stakepoolData(&service)

	// Fetch initial VSP data.
	vspData(&service)

	// Start update ticker.
	go func() {
		for {
			<-time.After(time.Minute * 5)
			stakepoolData(&service)
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

// stakepoolStats fetches statistics for a stakepool
func stakepoolStats(service *Service, key string, apiVersion int) error {
	var pool Stakepool

	service.Mutex.RLock()
	pool = service.Stakepools[key]
	service.Mutex.RUnlock()
	poolURL := fmt.Sprintf("%s/api/v%d/stats", pool.URL, apiVersion)

	poolRespBody, err := service.getHTTP(poolURL)
	if err != nil {
		return err
	}

	var poolData map[string]interface{}
	err = json.Unmarshal(poolRespBody, &poolData)
	if err != nil {
		return fmt.Errorf("%v: unmarshal failed: %v",
			poolURL, err)
	}

	status := poolData["status"].(string)
	if status != "success" {
		return fmt.Errorf("%v: non-success status '%v': %v",
			poolURL, status, string(poolRespBody))
	}

	data := poolData["data"].(map[string]interface{})
	_, hasImmature := data["Immature"]
	_, hasLive := data["Live"]
	_, hasVoted := data["Voted"]
	_, hasMissed := data["Missed"]
	_, hasPoolFees := data["PoolFees"]
	_, hasProportionLive := data["ProportionLive"]
	_, hasProportionMissed := data["ProportionMissed"]
	_, hasUserCount := data["UserCount"]
	_, hasUserCountActive := data["UserCountActive"]
	_, hasAPIVersionsSupported := data["APIVersionsSupported"]

	hasRequiredFields := hasImmature && hasLive && hasVoted && hasMissed &&
		hasPoolFees && hasProportionLive && hasProportionMissed &&
		hasUserCount && hasUserCountActive && hasAPIVersionsSupported

	if !hasRequiredFields {
		return fmt.Errorf("%v: missing required fields: %+v", poolURL, data)
	}

	poolVersion := NormalizeBuildString(data["Version"].(string))
	if len(poolVersion) > 13 {
		poolVersion = poolVersion[0:13]
	}
	pool.Version = poolVersion
	pool.BlockHeight = int(data["BlockHeight"].(float64))
	pool.Immature = int(data["Immature"].(float64))
	pool.Live = int(data["Live"].(float64))
	pool.Voted = int(data["Voted"].(float64))
	pool.Missed = int(data["Missed"].(float64))
	pool.PoolFees = data["PoolFees"].(float64)
	pool.ProportionLive = data["ProportionLive"].(float64)
	pool.ProportionMissed = data["ProportionMissed"].(float64)
	pool.UserCount = int(data["UserCount"].(float64))
	pool.UserCountActive = int(data["UserCountActive"].(float64))
	pool.APIVersionsSupported = data["APIVersionsSupported"].([]interface{})
	pool.APIEnabled = true
	pool.LastUpdated = time.Now().Unix()
	service.Mutex.Lock()
	service.Stakepools[key] = pool
	service.Mutex.Unlock()

	return nil
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

// stakepoolData fetches statistics for all listed DCR stakepools
func stakepoolData(service *Service) {
	var waitGroup sync.WaitGroup

	waitGroup.Add(len(service.Stakepools))
	for key := range service.Stakepools {
		// fetch the stakepool stats, trying the current version first
		// then the initial version if an error occurs
		go func(key string, service *Service) {
			defer waitGroup.Done()
			err := stakepoolStats(service, key, StakepoolAPICurrentVersion)
			if err != nil {
				log.Println(err)
				err := stakepoolStats(service, key, StakepoolAPIInitialVersion)
				if err != nil {
					log.Println(err)
				}
			}
		}(key, service)
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
	case "gsd":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.Stakepools)
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
