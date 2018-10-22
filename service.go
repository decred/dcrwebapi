package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// SVGTemplate contains the SVG template.
	SVGTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><mask id="a"><rect width="128" height="20" rx="3" fill="#fff"/></mask><g mask="url(#a)"><path fill="#555" d="M0 0h69v20H0z"/><path fill="#4c1" d="M69 0h59v20H69z"/><path fill="url(#b)" d="M0 0h128v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11"><text x="34.5" y="15" fill="#010101" fill-opacity=".3">downloads</text><text x="34.5" y="14">downloads</text><text x="97.5" y="15" fill="#010101" fill-opacity=".3">__COUNT__ total</text><text x="97.5" y="14">__COUNT__ total</text></g></svg>`

	// StakepoolAPIInitialVersion is the initial stakepool API version.
	StakepoolAPIInitialVersion = 1

	// StakepoolAPICurrentVersion is the current stakepool API version.
	StakepoolAPICurrentVersion = 2
)

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
	// the ticker
	Ticker *time.Ticker
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
			Timeout: time.Minute * 10,
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
			"Bravo": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcr.stakepool.net",
				Launched:             getUnixTime(2016, 5, 22, 22, 54),
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
			// Stakepool API is unreachable for Foxtrot
			// "Foxtrot" = Stakepool{
			//   APIVersionsSupported: []interface{}{},
			//   Network:              "mainnet",
			//   URL:                  "https://dcrstakes.com",
			//   Launched:             getUnixTime(2016, 5, 31, 13, 23),
			// }
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
			"Oscar": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://pos.dcr.fans",
				Launched:             getUnixTime(2017, 12, 21, 17, 50),
			},
			"Papa": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://stakey.net",
				Launched:             getUnixTime(2018, 1, 22, 21, 04),
			},
			"Ray": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpos.idcray.com",
				Launched:             getUnixTime(2018, 2, 12, 14, 44),
			},
			"Sierra": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://decredvoting.com",
				Launched:             getUnixTime(2018, 8, 30, 11, 55),
			},
			"Ethan": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://tokensmart.io",
				Launched:             getUnixTime(2018, 4, 2, 16, 44),
			},
			"Life": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://dcrpool.ibitlin.com",
				Launched:             getUnixTime(2018, 7, 7, 1, 10),
			},
			"James": {
				APIVersionsSupported: []interface{}{},
				Network:              "mainnet",
				URL:                  "https://d1pool.com",
				Launched:             getUnixTime(2018, 8, 9, 22, 10),
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
		},
		StakepoolKeys: []string{},
	}

	// get the stakepool key set.
	for key := range service.Stakepools {
		service.StakepoolKeys = append(service.StakepoolKeys, key)
	}

	// fetch initial stakepool data
	stakepoolData(&service)

	// start stakepool update ticker
	service.Ticker = time.NewTicker(time.Minute * 5)
	go func() {
		for range service.Ticker.C {
			stakepoolData(&service)
		}
	}()

	// setup route
	service.Router.HandleFunc("/", service.HandleRoutes)
	return &service
}

// filterDownloadCount filters the download dataset returning a map of
// asset filenames and their download counts
func filterDownloadCount(dataset []interface{}) int64 {
	var count int64

	for _, entry := range dataset {
		entry := entry.(map[string]interface{})
		_, hasAssets := entry["assets"].([]interface{})
		if hasAssets {
			assets := entry["assets"].([]interface{})
			for _, asset := range assets {
				asset := asset.(map[string]interface{})
				_, hasName := asset["name"].(string)
				_, hasDownloadCount := asset["download_count"].(float64)
				if hasName && hasDownloadCount {
					count += int64(asset["download_count"].(float64))
				}
			}
		}
	}

	return count
}

// downloadCount calculates the cummulative download count for DCR binaries and releases
func downloadCount(service *Service) ([]string, error) {
	now := time.Now()
	entry, hasDc := service.Cache.Load("dc")
	if hasDc {
		// return cached response if not invalidated
		entry, ok := entry.(CacheEntry)
		if !ok {
			return nil, fmt.Errorf("bad item")
		}

		if now.Before(entry.Expiry) {
			resp, ok := entry.Item.([]string)
			if !ok {
				return nil, fmt.Errorf("bad item")
			}
			return resp, nil
		}
	}

	// fetch all binaries
	binReq, err := http.NewRequest("GET", "https://api.github.com/repos/decred/decred-binaries/releases", nil)
	if err != nil {
		return nil, err
	}

	binReq.Header.Set("User-Agent", "decred/dcrweb bot")
	binResp, err := service.HTTPClient.Do(binReq)
	if err != nil {
		return nil, err
	}
	defer binResp.Body.Close()

	if binResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code %d, got %d", http.StatusOK, binResp.StatusCode)
	}

	binBody, err := ioutil.ReadAll(binResp.Body)
	if err != nil {
		return nil, err
	}

	var binaries []interface{}
	err = json.Unmarshal(binBody, &binaries)
	if err != nil {
		return nil, err
	}

	countB := filterDownloadCount(binaries)

	// fetch all releases
	relReq, err := http.NewRequest("GET", "https://api.github.com/repos/decred/decred-release/releases", nil)
	if err != nil {
		return nil, err
	}

	relReq.Header.Set("User-Agent", "decred/dcrweb bot")
	relResp, err := service.HTTPClient.Do(relReq)
	if err != nil {
		return nil, err
	}
	defer relResp.Body.Close()

	if relResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code %d, got %d", http.StatusOK, relResp.StatusCode)
	}

	relBody, err := ioutil.ReadAll(relResp.Body)
	if err != nil {
		return nil, err
	}

	var releases []interface{}
	err = json.Unmarshal(relBody, &releases)
	if err != nil {
		return nil, err
	}

	countR := filterDownloadCount(releases)

	count := countB + countR
	countStr := fmt.Sprintf("%dk", count/1000)
	resp := []string{"DownloadsCount", countStr}
	// cache response
	cacheEntry := CacheEntry{
		Item:   resp,
		Expiry: getFutureTime(&now, 0, 4, 0, 0),
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
		Expiry: getFutureTime(&now, 0, 4, 0, 0),
	}
	service.Cache.Store("dic", cacheEntry)
	return updatedSVG, nil
}

// insightStatus fetches blockchain explorer related statistics
// (mainnet.decred.org)
func insightStatus(service *Service) (map[string]interface{}, error) {
	now := time.Now()
	entry, hasGIS := service.Cache.Load("gis")
	if hasGIS {
		// return cached response if not invalidated
		entry := entry.(CacheEntry)
		if now.Before(entry.Expiry) {
			resp := entry.Item.(map[string]interface{})
			return resp, nil
		}
	}

	statusReq, err := http.NewRequest("GET",
		"https://mainnet.decred.org/api/status", nil)
	if err != nil {
		return nil, err
	}

	statusReq.Header.Set("User-Agent", "decred/dcrweb bot")
	statusResp, err := service.HTTPClient.Do(statusReq)
	if err != nil {
		return nil, err
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code %d, got %d", http.StatusOK, statusResp.StatusCode)
	}

	statusBody, err := ioutil.ReadAll(statusResp.Body)
	if err != nil {
		return nil, err
	}

	var status map[string]interface{}
	err = json.Unmarshal(statusBody, &status)
	if err != nil {
		return nil, err
	}

	// cache response
	cacheEntry := CacheEntry{
		Item:   status,
		Expiry: getFutureTime(&now, 0, 0, 1, 0),
	}
	service.Cache.Store("gis", cacheEntry)
	return status, nil
}

// coinSupply returns the DCR coin supply on mainnet
func coinSupply(service *Service) (map[string]interface{}, error) {
	now := time.Now()
	entry, hasGSC := service.Cache.Load("gsc")
	if hasGSC {
		// return cached response if not invalidated
		entry := entry.(CacheEntry)
		if now.Before(entry.Expiry) {
			resp := entry.Item.(map[string]interface{})
			return resp, nil
		}
	}

	supplyReq, err := http.NewRequest("GET", "https://mainnet.decred.org/api/status?q=getCoinSupply", nil)
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

	currentCoinSupply := round(supply["coinsupply"].(float64), 1)
	airdrop := 840000.0
	premine := 840000.0
	coinSupplyAvailable := round(currentCoinSupply/100000000, 0)
	coinSupplyAfterGenesisBlock := coinSupplyAvailable - airdrop - premine
	totalCoinSupply := 21000000.0
	resp := map[string]interface{}{
		"PercentMined":       round((coinSupplyAvailable/totalCoinSupply)*100, 1),
		"CoinSupplyMined":    coinSupplyAvailable,
		"CoinSupplyMinedRaw": currentCoinSupply,
		"CoinSupplyTotal":    21000000,
		"Airdrop":            round(airdrop/coinSupplyAvailable*100, 1),
		"Premine":            round(premine/coinSupplyAvailable*100, 1),
		"Pos":                round((coinSupplyAfterGenesisBlock*.3)/coinSupplyAvailable*100, 1),
		"Pow":                round((coinSupplyAfterGenesisBlock*.6)/coinSupplyAvailable*100, 1),
		"Subsidy":            round((coinSupplyAfterGenesisBlock*.1)/coinSupplyAvailable*100, 1),
	}

	// cache response
	cacheEntry := CacheEntry{
		Item:   resp,
		Expiry: getFutureTime(&now, 0, 0, 1, 0),
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
		return err
	}

	poolReq.Header.Set("User-Agent", "decred/dcrweb bot")
	poolResp, err := service.HTTPClient.Do(poolReq)
	if err != nil {
		return err
	}
	defer poolResp.Body.Close()

	if poolResp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status code %d, got %d", http.StatusOK, poolResp.StatusCode)
	}

	poolRespBody, err := ioutil.ReadAll(poolResp.Body)
	if err != nil {
		return err
	}

	var poolData map[string]interface{}
	err = json.Unmarshal(poolRespBody, &poolData)
	if err != nil {
		return err
	}

	status := poolData["status"].(string)
	if status != "success" {
		return fmt.Errorf("expected success status, got %v status. Response: %v",
			status, string(poolRespBody))
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
		return fmt.Errorf("%v: missing required fields: %+v", pool.URL, data)
	}

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
	writer.Header().Set("Access-Control-Allow-Origin", "*")
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
	case "gis":
		resp, err := insightStatus(service)
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
	case "gsd":
		respJSON, err := json.Marshal(service.Stakepools)
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
