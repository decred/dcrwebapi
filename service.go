// Copyright (c) 2017-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
)

const (
	// StakepoolAPIInitialVersion is the initial stakepool API version.
	StakepoolAPIInitialVersion = 1

	// StakepoolAPICurrentVersion is the current stakepool API version.
	StakepoolAPICurrentVersion = 2
)

// CoinSupply represents the data structure returned by the gcs request.
type CoinSupply struct {
	Airdrop            float64 `json:"Airdrop"`
	CoinSupplyMined    float64 `json:"CoinSupplyMined"`
	CoinSupplyMinedRaw float64 `json:"CoinSupplyMinedRaw"`
	CoinSupplyTotal    float64 `json:"CoinSupplyTotal"`
	PercentMined       float64 `json:"PercentMined"`
	PoS                float64 `json:"Pos"`
	PoW                float64 `json:"Pow"`
	Premine            float64 `json:"Premine"`
	Subsidy            float64 `json:"Subsidy"`
}

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
	APIVersions   []int64 `json:"apiversions"`
	FeePercentage float64 `json:"feepercentage"`
	Closed        bool    `json:"closed"`
	Voting        int64   `json:"voting"`
	Voted         int64   `json:"voted"`
	Revoked       int64   `json:"revoked"`
}
type vspSet map[string]Vsp

// Stakepool represents a decred stakepool solely for voting delegation.
type Stakepool struct {
	// APIEnabled defines if the api is enabled.
	APIEnabled bool `json:"APIEnabled"`

	// APIVersionsSupported contains the collection of collection of API
	// versions supported.
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

// CacheEntry represents a cache entry with a specified expiry.
type CacheEntry struct {
	// Item defines the cached item.
	Item interface{}

	// Expire contains the item's expiry.
	Expiry time.Time
}

// Service represents a dcrweb service.
type Service struct {
	// the http client
	HTTPClient *http.Client
	// the http router
	Router *http.ServeMux
	// the service cache
	Cache sync.Map
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
		Cache:  sync.Map{},
		Mutex:  sync.RWMutex{},

		Vsps: vspSet{
			"teststakepool.decred.org": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2020, 6, 1),
			},
			"test.stakey.net": Vsp{
				Network:  "testnet",
				Launched: getUnixTime(2020, 7, 31),
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
		},

		// Historical launch dates have been collected from these sources:
		//   - https://github.com/decred/dcrwebapi/commit/09113670a5b411c9c0c988e5a8ea627ee00ac007
		//   - https://forum.decred.org/threads/ultrapool-eu-new-stakepool.5276/#post-25188
		//   - https://decred.slack.com/
		//   - https://github.com/decred/dcrwebapi/commit/9374b388624ad2b3f587d3effef39fc752d892ec
		//   - https://github.com/decred/dcrwebapi/commit/e76f621d33050a506ab733ff2bc2f47f9366726c
		Stakepools: StakepoolSet{
			"Alfa": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://test-dcrpool.dittrex.com",
				Launched:             getUnixTime(2019, 2, 17),
			},
			"Everstake": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.everstake.one",
				Launched:             getUnixTime(2019, 7, 23),
			},
			"Dittrex": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpool.dittrex.com",
				Launched:             getUnixTime(2018, 11, 28),
			},
			"Delta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.stakeminer.com",
				Launched:             getUnixTime(2016, 5, 19),
			},
			"Echo": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://pool.d3c.red",
				Launched:             getUnixTime(2016, 5, 23),
			},
			"Golf": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakepool.dcrstats.com",
				Launched:             getUnixTime(2016, 5, 25),
			},
			"Hotel": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stake.decredbrasil.com",
				Launched:             getUnixTime(2016, 5, 28),
			},
			"India": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakepool.eu",
				Launched:             getUnixTime(2016, 5, 22),
			},
			"Juliett": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.ubiqsmart.com",
				Launched:             getUnixTime(2016, 6, 12),
			},
			"Lima": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://ultrapool.eu",
				Launched:             getUnixTime(2017, 5, 23),
			},
			"Mike": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.farm",
				Launched:             getUnixTime(2017, 12, 21),
			},
			"November": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.raqamiya.net",
				Launched:             getUnixTime(2017, 12, 21),
			},
			"Papa": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakey.net",
				Launched:             getUnixTime(2018, 1, 22),
			},
			"Quebec": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://test.stakey.net",
				Launched:             getUnixTime(2018, 1, 22),
			},
			"Sierra": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decredvoting.com",
				Launched:             getUnixTime(2018, 8, 30),
			},
			"Life": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpool.ibitlin.com",
				Launched:             getUnixTime(2018, 7, 7),
			},
			"Scarmani": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakey.com",
				Launched:             getUnixTime(2018, 10, 12),
			},
			"Mega": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpos.megapool.info",
				Launched:             getUnixTime(2018, 10, 20),
			},
			"Zeta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrstake.coinmine.pl",
				Launched:             getUnixTime(2018, 10, 22),
			},
			"Tango": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://testnet.decredvoting.com",
				Launched:             getUnixTime(2018, 8, 30),
			},
			"99split": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://99split.com",
				Launched:             getUnixTime(2019, 12, 17),
			},
			"Charlie": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.yieldwallet.io",
				Launched:             getUnixTime(2020, 1, 29),
			},
			"Dinner": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://dcrstakedinner.com",
				Launched:             getUnixTime(2020, 3, 10),
			},
		},
	}

	// fetch initial stakepool data
	stakepoolData(&service)

	// Fetch initial VSP data.
	vspData(&service)

	// start stakepool update ticker
	stakepoolTicker := time.NewTicker(time.Minute * 5)
	go func() {
		for range stakepoolTicker.C {
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

	respBody, err := ioutil.ReadAll(poolResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to read body: %v",
			url, err)
	}

	return respBody, nil
}

// downloadCount calculates the cummulative download count for DCR binaries and releases
func downloadCount(service *Service) ([]string, error) {
	now := time.Now()
	entry, hasDc := service.Cache.Load("dc")
	if hasDc {
		// return cached response if not invalidated
		entry, ok := entry.(CacheEntry)
		if !ok {
			return nil, fmt.Errorf("bad item in dc cache")
		}

		if now.Before(entry.Expiry) {
			resp, ok := entry.Item.([]string)
			if !ok {
				return nil, fmt.Errorf("bad item")
			}
			return resp, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := github.NewClient(nil)
	binaries, _, err := client.Repositories.ListReleases(ctx,
		"decred", "decred-binaries", nil)
	if err != nil {
		return nil, err
	}

	var countB int
	for _, binary := range binaries {
		for _, asset := range binary.Assets {
			countB += asset.GetDownloadCount()
		}
	}

	releases, _, err := client.Repositories.ListReleases(ctx,
		"decred", "decred-release", nil)
	if err != nil {
		return nil, err
	}

	var countR int
	for _, release := range releases {
		for _, asset := range release.Assets {
			countR += asset.GetDownloadCount()
		}
	}

	count := countB + countR
	countStr := fmt.Sprintf("%dk", count/1000)
	resp := []string{"DownloadsCount", countStr}
	// cache response
	cacheEntry := CacheEntry{
		Item:   resp,
		Expiry: now.Add(4 * time.Hour),
	}
	service.Cache.Store("dc", cacheEntry)

	return resp, nil
}

// coinSupply returns the DCR coin supply on mainnet
func coinSupply(service *Service) (*CoinSupply, error) {
	now := time.Now()
	entry, hasGSC := service.Cache.Load("gsc")
	if hasGSC {
		// return cached response if not invalidated
		entry := entry.(CacheEntry)
		if now.Before(entry.Expiry) {
			resp := entry.Item.(*CoinSupply)
			return resp, nil
		}
	}

	supplyBody, err := service.getHTTP("https://dcrdata.decred.org/api/supply")
	if err != nil {
		return nil, err
	}

	var supply map[string]interface{}
	err = json.Unmarshal(supplyBody, &supply)
	if err != nil {
		return nil, err
	}

	currentCoinSupply := round(supply["supply_mined"].(float64), 1)
	airdrop := 840000.0
	premine := 840000.0
	coinSupplyAvailable := round(currentCoinSupply/100000000, 0)
	coinSupplyAfterGenesisBlock := coinSupplyAvailable - airdrop - premine
	totalCoinSupply := 21000000.0
	resp := &CoinSupply{
		PercentMined:       round((coinSupplyAvailable/totalCoinSupply)*100, 1),
		CoinSupplyMined:    coinSupplyAvailable,
		CoinSupplyMinedRaw: currentCoinSupply,
		CoinSupplyTotal:    21000000,
		Airdrop:            round(airdrop/coinSupplyAvailable*100, 1),
		Premine:            round(premine/coinSupplyAvailable*100, 1),
		PoS:                round((coinSupplyAfterGenesisBlock*.3)/coinSupplyAvailable*100, 1),
		PoW:                round((coinSupplyAfterGenesisBlock*.6)/coinSupplyAvailable*100, 1),
		Subsidy:            round((coinSupplyAfterGenesisBlock*.1)/coinSupplyAvailable*100, 1),
	}

	// cache response
	cacheEntry := CacheEntry{
		Item:   resp,
		Expiry: now.Add(1 * time.Minute),
	}
	service.Cache.Store("gsc", cacheEntry)

	return resp, nil
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

	hasRequiredFields := hasAPIVersions && hasFeePercentage &&
		hasClosed && hasVoting && hasVoted && hasRevoked

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

	vsp.LastUpdated = time.Now().Unix()

	service.Mutex.Lock()
	service.Vsps[url] = vsp
	service.Mutex.Unlock()

	return nil
}

func vspData(service *Service) {
	var waitGroup sync.WaitGroup
	for url := range service.Vsps {
		go func(url string) {
			waitGroup.Add(1)
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

// GetCoinSupply is the handler func for the `/gsc` route.
// It returns statistics on the DCR blockchain and the available coin supply
func (service *Service) GetCoinSupply(writer http.ResponseWriter, request *http.Request) {
	resp, err := coinSupply(service)
	if err != nil {
		writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
		return
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
		return
	}

	writeJSONResponse(&writer, http.StatusOK, &respJSON)
}

// HandleRoutes is the handler func for all endpoints exposed by the service
func (service *Service) HandleRoutes(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
		return
	}

	route := request.FormValue("c")
	switch route {
	case "dc":
		resp, err := downloadCount(service)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		respJSON, err := json.Marshal(resp)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return
	case "gcs":
		resp, err := coinSupply(service)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		respJSON, err := json.Marshal(resp)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return
	case "vsp":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.Vsps)
		service.Mutex.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return
	case "gsd":
		service.Mutex.RLock()
		respJSON, err := json.Marshal(service.Stakepools)
		service.Mutex.RUnlock()
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		writeJSONResponse(&writer, http.StatusOK, &respJSON)
		return
	case "cc":
		addr, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
		}

		if addr == "::1" || addr == "127.0.0.1" {
			service.Cache = sync.Map{}
			respJSON := []byte(`{"response": "cache cleared"}`)
			writeJSONResponse(&writer, http.StatusOK, &respJSON)
			return
		}

		respJSON := []byte(`{"response": "unauthorized"}`)
		writeJSONResponse(&writer, http.StatusBadRequest, &respJSON)
		return
	default:
		writer.WriteHeader(http.StatusNotFound)
		return
	}
}
