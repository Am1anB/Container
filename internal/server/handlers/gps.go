package handlers

import (
	"haulagex/internal/server/middleware"
	"haulagex/internal/services"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	lastKnownLocations = make(map[uint]services.Location)
	locationMutex      = &sync.RWMutex{}
)

type UpdateLocationReq struct {
	Lat float64 `json:"lat" binding:"required"`
	Lng float64 `json:"lng" binding:"required"`
}

func UpdateUserLocation(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	var req UpdateLocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	locationMutex.Lock()
	lastKnownLocations[uid] = services.Location{Lat: req.Lat, Lng: req.Lng}
	locationMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "location updated"})
}

func GetUserLocation(userID uint) (services.Location, bool) {
	locationMutex.RLock()
	loc, ok := lastKnownLocations[userID]
	locationMutex.RUnlock()
	return loc, ok
}
