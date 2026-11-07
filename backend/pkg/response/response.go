package response

import (
	"math"

	"github.com/gofiber/fiber/v2"
)

// Standard API Error Codes matching docs/api-contract.md
const (
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeResourceNotFound    = "RESOURCE_NOT_FOUND"
	CodeDuplicateResource   = "DUPLICATE_RESOURCE"
	CodeCourseHasHistory    = "COURSE_HAS_HISTORY"
	CodeStudentHasHistory   = "STUDENT_HAS_HISTORY"
	CodeActiveSessionExists = "ACTIVE_SESSION_EXISTS"
	CodeSessionAlreadyEnded = "SESSION_ALREADY_ENDED"
	CodePickAlreadyCancel   = "PICK_ALREADY_CANCELLED"
	CodeStudentNotEnrolled  = "STUDENT_NOT_ENROLLED"
	CodeNoEligibleStudents  = "NO_ELIGIBLE_STUDENTS"
	CodeRoundCompleted      = "ROUND_COMPLETED"
	CodeInternalServerError = "INTERNAL_SERVER_ERROR"
	CodeDatabaseUnavailable = "DATABASE_UNAVAILABLE"
)

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
}

type SuccessEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    interface{}     `json:"data"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}

func NewPaginationMeta(page, limit int, totalItems int64) *PaginationMeta {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	totalPages := 0
	if totalItems > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(limit)))
	}
	return &PaginationMeta{
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

func OK(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(SuccessEnvelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Created(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(SuccessEnvelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Paginated(c *fiber.Ctx, message string, data interface{}, page, limit int, totalItems int64) error {
	return c.Status(fiber.StatusOK).JSON(SuccessEnvelope{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    NewPaginationMeta(page, limit, totalItems),
	})
}

func Error(c *fiber.Ctx, status int, code, message string, details map[string]string) error {
	return c.Status(status).JSON(ErrorEnvelope{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func InvalidRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, CodeInvalidRequest, message, nil)
}

func ValidationError(c *fiber.Ctx, message string, details map[string]string) error {
	return Error(c, fiber.StatusBadRequest, CodeValidationError, message, details)
}

func NotFound(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusNotFound, CodeResourceNotFound, message, nil)
}

func Conflict(c *fiber.Ctx, code, message string) error {
	return Error(c, fiber.StatusConflict, code, message, nil)
}

func Unprocessable(c *fiber.Ctx, code, message string) error {
	return Error(c, fiber.StatusUnprocessableEntity, code, message, nil)
}

func InternalError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "เกิดข้อผิดพลาดภายในเซิร์ฟเวอร์"
	}
	return Error(c, fiber.StatusInternalServerError, CodeInternalServerError, message, nil)
}

