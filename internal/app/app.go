package app

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"skilljudge/backend/internal/bootstrap"
	"skilljudge/backend/internal/config"
	"skilljudge/backend/internal/http/middleware"
	"skilljudge/backend/internal/modules/ai"
	"skilljudge/backend/internal/modules/auth"
	"skilljudge/backend/internal/modules/project"
	"skilljudge/backend/internal/modules/scoring"
	"skilljudge/backend/internal/modules/system"
	"skilljudge/backend/internal/modules/task"
	"skilljudge/backend/internal/modules/taskscorer"
	"skilljudge/backend/internal/modules/user"
	"skilljudge/backend/internal/modules/video"
	"skilljudge/backend/internal/platform/cache"
	"skilljudge/backend/internal/platform/database"
	platformjwt "skilljudge/backend/internal/platform/jwt"
	platformmail "skilljudge/backend/internal/platform/mail"
	"skilljudge/backend/internal/platform/storage"

	"github.com/gin-gonic/gin"
)

type App struct {
	engine      *gin.Engine
	host        string
	port        string
	aiService   *ai.Service
	taskService *task.Service
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := database.NewPostgres(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("application startup aborted: %w", err)
	}

	redisClient, err := cache.NewRedis(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("application startup aborted: %w", err)
	}

	// Production-like environments should not mutate seeded RBAC data on every
	// startup. Bootstrap is opt-in through BOOTSTRAP_ENABLED.
	if cfg.Bootstrap.Enabled {
		if err := bootstrap.Run(routerContext(), db, cfg.Bootstrap); err != nil {
			return nil, fmt.Errorf("application startup aborted: bootstrap failed: %w", err)
		}
	}

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)
	projectRepo := project.NewRepository(db)
	projectService := project.NewService(projectRepo)
	taskRepo := task.NewRepository(db)
	storageProvider := storage.NewProvider(cfg.Storage)
	mailSender, err := platformmail.NewSender(cfg.Mail)
	if err != nil {
		return nil, fmt.Errorf("application startup aborted: mail init failed: %w", err)
	}
	taskService := task.NewService(taskRepo, projectRepo, storageProvider)
	aiRepo := ai.NewRepository(db)
	aiService := ai.NewService(aiRepo, taskService, storageProvider, cfg.AI)
	scoringRepo := scoring.NewRepository(db)
	taskScorerRepo := taskscorer.NewRepository(db)
	taskScorerService := taskscorer.NewService(taskScorerRepo, taskService, userRepo, mailSender, cfg.Mail.InviteBaseURL)
	scoringService := scoring.NewService(scoringRepo, aiService, taskService, storageProvider, taskScorerService)
	videoRepo := video.NewRepository(db)
	videoService := video.NewService(videoRepo, aiService, projectRepo, taskService, userRepo, storageProvider)

	jwtManager := platformjwt.NewManager(cfg.JWT.Issuer, cfg.JWT.Secret)
	sessionStore := auth.NewSessionStore(redisClient)
	authService := auth.NewService(userRepo, userService, sessionStore, jwtManager, cfg.JWT)

	authHandler := auth.NewHandler(authService, userService)
	aiHandler := ai.NewHandler(aiService)
	projectHandler := project.NewHandler(projectService)
	scoringHandler := scoring.NewHandler(scoringService)
	systemHandler := system.NewHandler(db, redisClient)
	taskHandler := task.NewHandler(taskService)
	taskScorerHandler := taskscorer.NewHandler(taskScorerService)
	userHandler := user.NewHandler(userService)
	videoHandler := video.NewHandler(videoService)

	router := gin.Default()
	router.Use(corsMiddleware())
	// Route registration stays centralized here so module wiring and
	// permission boundaries can be audited from a single entry point.
	registerRoutes(router, authService, userService, authHandler, aiHandler, systemHandler, userHandler, projectHandler, taskHandler, taskScorerHandler, scoringHandler, videoHandler)

	return &App{
		engine:      router,
		host:        cfg.HTTP.Host,
		port:        cfg.HTTP.Port,
		aiService:   aiService,
		taskService: taskService,
	}, nil
}

func routerContext() context.Context {
	return context.Background()
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := map[string]struct{}{
		"http://localhost:8081": {},
		"http://127.0.0.1:8081": {},
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (a *App) Run() error {
	if a.aiService != nil {
		log.Printf("ai poller starting")
		go a.aiService.RunPoller(context.Background())
	}
	if a.taskService != nil {
		log.Printf("task analysis report worker starting")
		go a.taskService.RunReportWorker(context.Background())
	}
	addr := fmt.Sprintf(":%s", a.port)
	if a.host != "" {
		addr = fmt.Sprintf("%s:%s", a.host, a.port)
	}
	return a.engine.Run(addr)
}

func registerRoutes(router *gin.Engine, authService *auth.Service, userService *user.Service, authHandler *auth.Handler, aiHandler *ai.Handler, systemHandler *system.Handler, userHandler *user.Handler, projectHandler *project.Handler, taskHandler *task.Handler, taskScorerHandler *taskscorer.Handler, scoringHandler *scoring.Handler, videoHandler *video.Handler) {
	router.GET("/health", systemHandler.Health)
	router.GET("/ready", systemHandler.Ready)

	api := router.Group("/api/v1")
	router.GET("/task-scorer-invitations/:token/accept", taskScorerHandler.AcceptPage)

	authGroup := api.Group("/auth")
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", middleware.RequireAuth(authService), authHandler.Logout)
	api.POST("/task-scorer-invitations/:token/accept", taskScorerHandler.Accept)
	api.GET("/task-scorer-invitations/my", middleware.RequireAuth(authService), middleware.RequirePermission(userService, "task:read"), taskScorerHandler.ListMine)
	api.PATCH("/task-scorer-invitations/my/:id/read", middleware.RequireAuth(authService), middleware.RequirePermission(userService, "task:read"), taskScorerHandler.MarkMineRead)
	api.POST("/task-scorer-invitations/my/:id/accept", middleware.RequireAuth(authService), middleware.RequirePermission(userService, "task:read"), taskScorerHandler.AcceptMine)
	api.POST("/scorers/provision", middleware.RequireAuth(authService), middleware.RequirePermission(userService, "task:update"), taskScorerHandler.Provision)

	userGroup := api.Group("/users", middleware.RequireAuth(authService))
	userGroup.GET("/me", userHandler.GetMe)
	userGroup.PATCH("/me", userHandler.UpdateMe)
	userGroup.POST("", middleware.RequirePermission(userService, "user:create"), userHandler.Create)
	userGroup.POST("/batch", middleware.RequirePermission(userService, "user:create"), userHandler.BatchCreate)
	userGroup.GET("", middleware.RequirePermission(userService, "user:read"), userHandler.List)
	userGroup.PATCH("/:id", middleware.RequirePermission(userService, "user:update"), userHandler.Update)
	userGroup.DELETE("/:id", middleware.RequirePermission(userService, "user:delete"), userHandler.Delete)

	projectGroup := api.Group("/projects", middleware.RequireAuth(authService))
	projectGroup.POST("", middleware.RequirePermission(userService, "project:create"), projectHandler.Create)
	projectGroup.GET("", middleware.RequirePermission(userService, "project:read"), projectHandler.List)
	projectGroup.POST("/:projectId/tasks", middleware.RequirePermission(userService, "task:create"), taskHandler.Create)
	projectGroup.GET("/:projectId/tasks", middleware.RequirePermission(userService, "task:read"), taskHandler.ListByProject)
	projectGroup.GET("/:projectId", middleware.RequirePermission(userService, "project:read"), projectHandler.Get)
	projectGroup.PATCH("/:projectId", middleware.RequirePermission(userService, "project:update"), projectHandler.Update)
	projectGroup.DELETE("/:projectId", middleware.RequirePermission(userService, "project:delete"), projectHandler.Delete)

	taskGroup := api.Group("/tasks", middleware.RequireAuth(authService))
	taskGroup.GET("/my", middleware.RequirePermission(userService, "task:read"), scoringHandler.ListMyTasks)
	taskGroup.GET("/assignable-scorers", middleware.RequirePermission(userService, "task:update"), scoringHandler.ListAssignableScorers)
	taskGroup.GET("/:id/scorers", middleware.RequirePermission(userService, "task:update"), taskScorerHandler.List)
	taskGroup.POST("/:id/scorers/:scorerId/notify", middleware.RequirePermission(userService, "task:update"), taskScorerHandler.Notify)
	taskGroup.POST("/:id/scorers/invite", middleware.RequirePermission(userService, "task:update"), taskScorerHandler.Invite)
	taskGroup.POST("/:id/scorers/:scorerId/resend-invite", middleware.RequirePermission(userService, "task:update"), taskScorerHandler.Resend)
	taskGroup.DELETE("/:id/scorers/:scorerId", middleware.RequirePermission(userService, "task:update"), taskScorerHandler.Remove)
	taskGroup.GET("/:id/scoreboard", middleware.RequirePermission(userService, "task:read"), taskHandler.GetScoreboard)
	taskGroup.GET("/:id/pending-assignments", middleware.RequirePermission(userService, "task:read"), scoringHandler.ListPendingAssignments)
	taskGroup.GET("/:id/analysis", middleware.RequirePermission(userService, "task:read"), taskHandler.GetAnalysis)
	taskGroup.POST("/:id/analysis-report", middleware.RequirePermission(userService, "task:update"), taskHandler.GenerateAnalysisReport)
	taskGroup.GET("/:id/analysis-report", middleware.RequirePermission(userService, "task:read"), taskHandler.GetAnalysisReport)
	taskGroup.GET("/:id", middleware.RequirePermission(userService, "task:read"), func(c *gin.Context) {
		actor := middleware.CurrentUser(c)
		// Phase 1 keeps the API path in api.md, but scorers use this route as a
		// "scoring task" view backed by video assignment data instead of batch detail.
		if actor.Role == "scorer" {
			scoringHandler.GetTaskDetail(c)
			return
		}
		taskHandler.Get(c)
	})
	taskGroup.POST("/:id/save-draft", middleware.RequirePermission(userService, "task:submit"), scoringHandler.SaveTaskDraft)
	taskGroup.POST("/:id/submit-saved", middleware.RequirePermission(userService, "task:submit"), scoringHandler.SubmitSavedTask)
	taskGroup.POST("/:id/submit", middleware.RequirePermission(userService, "task:submit"), scoringHandler.SubmitTask)
	taskGroup.POST("/:id/assignments", middleware.RequirePermission(userService, "task:update"), scoringHandler.AssignScorers)
	taskGroup.POST("/:id/reassign-pending", middleware.RequirePermission(userService, "task:update"), scoringHandler.ReassignPendingVideos)
	taskGroup.POST("/:id/ai-evaluations", middleware.RequirePermission(userService, "task:update"), aiHandler.BatchCreateForTask)

	rubricGroup := api.Group("/rubrics", middleware.RequireAuth(authService))
	rubricGroup.POST("", middleware.RequirePermission(userService, "rubric:create"), projectHandler.CreateRubric)
	rubricGroup.POST("/upload-template", middleware.RequirePermission(userService, "rubric:create"), projectHandler.UploadRubricTemplate)
	rubricGroup.GET("", middleware.RequirePermission(userService, "rubric:read"), projectHandler.ListRubrics)
	rubricGroup.GET("/:id", middleware.RequirePermission(userService, "rubric:read"), projectHandler.GetRubric)
	rubricGroup.PATCH("/:id", middleware.RequirePermission(userService, "rubric:update"), projectHandler.UpdateRubric)
	rubricGroup.DELETE("/:id", middleware.RequirePermission(userService, "rubric:delete"), projectHandler.DeleteRubric)

	videoGroup := api.Group("/videos", middleware.RequireAuth(authService))
	videoGroup.POST("/upload-credential", middleware.RequirePermission(userService, "video:create"), videoHandler.CreateUploadCredential)
	videoGroup.POST("/:id/confirm-upload", middleware.RequirePermission(userService, "video:create"), videoHandler.ConfirmUpload)
	videoGroup.POST("/:id/ai-evaluations", middleware.RequirePermission(userService, "task:update"), aiHandler.CreateForVideo)
	videoGroup.GET("", middleware.RequirePermission(userService, "video:read"), videoHandler.List)
	videoGroup.GET("/:id", middleware.RequirePermission(userService, "video:read"), videoHandler.Get)
	videoGroup.POST("/:id/ai-report", middleware.RequirePermission(userService, "task:update"), videoHandler.GenerateAIReport)
	videoGroup.GET("/:id/ai-report", middleware.RequirePermission(userService, "video:read"), videoHandler.GetAIReport)
	videoGroup.GET("/:id/ai-report/preview", middleware.RequirePermission(userService, "video:read"), videoHandler.PreviewAIReport)
	videoGroup.GET("/:id/ai-report/download", middleware.RequirePermission(userService, "video:read"), videoHandler.DownloadAIReport)
	videoGroup.DELETE("/:id", middleware.RequirePermission(userService, "video:delete"), videoHandler.Delete)

	studentGroup := api.Group("/students", middleware.RequireAuth(authService))
	studentGroup.GET("/me/videos", videoHandler.ListMine)
	studentGroup.GET("/me/videos/:id", videoHandler.GetMine)

	aiGroup := api.Group("/ai-evaluations", middleware.RequireAuth(authService))
	aiGroup.GET("/:id", middleware.RequirePermission(userService, "task:read"), aiHandler.Get)
	aiGroup.GET("/:id/result", middleware.RequirePermission(userService, "task:read"), aiHandler.GetResult)

}
