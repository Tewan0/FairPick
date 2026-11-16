package routes

import (
	"strings"

	"github.com/Tewan0/FairPick/config"
	"github.com/Tewan0/FairPick/database"
	"github.com/Tewan0/FairPick/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"gorm.io/gorm"
)

func NewCORSConfig(cfg *config.Config) cors.Config {
	allowOrigins := "http://localhost:3000"
	if cfg != nil && strings.TrimSpace(cfg.CORSOrigins) != "" {
		allowOrigins = strings.TrimSpace(cfg.CORSOrigins)
	}
	return cors.Config{
		AllowOrigins: allowOrigins,
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
		MaxAge:       86400,
	}
}

func Setup(app *fiber.App, cfg *config.Config) {
	SetupWithDB(app, cfg, database.DB)
}

func SetupWithDB(app *fiber.App, cfg *config.Config, db *gorm.DB) {
	// Set standard HTTP security headers per GO-HTTP-004
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	})
	app.Use(cors.New(NewCORSConfig(cfg)))
	api := app.Group("/api")

	// 0. Health Check
	healthHandler := handlers.NewHealthHandler(cfg)
	api.Get("/health/db", healthHandler.CheckDB)

	if db == nil {
		return
	}

	// 1. Courses CRUD
	courseHandler := handlers.NewCourseHandler(db)
	api.Get("/courses", courseHandler.ListCourses)
	api.Post("/courses", courseHandler.CreateCourse)
	api.Get("/courses/:courseId", courseHandler.GetCourse)
	api.Put("/courses/:courseId", courseHandler.UpdateCourse)
	api.Delete("/courses/:courseId", courseHandler.DeleteCourse)

	// 2. Students CRUD
	studentHandler := handlers.NewStudentHandler(db)
	api.Get("/students", studentHandler.ListStudents)
	api.Post("/students", studentHandler.CreateStudent)
	api.Get("/students/:studentId", studentHandler.GetStudent)
	api.Put("/students/:studentId", studentHandler.UpdateStudent)
	api.Delete("/students/:studentId", studentHandler.DeleteStudent)

	// 3 & 4. Bulk Import & Course Enrollments
	enrollmentHandler := handlers.NewEnrollmentHandler(db)
	api.Post("/courses/:courseId/students/import-file", enrollmentHandler.ImportStudentsFile)
	api.Post("/courses/:courseId/students/bulk", enrollmentHandler.BulkEnrollStudents)
	api.Get("/courses/:courseId/students", enrollmentHandler.ListCourseStudents)
	api.Post("/courses/:courseId/enrollments", enrollmentHandler.EnrollStudent)
	api.Delete("/courses/:courseId/enrollments/:studentId", enrollmentHandler.WithdrawStudent)

	// 5. Class Sessions
	sessionHandler := handlers.NewSessionHandler(db)
	api.Get("/courses/:courseId/sessions", sessionHandler.ListCourseSessions)
	api.Post("/courses/:courseId/sessions", sessionHandler.StartSession)
	api.Get("/sessions/:sessionId", sessionHandler.GetSession)
	api.Patch("/sessions/:sessionId/end", sessionHandler.EndSession)
	api.Patch("/sessions/:sessionId/mode", sessionHandler.SwitchMode)

	// 6. Attendance
	attendanceHandler := handlers.NewAttendanceHandler(db)
	api.Get("/sessions/:sessionId/attendance", attendanceHandler.GetSessionAttendance)
	api.Patch("/sessions/:sessionId/attendance/:studentId", attendanceHandler.UpdateStudentAttendance)
	api.Patch("/sessions/:sessionId/attendance", attendanceHandler.BulkUpdateAttendance)

	// 7, 8 & 9. Random Pick, Cancel Pick & Pick History
	pickHandler := handlers.NewPickHandler(db)
	api.Get("/sessions/:sessionId/picks/pool", pickHandler.GetPickPool)
	api.Post("/sessions/:sessionId/picks", pickHandler.ExecutePick)
	api.Post("/sessions/:sessionId/picks/new-round", pickHandler.StartNewRound)
	api.Patch("/picks/:pickId/cancel", pickHandler.CancelPick)
	api.Get("/sessions/:sessionId/picks", pickHandler.ListSessionPicks)
	api.Get("/courses/:courseId/picks", pickHandler.ListCoursePicks)

	// 10. Statistics
	statsHandler := handlers.NewStatsHandler(db)
	api.Get("/courses/:courseId/stats/overview", statsHandler.GetCourseOverviewStats)
	api.Get("/courses/:courseId/stats/students", statsHandler.ListCourseStudentStats)
	api.Get("/courses/:courseId/stats/students/:studentId", statsHandler.GetStudentCourseStats)
}
