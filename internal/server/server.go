package server

import (
	"fmt"
	"haulagex/internal/auth"
	"haulagex/internal/models"
	"haulagex/internal/server/handlers"
	"haulagex/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Engine *gin.Engine
	Port   string
}

func New(port string, jwtSvc *auth.JWTService) *Server {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.Static("/uploads", "./uploads")
	api := r.Group("/api")
	{
		ah := handlers.NewAuthHandler(jwtSvc)
		api.POST("/auth/register", ah.Register)
		api.POST("/auth/login", ah.Login)
		api.POST("/auth/refresh", ah.Refresh)
		admin := api.Group("/admin",
			middleware.AuthRequired(jwtSvc),
			middleware.RequireRole(models.RoleAdmin),
		)
		{
			admin.GET("/dashboard", handlers.AdminDashboardStats)
			admin.POST("/jobs", handlers.AdminAssignJob)
			admin.GET("/jobs", handlers.AdminListJobs)
			admin.GET("/users", handlers.AdminListUsers)
			admin.POST("/jobs/bulk", handlers.AdminBulkCreateJobs)
			admin.POST("/jobs/distribute", handlers.AdminDistributeJobs)
			admin.POST("/jobs/assign-first", handlers.AdminAssignFirstJob)
			admin.POST("/jobs/:id/accident-reassign", handlers.AdminReassignAccidentJob)
		}

		exportGrp := api.Group("/admin/export",
				middleware.AuthQueryParam(jwtSvc),
				middleware.RequireRole(models.RoleAdmin),
			)
			{
				exportGrp.GET("/jobs", handlers.AdminExportJobs)
			}

		authRoutes := api.Group("/", middleware.AuthRequired(jwtSvc))
		{
			authRoutes.GET("/me", ah.Me)
			authRoutes.POST("/me/upload-photo", handlers.UploadProfilePhoto)
			authRoutes.POST("/gps/update", handlers.UpdateUserLocation)
			authRoutes.GET("/jobs/open", handlers.ListOpenJobs)
			authRoutes.GET("/jobs/my", handlers.ListMyJobs)
			authRoutes.POST("/jobs/:id/accept", handlers.AcceptJob)
			authRoutes.POST("/jobs/:id/start", handlers.StartJob)
			authRoutes.POST("/jobs/:id/complete", handlers.CompleteJob)
			authRoutes.GET("/jobs/:id", handlers.GetJob)
			authRoutes.POST("/jobs/:id/status", handlers.UpdateJobStatus)
			authRoutes.POST("/jobs/:id/ocr", handlers.OCRJob)
		}
	}

	return &Server{Engine: r, Port: port}
}

func (s *Server) Start() error { return s.Engine.Run(fmt.Sprintf(":%s", s.Port)) }
