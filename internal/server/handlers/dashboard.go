package handlers

import (
	"net/http"
	"haulagex/internal/db"
	"haulagex/internal/models"
	"github.com/gin-gonic/gin"
)

type DashboardStats struct {
	TotalJobs    int64            `json:"totalJobs"`
	JobsByStatus map[string]int64 `json:"jobsByStatus"`
	TotalUsers   int64            `json:"totalUsers"`
	ActiveUsers  int64            `json:"activeUsers"`
}

func AdminDashboardStats(c *gin.Context) {
	var stats DashboardStats
	stats.JobsByStatus = make(map[string]int64)
	db.DB.Model(&models.Job{}).Count(&stats.TotalJobs)
	var statusCounts []struct {
		Status string
		Count  int64
	}
	db.DB.Model(&models.Job{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts)
	for _, sc := range statusCounts {
		stats.JobsByStatus[sc.Status] = sc.Count
	}
	db.DB.Model(&models.User{}).Where("role = ?", models.RoleUser).Count(&stats.TotalUsers)
	db.DB.Model(&models.Job{}).Where("status = ?", models.StatusInProgress).Distinct("assignee_id").Count(&stats.ActiveUsers)
	c.JSON(http.StatusOK, stats)
}
