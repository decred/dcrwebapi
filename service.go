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
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
)

const (
	// SVGTemplate contains the SVG template.
	SVGTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><mask id="a"><rect width="128" height="20" rx="3" fill="#fff"/></mask><g mask="url(#a)"><path fill="#555" d="M0 0h69v20H0z"/><path fill="#4c1" d="M69 0h59v20H69z"/><path fill="url(#b)" d="M0 0h128v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11"><text x="34.5" y="15" fill="#010101" fill-opacity=".3">downloads</text><text x="34.5" y="14">downloads</text><text x="97.5" y="15" fill="#010101" fill-opacity=".3">__COUNT__ total</text><text x="97.5" y="14">__COUNT__ total</text></g></svg>`

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
	// the stakepool keys
	StakepoolKeys []string
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
				Launched:             getUnixTime(2019, 2, 17, 14, 0),
			},
			"Everstake": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.everstake.one",
				Launched:             getUnixTime(2019, 7, 23, 15, 46),
			},
			"Dittrex": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpool.dittrex.com",
				Launched:             getUnixTime(2018, 11, 28, 16, 13),
			},
			"Delta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.stakeminer.com",
				Launched:             getUnixTime(2016, 5, 19, 15, 19),
			},
			"Echo": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://pool.d3c.red",
				Launched:             getUnixTime(2016, 5, 23, 17, 59),
			},
			"Golf": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakepool.dcrstats.com",
				Launched:             getUnixTime(2016, 5, 25, 9, 9),
			},
			"Hotel": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stake.decredbrasil.com",
				Launched:             getUnixTime(2016, 5, 28, 19, 31),
			},
			"India": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakepool.eu",
				Launched:             getUnixTime(2016, 5, 22, 18, 58),
			},
			"Juliett": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.ubiqsmart.com",
				Launched:             getUnixTime(2016, 6, 12, 20, 52),
			},
			"Kilo": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://teststakepool.decred.org",
				Launched:             getUnixTime(2017, 2, 7, 22, 0),
			},
			"Lima": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://ultrapool.eu",
				Launched:             getUnixTime(2017, 5, 23, 10, 16),
			},
			"Mike": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.farm",
				Launched:             getUnixTime(2017, 12, 21, 17, 50),
			},
			"November": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.raqamiya.net",
				Launched:             getUnixTime(2017, 12, 21, 17, 50),
			},
			"Papa": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakey.net",
				Launched:             getUnixTime(2018, 1, 22, 21, 04),
			},
			"Quebec": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://test.stakey.net",
				Launched:             getUnixTime(2018, 1, 22, 21, 04),
			},
			"Sierra": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decredvoting.com",
				Launched:             getUnixTime(2018, 8, 30, 11, 55),
			},
			"Life": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpool.ibitlin.com",
				Launched:             getUnixTime(2018, 7, 7, 1, 10),
			},
			"Scarmani": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakey.com",
				Launched:             getUnixTime(2018, 10, 12, 15, 10),
			},
			"Mega": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpos.megapool.info",
				Launched:             getUnixTime(2018, 10, 20, 9, 30),
			},
			"Zeta": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrstake.coinmine.pl",
				Launched:             getUnixTime(2018, 10, 22, 22, 30),
			},
			"Staked": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.staked.us",
				Launched:             getUnixTime(2018, 11, 28, 19, 30),
			},
			"Tango": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://testnet.decredvoting.com",
				Launched:             getUnixTime(2018, 8, 30, 11, 55),
			},
			"99split": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://99split.com",
				Launched:             getUnixTime(2019, 12, 17, 1, 57),
			},
			"Charlie": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decred.yieldwallet.io",
				Launched:             getUnixTime(2020, 1, 29, 15, 32),
			},
			"Dinner": {
				APIVersionsSupported: []interface{}{},
				Network:              "testnet",
				URL:                  "https://dcrstakedinner.com",
				Launched:             getUnixTime(2020, 3, 10, 15, 28),
			},
		},
		StakepoolKeys: []string{},
	}

	// get the stakepool key set.
	service.StakepoolKeys = make([]string, 0, len(service.Stakepools))
	for key := range service.Stakepools {
		service.StakepoolKeys = append(service.StakepoolKeys, key)
	}

	// fetch initial stakepool data
	stakepoolData(&service)

	// start stakepool update ticker
	stakepoolTicker := time.NewTicker(time.Minute * 5)
	go func() {
		for range stakepoolTicker.C {
			stakepoolData(&service)
		}
	}()

	// setup route
	service.Router.HandleFunc("/", service.HandleRoutes)
	return &service
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

// downloadsImageCache generates a downloads count image svg
func downloadsImageCache(service *Service) (string, error) {
	now := time.Now()
	entry, hasDic := service.Cache.Load("dic")
	if hasDic {
		// return cached response if not invalidated
		entry := entry.(CacheEntry)
		if now.Before(entry.Expiry) {
			resp := entry.Item.(string)
			return resp, nil
		}
	}

	// get the assets download count
	var dlCount string
	entry, hasDc := service.Cache.Load("dc")
	if hasDc {
		entry := entry.(CacheEntry)
		if now.Before(entry.Expiry) {
			resp := entry.Item.([]string)
			dlCount = resp[1]
		}
	}

	if len(dlCount) == 0 {
		resp, err := downloadCount(service)
		if err != nil {
			return "", err
		}

		dlCount = (resp)[1]
	}

	updatedSVG := strings.Replace(SVGTemplate, "__COUNT__", dlCount, -1)
	// cache response
	cacheEntry := CacheEntry{
		Item:   updatedSVG,
		Expiry: now.Add(4 * time.Hour),
	}
	service.Cache.Store("dic", cacheEntry)
	return updatedSVG, nil
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

	supplyReq, err := http.NewRequest("GET", "https://dcrdata.decred.org/api/supply", nil)
	if err != nil {
		return nil, err
	}

	supplyReq.Header.Set("User-Agent", "decred/dcrweb bot")
	supplyResp, err := service.HTTPClient.Do(supplyReq)
	if err != nil {
		return nil, err
	}
	defer supplyResp.Body.Close()

	if supplyResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code %d, got %d", http.StatusOK, supplyResp.StatusCode)
	}

	supplyBody, err := ioutil.ReadAll(supplyResp.Body)
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
	poolReq, err := http.NewRequest("GET", poolURL, nil)
	if err != nil {
		return fmt.Errorf("%v: failed to create request: %v",
			poolURL, err)
	}

	poolReq.Header.Set("User-Agent", "decred/dcrweb bot")
	poolResp, err := service.HTTPClient.Do(poolReq)
	if err != nil {
		return fmt.Errorf("%v: failed to send request: %v",
			poolURL, err)
	}
	defer poolResp.Body.Close()

	if poolResp.StatusCode != http.StatusOK {
		return fmt.Errorf("%v: non-success status: %d",
			poolURL, poolResp.StatusCode)
	}

	poolRespBody, err := ioutil.ReadAll(poolResp.Body)
	if err != nil {
		return fmt.Errorf("%v: failed to read body: %v",
			poolURL, err)
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

// stakepoolData fetches statistics for all listed DCR stakepools
func stakepoolData(service *Service) {
	var waitGroup sync.WaitGroup

	waitGroup.Add(len(service.Stakepools))
	for _, key := range service.StakepoolKeys {
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

func getUnixTime(year int, month time.Month, day, hour, min int) int64 {
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC).Unix()
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
	case "dic":
		resp, err := downloadsImageCache(service)
		if err != nil {
			writeJSONErrorResponse(&writer, http.StatusInternalServerError, err)
			return
		}

		writeSVGResponse(&writer, http.StatusOK, &resp)
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
