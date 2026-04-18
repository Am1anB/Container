package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"haulagex/internal/db"
	"haulagex/internal/models"
	"haulagex/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func AdminListUsers(c *gin.Context) {
	role := strings.TrimSpace(c.Query("role")) 
	q := strings.TrimSpace(c.Query("q"))
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	var users []models.User
	tx := db.DB.Model(&models.User{})

	if role != "" {
		tx = tx.Where("role = ?", role)
	}
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("name ILIKE ? OR email ILIKE ? OR phone ILIKE ?", like, like, like)
	}

	if err := tx.Limit(limit).Order("id DESC").
		Select("id,name,phone,email,role,created_at,updated_at,license_plate,profile_image_url").
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query error"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func UploadProfilePhoto(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	uploadDir := filepath.Join(".", "uploads")
	if err := ensureDir(uploadDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create upload dir"})
		return
	}
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("user_%d_%d%s", uid, time.Now().UnixNano(), ext)
	fullPath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save file error"})
		return
	}

	photoURL := "/uploads/" + filename

	if err := db.DB.Model(&models.User{}).Where("id = ?", uid).Update("profile_image_url", photoURL).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update user photo error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"photoUrl": photoURL,
	})
}
