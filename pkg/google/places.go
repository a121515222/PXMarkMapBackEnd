package google

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// PlaceSearchResponse 回傳結構
type PlaceSearchResponse struct {
	Places []struct {
		ID               string `json:"id"`
		FormattedAddress string `json:"formattedAddress"`
		DisplayName      struct {
			Text string `json:"text"`
		} `json:"displayName"`
		Location struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"location"`
	} `json:"places"`
}

// SearchPlaceByName 查詢店名
func SearchPlaceByName(storeName string) (*PlaceSearchResponse, error) {
	apiKey := os.Getenv("GOOGLE_PLACES_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_PLACES_API_KEY not set")
	}

	endpoint := "https://places.googleapis.com/v1/places:searchText"

	bodyMap := map[string]string{"textQuery": storeName}
	bodyJSON, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", apiKey)
	req.Header.Set("X-Goog-FieldMask", "places.displayName,places.id,places.formattedAddress,places.location")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result PlaceSearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if len(result.Places) == 0 {
		return nil, fmt.Errorf("no places found for %s", storeName)
	}

	return &result, nil
}

// EnrichStoresWithPlaceData 為所有店家加上地點資訊
// func EnrichStoresWithPlaceData(storeMap map[string]*StoreData) error {
// 	for storeName, storeData := range storeMap {
// 		// 組合搜尋關鍵字：全聯 + 店名
// 		searchQuery := fmt.Sprintf("全聯 %s", storeName)
// 		log.Printf("搜尋店家: %s", searchQuery)

// 		placeRes, err := SearchPlaceByName(searchQuery)
// 		if err != nil {
// 			log.Printf("⚠ 無法找到 %s 的地點資訊: %v", searchQuery, err)
// 			continue
// 		}

// 		if len(placeRes.Places) > 0 {
// 			place := placeRes.Places[0]
// 			storeData.PlaceID = place.ID
// 			storeData.FormattedAddress = place.FormattedAddress
// 			storeData.Latitude = place.Location.Latitude
// 			storeData.Longitude = place.Location.Longitude

// 			log.Printf("✓ 找到 %s: %s (%.6f, %.6f)",
// 				storeName,
// 				place.FormattedAddress,
// 				place.Location.Latitude,
// 				place.Location.Longitude,
// 			)
// 		}
// 	}

// 	return nil
// }
func EnrichStoresWithPlaceData(storeMap map[string]*StoreData) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // 同時最多 10 個查詢

	for storeName, storeData := range storeMap {
		wg.Add(1)
		go func(name string, data *StoreData) {
			defer wg.Done()
			sem <- struct{}{} // 進入工作池
			defer func() { <-sem }()

			searchQuery := "全聯 " + name
			log.Printf("搜尋店家: %s", searchQuery)

			placeRes, err := SearchPlaceByName(searchQuery)
			if err != nil {
				log.Printf("⚠ 無法找到 %s 的地點資訊: %v", searchQuery, err)
				return
			}

			if len(placeRes.Places) > 0 {
				place := placeRes.Places[0]
				data.PlaceID = place.ID
				data.FormattedAddress = place.FormattedAddress
				data.Latitude = place.Location.Latitude
				data.Longitude = place.Location.Longitude

				log.Printf("✓ 找到 %s: %s (%.6f, %.6f)",
					name, place.FormattedAddress,
					place.Location.Latitude, place.Location.Longitude)
			}

			// 為避免 API 配額過快消耗，可加一點點間隔
			time.Sleep(150 * time.Millisecond)
		}(storeName, storeData)
	}

	wg.Wait()
	log.Println("[INFO] 所有店家地點查詢完成")
	return nil
}