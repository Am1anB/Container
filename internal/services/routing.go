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
	} else {
		log.Printf("[DEBUG] NOSTRA_API_KEY prefix: %s...", key[:10])
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
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		return Location{}, fmt.Errorf("GOOGLE_MAPS_API_KEY is not set")
	}

	params := url.Values{}
	params.Set("address", address)
	params.Set("key", apiKey)
	apiURL := "https://maps.googleapis.com/maps/api/geocode/json?" + params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return Location{}, fmt.Errorf("geocode request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Location{}, fmt.Errorf("failed to read geocode response: %v", err)
	}

	var result struct {
		Status  string `json:"status"`
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return Location{}, fmt.Errorf("failed to parse geocode response: %v", err)
	}

	if result.Status != "OK" || len(result.Results) == 0 {
		return Location{}, fmt.Errorf("geocode failed: %s", result.Status)
	}

	loc := result.Results[0].Geometry.Location
	log.Printf("[INFO] Geocoded '%s' -> Lat: %f, Lng: %f", address, loc.Lat, loc.Lng)
	return Location{Lat: loc.Lat, Lng: loc.Lng}, nil
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
