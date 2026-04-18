package handlers

import (
	"net/http"
	"strings"
	"haulagex/internal/auth"
	"haulagex/internal/db"
	"haulagex/internal/models"
	"haulagex/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	JWT *auth.JWTService
}

func NewAuthHandler(jwt *auth.JWTService) *AuthHandler {
	return &AuthHandler{JWT: jwt}
}

type registerReq struct {
	Name         string `json:"name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=6"`
	LicensePlate string `json:"licensePlate"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var cnt int64
	db.DB.Model(&models.User{}).Where("email = ? OR phone = ?", req.Email, req.Phone).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "email or phone already in use"})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := models.User{
		Name:         req.Name,
		Phone:        req.Phone,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         models.RoleUser, 
		LicensePlate: req.LicensePlate,
	}
	if err := db.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create user"})
		return
	}
	access, _ := h.JWT.GenerateAccessToken(u.ID, u.Role)
	refresh, _ := h.JWT.GenerateRefreshToken(u.ID, u.Role)
	c.JSON(http.StatusCreated, gin.H{
		"user":         gin.H{"id": u.ID, "name": u.Name, "email": u.Email, "phone": u.Phone, "role": u.Role, "licensePlate": u.LicensePlate, "profileImageURL": u.ProfileImageURL},
		"accessToken":  access,
		"refreshToken": refresh,
	})
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var u models.User
	if err := db.DB.Where("email = ?", req.Email).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	access, _ := h.JWT.GenerateAccessToken(u.ID, u.Role)
	refresh, _ := h.JWT.GenerateRefreshToken(u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{
		"user":         gin.H{"id": u.ID, "name": u.Name, "email": u.Email, "phone": u.Phone, "role": u.Role, "licensePlate": u.LicensePlate, 
		"profileImageURL": u.ProfileImageURL},
		"accessToken":  access,
		"refreshToken": refresh,
	})
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims, err := h.JWT.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	access, _ := h.JWT.GenerateAccessToken(claims.UserID, claims.Role)
	c.JSON(http.StatusOK, gin.H{"accessToken": access})
}

func (h *AuthHandler) Me(c *gin.Context) {
	uidAny, _ := c.Get(middleware.CtxUserID)
	var u models.User
	if err := db.DB.First(&u, uidAny.(uint)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": u.ID, "name": u.Name, "email": u.Email, "phone": u.Phone, "role": u.Role, "licensePlate": u.LicensePlate, "profileImageURL": u.ProfileImageURL,
	})
}
