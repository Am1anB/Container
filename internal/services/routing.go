package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func getNostraKey() string {
	key := os.Getenv("NOSTRA_API_KEY")
	if key == "" {
		log.Println("[WARN] getNostraKey: NOSTRA_API_KEY is not set in environment.")
	}
	return key
}

type nostraGeocodeResponse struct {
	Results []struct {
		LatLon string `json:"LatLon"`
	} `json:"results"`
	ErrorMessage string `json:"errorMessage"`
}

func GeocodeAddress(address string) (Location, error) {
	nostraApiKey := getNostraKey()

	if nostraApiKey == "" {
		log.Println("[ERROR] GeocodeAddress: NOSTRA_API_KEY is not set.")
		// คืนค่า Default หรือ Error ตามต้องการ (ในที่นี้คืน Error เพื่อให้รู้ว่าไม่ได้ Key)
		return Location{}, fmt.Errorf("NOSTRA_API_KEY is not set")
	}

	baseURL := "https://api.nostramap.com/Service/V2/Location/Search"
	apiURL := fmt.Sprintf(
		"%s?key=%s&keyword=%s&limit=1",
		baseURL,
		url.QueryEscape(nostraApiKey),
		url.QueryEscape(address),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return Location{}, fmt.Errorf("failed to create geocode request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Location{}, fmt.Errorf("nostra geocode http request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Location{}, fmt.Errorf("failed to read nostra geocode response: %v", err)
	}

	var apiResp nostraGeocodeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("[DEBUG] Nostra Geocode JSON Error (Raw Body): %s", string(body))
		return Location{}, fmt.Errorf("failed to parse nostra geocode response: %s", err.Error())
	}

	if apiResp.ErrorMessage != "" {
		log.Printf("[ERROR] Nostra Geocode API returned an error. Raw Body: %s", string(body))
		return Location{}, fmt.Errorf("Nostra API Error: %s", apiResp.ErrorMessage)
	}

	if len(apiResp.Results) == 0 {
		log.Printf("[WARN] Nostra Geocode returned no results for address: %s", address)
		return Location{}, fmt.Errorf("no geocode results found for: %s", address)
	}

	latLonStr := apiResp.Results[0].LatLon
	parts := strings.Split(latLonStr, ",")
	if len(parts) != 2 {
		return Location{}, fmt.Errorf("invalid LatLon format from API: %s", latLonStr)
	}

	lat, errLat := strconv.ParseFloat(parts[0], 64)
	lng, errLng := strconv.ParseFloat(parts[1], 64)
	if errLat != nil || errLng != nil {
		return Location{}, fmt.Errorf("failed to parse LatLon values: %s", latLonStr)
	}

	location := Location{Lat: lat, Lng: lng}
	log.Printf("[INFO] Geocoded (Nostra) '%s' -> Lat: %f, Lng: %f", address, location.Lat, location.Lng)
	return location, nil
}

type nostraRouteResponse struct {
	Routes []struct {
		Summary struct {
			TotalDistance int `json:"totalDistance"`
			TotalTime     int `json:"totalTime"`
		} `json:"summary"`
	} `json:"routes"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// GetRouteDuration calls Nostra routing API and returns (distanceMeters, travelDuration).
// Falls back to Haversine estimate if the API call fails.
func GetRouteDuration(from, to Location) (int, time.Duration) {
	apiKey := getNostraKey()
	if apiKey == "" {
		return haversineFallback(from, to)
	}

	apiURL := fmt.Sprintf(
		"https://api.nostramap.com/Route/v2/route?key=%s&origin=%f,%f&destination=%f,%f&mode=car",
		url.QueryEscape(apiKey), from.Lat, from.Lng, to.Lat, to.Lng,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("[WARN] Nostra Route API failed: %v — using Haversine fallback", err)
		return haversineFallback(from, to)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[WARN] Nostra Route read failed: %v — using Haversine fallback", err)
		return haversineFallback(from, to)
	}

	var apiResp nostraRouteResponse
	if err := json.Unmarshal(body, &apiResp); err != nil || len(apiResp.Routes) == 0 {
		log.Printf("[WARN] Nostra Route parse failed (body: %s) — using Haversine fallback", string(body))
		return haversineFallback(from, to)
	}

	summary := apiResp.Routes[0].Summary
	return summary.TotalDistance, time.Duration(summary.TotalTime) * time.Second
}

func haversineFallback(from, to Location) (int, time.Duration) {
	dx := from.Lng - to.Lng
	dy := from.Lat - to.Lat
	distMeters := math.Sqrt(dx*dx+dy*dy) * 111000
	speedMetersPerSecond := 40.0 * 1000 / 3600
	durationSeconds := distMeters / speedMetersPerSecond
	return int(distMeters), time.Duration(durationSeconds) * time.Second
}

// EstimateDistanceDuration kept for backward compatibility — delegates to GetRouteDuration.
func EstimateDistanceDuration(from, to Location) (int, time.Duration) {
	return GetRouteDuration(from, to)
}
