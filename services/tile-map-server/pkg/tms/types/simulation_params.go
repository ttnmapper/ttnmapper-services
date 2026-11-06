package types

import (
	"encoding/json"
	"os"
	"sync"
)

type SimulationParams struct {
	Climate      int     `json:"climate"`
	Clutter      float64 `json:"clutter"`
	Conductivity float64 `json:"conductivity"`
	Dielectric   float64 `json:"dielectric"`
	Reliability  float64 `json:"reliability"`
	ConstantLoss float64 `json:"constant_loss"`
	HeightAgl    float64 `json:"height_agl"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`

	// Progress tracking
	LocationLastRMSE   float64 `json:"location_last_rmse"`
	LocationBestRadius float64 `json:"location_best_radius"`
	RadioLastRMSE      float64 `json:"radio_last_rmse"`
	LocationTriesDone  int     `json:"location_tries_done"`
	RadioTriesDone     int     `json:"radio_tries_done"`
}

// Only the configured file is retained; lookups stat it to detect offline edits.
// The loaded map is immutable and callers receive SimulationParams by value.
var gatewayParamsCache struct {
	sync.Mutex
	path   string
	info   os.FileInfo
	params map[string]map[string]SimulationParams
}

func loadGatewayParamsFile(path string) map[string]map[string]SimulationParams {
	gatewayParamsCache.Lock()
	defer gatewayParamsCache.Unlock()
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if gatewayParamsCache.path == path && gatewayParamsCache.info != nil &&
		os.SameFile(info, gatewayParamsCache.info) && info.Size() == gatewayParamsCache.info.Size() &&
		info.ModTime().Equal(gatewayParamsCache.info.ModTime()) {
		return gatewayParamsCache.params
	}
	var params map[string]map[string]SimulationParams
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &params) != nil {
		// An offline writer may be midway through replacing the file. Retry
		// on the next lookup instead of retaining a transient parse failure.
		return nil
	}
	if params == nil {
		params = make(map[string]map[string]SimulationParams)
	}

	gatewayParamsCache.path = path
	gatewayParamsCache.info = info
	gatewayParamsCache.params = params
	return params
}

func GetGatewayParams(paramsFile string, networkID string, gatewayID string, defaultLat float64, defaultLon float64, defaultAltM int) SimulationParams {
	params := SimulationParams{
		Climate:            5,
		Clutter:            0.0,
		Conductivity:       0.005,
		Dielectric:         15.0,
		Reliability:        90.0,
		ConstantLoss:       0.0,
		HeightAgl:          0.0,
		Lat:                defaultLat,
		Lon:                defaultLon,
		LocationLastRMSE:   0.0,
		LocationBestRadius: 0.0,
		RadioLastRMSE:      0.0,
		LocationTriesDone:  0,
		RadioTriesDone:     0,
	}

	if paramsFile == "" {
		return params
	}

	allParams := loadGatewayParamsFile(paramsFile)
	if allParams == nil {
		return params
	}

	if netParams, ok := allParams[networkID]; ok {
		if gwParams, ok := netParams[gatewayID]; ok {
			// Merge loaded params into our defaults to ensure all fields are present
			if gwParams.Climate != 0 {
				params.Climate = gwParams.Climate
			}
			if gwParams.Clutter != 0 {
				params.Clutter = gwParams.Clutter
			}
			if gwParams.Conductivity != 0 {
				params.Conductivity = gwParams.Conductivity
			}
			if gwParams.Dielectric != 0 {
				params.Dielectric = gwParams.Dielectric
			}
			if gwParams.Reliability != 0 {
				params.Reliability = gwParams.Reliability
			}
			if gwParams.ConstantLoss != 0 {
				params.ConstantLoss = gwParams.ConstantLoss
			}
			if gwParams.HeightAgl != 0 {
				params.HeightAgl = gwParams.HeightAgl
			}
			if gwParams.Lat != 0 {
				params.Lat = gwParams.Lat
			} else if defaultLat != 0 {
				params.Lat = defaultLat
			}
			if gwParams.Lon != 0 {
				params.Lon = gwParams.Lon
			} else if defaultLon != 0 {
				params.Lon = defaultLon
			}

			params.LocationLastRMSE = gwParams.LocationLastRMSE
			params.LocationBestRadius = gwParams.LocationBestRadius
			params.RadioLastRMSE = gwParams.RadioLastRMSE
			params.LocationTriesDone = gwParams.LocationTriesDone
			params.RadioTriesDone = gwParams.RadioTriesDone

			return params
		}
	}

	// If we are here, it means the gateway was not in the params file.
	// Use DB altitude if available.
	if params.HeightAgl == 0 && defaultAltM != 0 {
		params.HeightAgl = float64(defaultAltM)
	}

	// Final fallback for height
	if params.HeightAgl == 0 {
		params.HeightAgl = 2.0
	}

	return params
}

func SaveTemporaryParams(paramsFile string, networkID string, gatewayID string, params SimulationParams) {
	if paramsFile == "" {
		return
	}
	allParams := make(map[string]map[string]SimulationParams)
	data, err := os.ReadFile(paramsFile)
	if err == nil {
		if err := json.Unmarshal(data, &allParams); err != nil {
			// If file exists but is invalid, we might overwrite it or log error.
			// For now, let's just proceed with an empty map if it fails to unmarshal.
		}
	}

	if _, ok := allParams[networkID]; !ok {
		allParams[networkID] = make(map[string]SimulationParams)
	}
	allParams[networkID][gatewayID] = params

	newData, err := json.MarshalIndent(allParams, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(paramsFile, newData, 0644)
}

func SaveFinalParams(paramsFile string, networkID string, gatewayID string, params SimulationParams) {
	SaveTemporaryParams(paramsFile, networkID, gatewayID, params)
}
