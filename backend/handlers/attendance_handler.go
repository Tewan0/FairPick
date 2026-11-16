package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/response"
	"github.com/Tewan0/FairPick/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AttendanceHandler struct {
	db *gorm.DB
}

func NewAttendanceHandler(db *gorm.DB) *AttendanceHandler {
	return &AttendanceHandler{db: db}
}

type AttendanceRecordDTO struct {
	AttendanceID      uint64     `json:"attendanceId"`
	StudentID         uint64     `json:"studentId"`
	StudentCode       string     `json:"studentCode"`
	FullName          string     `json:"fullName"`
	Status            string     `json:"status"`
	IsEligibleForPick bool       `json:"isEligibleForPick"`
	CheckedAt         *time.Time `json:"checkedAt"`
	Note              *string    `json:"note"`
}

type SessionAttendanceResponseDTO struct {
	SessionID             uint64                `json:"sessionId"`
	CourseID              uint64                `json:"courseId"`
	SessionStatus         string                `json:"sessionStatus"`
	HasUnconfirmedWarning bool                  `json:"hasUnconfirmedWarning"`
	Summary               AttendanceSummaryDTO  `json:"summary"`
	Records               []AttendanceRecordDTO `json:"records"`
}

type UpdateAttendanceRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note"`
}

type SingleAttendanceUpdateResponseDTO struct {
	AttendanceID      uint64     `json:"attendanceId"`
	SessionID         uint64     `json:"sessionId"`
	CourseID          uint64     `json:"courseId"`
	StudentID         uint64     `json:"studentId"`
	StudentCode       string     `json:"studentCode"`
	FullName          string     `json:"fullName"`
	Status            string     `json:"status"`
	IsEligibleForPick bool       `json:"isEligibleForPick"`
	CheckedAt         *time.Time `json:"checkedAt"`
	Note              *string    `json:"note"`
}

type BulkAttendanceItem struct {
	StudentID uint64  `json:"studentId"`
	Status    string  `json:"status"`
	Note      *string `json:"note"`
}

type BulkUpdateAttendanceRequest struct {
	MarkAllStatus *string              `json:"markAllStatus"`
	Items         []BulkAttendanceItem `json:"items"`
}

type BulkUpdateAttendanceResponseDTO struct {
	SessionID    uint64               `json:"sessionId"`
	UpdatedCount int                  `json:"updatedCount"`
	Summary      AttendanceSummaryDTO `json:"summary"`
}

var errStudentNotEnrolled = errors.New("student not enrolled")

func isValidAttendanceStatus(status string) bool {
	switch status {
	case models.AttendanceStatusPresent,
		models.AttendanceStatusAbsent,
		models.AttendanceStatusLate,
		models.AttendanceStatusUnconfirmed:
		return true
	default:
		return false
	}
}

// GetSessionAttendance handles GET /api/sessions/:sessionId/attendance
func (h *AttendanceHandler) GetSessionAttendance(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	statusFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("status")))
	if statusFilter != "" && !isValidAttendanceStatus(statusFilter) {
		return response.ValidationError(c, "ตัวกรองสถานะการเข้าเรียนไม่ถูกต้อง", map[string]string{
			"status": "status ต้องเป็น present, absent, late หรือ unconfirmed เท่านั้น",
		})
	}

	search := validator.NormalizeRequiredString(c.Query("search"))

	var session models.ClassSession
	if err := h.db.First(&session, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลคาบเรียนได้")
	}

	summary, err := ComputeAttendanceSummary(h.db, session.ID, session.CourseID)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถคำนวณสรุปข้อมูลการเข้าเรียนได้")
	}

	type joinedAttendanceRow struct {
		AttendanceID     uint64
		StudentID        uint64
		StudentCode      string
		FullName         string
		Status           string
		EnrollmentStatus string
		CheckedAt        *time.Time
		Note             *string
	}

	query := h.db.Table("attendance").
		Select(`attendance.id AS attendance_id,
			attendance.student_id,
			students.student_code,
			students.full_name,
			attendance.status,
			COALESCE(enrollments.status, '') AS enrollment_status,
			attendance.checked_at,
			attendance.note`).
		Joins("JOIN students ON students.id = attendance.student_id").
		Joins("LEFT JOIN enrollments ON enrollments.course_id = attendance.course_id AND enrollments.student_id = attendance.student_id").
		Where("attendance.session_id = ?", session.ID)

	if statusFilter != "" {
		query = query.Where("attendance.status = ?", statusFilter)
	}

	if search != "" {
		like := validator.FormatSearchLike(search)
		query = query.Where("(students.student_code ILIKE ? OR students.full_name ILIKE ?)", like, like)
	}

	var rows []joinedAttendanceRow
	if err := query.Order("students.student_code ASC").Scan(&rows).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายชื่อการเข้าเรียนได้")
	}

	records := make([]AttendanceRecordDTO, 0, len(rows))
	for _, r := range rows {
		eligible := (r.Status == models.AttendanceStatusPresent || r.Status == models.AttendanceStatusLate) &&
			r.EnrollmentStatus == models.EnrollmentStatusActive
		records = append(records, AttendanceRecordDTO{
			AttendanceID:      r.AttendanceID,
			StudentID:         r.StudentID,
			StudentCode:       r.StudentCode,
			FullName:          r.FullName,
			Status:            r.Status,
			IsEligibleForPick: eligible,
			CheckedAt:         r.CheckedAt,
			Note:              r.Note,
		})
	}

	return response.OK(c, "ดึงข้อมูลการเข้าเรียนสำเร็จ", SessionAttendanceResponseDTO{
		SessionID:             session.ID,
		CourseID:              session.CourseID,
		SessionStatus:         session.Status,
		HasUnconfirmedWarning: summary.Unconfirmed > 0,
		Summary:               summary,
		Records:               records,
	})
}

// UpdateStudentAttendance handles PATCH /api/sessions/:sessionId/attendance/:studentId
func (h *AttendanceHandler) UpdateStudentAttendance(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var req UpdateAttendanceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	status := strings.ToLower(validator.NormalizeRequiredString(req.Status))
	note := validator.NormalizeOptionalString(req.Note)

	details := make(map[string]string)
	if !isValidAttendanceStatus(status) {
		details["status"] = "status ต้องเป็น present, absent, late หรือ unconfirmed เท่านั้น"
	}
	if note != nil && len([]rune(*note)) > 255 {
		details["note"] = "หมายเหตุต้องมีความยาวไม่เกิน 255 ตัวอักษร"
	}
	if len(details) > 0 {
		return response.ValidationError(c, "ข้อมูลการเช็กชื่อไม่ถูกต้อง", details)
	}

	var session models.ClassSession
	var student models.Student
	var att models.Attendance

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSessionNotFound
			}
			return err
		}

		if session.Status == models.SessionStatusEnded {
			return errSessionAlreadyEnded
		}

		var enrollment models.Enrollment
		if err := tx.Where("course_id = ? AND student_id = ?", session.CourseID, studentID).First(&enrollment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errStudentNotEnrolled
			}
			return err
		}
		if enrollment.Status != models.EnrollmentStatusActive {
			return errStudentNotEnrolled
		}

		if err := tx.First(&student, studentID).Error; err != nil {
			return err
		}

		now := time.Now()
		var checkedAt *time.Time
		if status != models.AttendanceStatusUnconfirmed {
			checkedAt = &now
		}

		err := tx.Where("session_id = ? AND student_id = ?", session.ID, studentID).First(&att).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			att = models.Attendance{
				SessionID: session.ID,
				CourseID:  session.CourseID,
				StudentID: studentID,
				Status:    status,
				CheckedAt: checkedAt,
				Note:      note,
			}
			if err := tx.Create(&att).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			updates := map[string]interface{}{
				"status":     status,
				"checked_at": checkedAt,
				"updated_at": now,
			}
			if req.Note != nil {
				updates["note"] = note
			}
			if err := tx.Model(&att).Updates(updates).Error; err != nil {
				return err
			}
			att.Status = status
			att.CheckedAt = checkedAt
			if req.Note != nil {
				att.Note = note
			}
		}

		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถแก้ไขสถานะการเข้าเรียนในคาบเรียนที่จบไปแล้วได้")
		case errors.Is(txErr, errStudentNotEnrolled):
			return response.Unprocessable(c, response.CodeStudentNotEnrolled, "นักศึกษาไม่ได้ลงทะเบียนเรียนในรายวิชาของคาบเรียนนี้ หรือถอนรายวิชาไปแล้ว")
		default:
			return response.InternalError(c, "ไม่สามารถอัปเดตสถานะการเข้าเรียนได้")
		}
	}

	eligible := att.Status == models.AttendanceStatusPresent || att.Status == models.AttendanceStatusLate

	return response.OK(c, "อัปเดตสถานะการเข้าเรียนสำเร็จ", SingleAttendanceUpdateResponseDTO{
		AttendanceID:      att.ID,
		SessionID:         session.ID,
		CourseID:          session.CourseID,
		StudentID:         student.ID,
		StudentCode:       student.StudentCode,
		FullName:          student.FullName,
		Status:            att.Status,
		IsEligibleForPick: eligible,
		CheckedAt:         att.CheckedAt,
		Note:              att.Note,
	})
}

// BulkUpdateAttendance handles PATCH /api/sessions/:sessionId/attendance
func (h *AttendanceHandler) BulkUpdateAttendance(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var req BulkUpdateAttendanceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	hasMarkAll := req.MarkAllStatus != nil && validator.NormalizeRequiredString(*req.MarkAllStatus) != ""
	hasItems := len(req.Items) > 0

	if !hasMarkAll && !hasItems {
		return response.ValidationError(c, "กรุณาระบุ markAllStatus หรือรายการ items อย่างน้อย 1 รายการ", map[string]string{
			"items": "ต้องระบุ markAllStatus หรือ items อย่างน้อย 1 รายการ",
		})
	}

	var markAllStatus string
	if hasMarkAll {
		markAllStatus = strings.ToLower(validator.NormalizeRequiredString(*req.MarkAllStatus))
		if !isValidAttendanceStatus(markAllStatus) {
			return response.ValidationError(c, "ข้อมูลสถานะการเข้าเรียนไม่ถูกต้อง", map[string]string{
				"markAllStatus": "markAllStatus ต้องเป็น present, absent, late หรือ unconfirmed เท่านั้น",
			})
		}
	}

	type normalizedBulkItem struct {
		StudentID uint64
		Status    string
		HasNote   bool
		Note      *string
	}
	var normalizedItems []normalizedBulkItem

	if hasItems {
		details := make(map[string]string)
		normalizedItems = make([]normalizedBulkItem, 0, len(req.Items))
		for i, item := range req.Items {
			st := strings.ToLower(validator.NormalizeRequiredString(item.Status))
			note := validator.NormalizeOptionalString(item.Note)
			if item.StudentID == 0 {
				details[fmt.Sprintf("items[%d].studentId", i)] = "รหัสนักศึกษาต้องเป็นตัวเลขจำนวนเต็มบวก"
			}
			if !isValidAttendanceStatus(st) {
				details[fmt.Sprintf("items[%d].status", i)] = "status ต้องเป็น present, absent, late หรือ unconfirmed เท่านั้น"
			}
			if note != nil && len([]rune(*note)) > 255 {
				details[fmt.Sprintf("items[%d].note", i)] = "หมายเหตุต้องมีความยาวไม่เกิน 255 ตัวอักษร"
			}
			normalizedItems = append(normalizedItems, normalizedBulkItem{
				StudentID: item.StudentID,
				Status:    st,
				HasNote:   item.Note != nil,
				Note:      note,
			})
		}
		if len(details) > 0 {
			return response.ValidationError(c, "ข้อมูลการเช็กชื่อแบบกลุ่มไม่ถูกต้อง", details)
		}
	}

	var session models.ClassSession
	var updatedCount int
	var summary AttendanceSummaryDTO

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSessionNotFound
			}
			return err
		}

		if session.Status == models.SessionStatusEnded {
			return errSessionAlreadyEnded
		}

		now := time.Now()

		if hasMarkAll {
			var activeEnrollments []models.Enrollment
			if err := tx.Where("course_id = ? AND status = ?", session.CourseID, models.EnrollmentStatusActive).
				Find(&activeEnrollments).Error; err != nil {
				return err
			}

			var checkedAt *time.Time
			if markAllStatus != models.AttendanceStatusUnconfirmed {
				checkedAt = &now
			}

			for _, enr := range activeEnrollments {
				var att models.Attendance
				err := tx.Where("session_id = ? AND student_id = ?", session.ID, enr.StudentID).First(&att).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					att = models.Attendance{
						SessionID: session.ID,
						CourseID:  session.CourseID,
						StudentID: enr.StudentID,
						Status:    markAllStatus,
						CheckedAt: checkedAt,
					}
					if err := tx.Create(&att).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				} else {
					if err := tx.Model(&att).Updates(map[string]interface{}{
						"status":     markAllStatus,
						"checked_at": checkedAt,
						"updated_at": now,
					}).Error; err != nil {
						return err
					}
				}
				updatedCount++
			}
		}

		if hasItems {
			for _, item := range normalizedItems {
				var enrollment models.Enrollment
				if err := tx.Where("course_id = ? AND student_id = ?", session.CourseID, item.StudentID).First(&enrollment).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return errStudentNotEnrolled
					}
					return err
				}
				if enrollment.Status != models.EnrollmentStatusActive {
					return errStudentNotEnrolled
				}

				var checkedAt *time.Time
				if item.Status != models.AttendanceStatusUnconfirmed {
					checkedAt = &now
				}

				var att models.Attendance
				err := tx.Where("session_id = ? AND student_id = ?", session.ID, item.StudentID).First(&att).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					att = models.Attendance{
						SessionID: session.ID,
						CourseID:  session.CourseID,
						StudentID: item.StudentID,
						Status:    item.Status,
						CheckedAt: checkedAt,
						Note:      item.Note,
					}
					if err := tx.Create(&att).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				} else {
					updates := map[string]interface{}{
						"status":     item.Status,
						"checked_at": checkedAt,
						"updated_at": now,
					}
					if item.HasNote {
						updates["note"] = item.Note
					}
					if err := tx.Model(&att).Updates(updates).Error; err != nil {
						return err
					}
				}
				updatedCount++
			}
		}

		sum, err := ComputeAttendanceSummary(tx, session.ID, session.CourseID)
		if err != nil {
			return err
		}
		summary = sum
		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถแก้ไขสถานะการเข้าเรียนในคาบเรียนที่จบไปแล้วได้")
		case errors.Is(txErr, errStudentNotEnrolled):
			return response.Unprocessable(c, response.CodeStudentNotEnrolled, "พบรหัสนักศึกษาที่ไม่ได้ลงทะเบียนเรียนในรายวิชาของคาบเรียนนี้ หรือถอนรายวิชาไปแล้ว")
		default:
			return response.InternalError(c, "ไม่สามารถอัปเดตสถานะการเข้าเรียนแบบกลุ่มได้")
		}
	}

	return response.OK(c, "อัปเดตสถานะการเข้าเรียนแบบกลุ่มสำเร็จ", BulkUpdateAttendanceResponseDTO{
		SessionID:    session.ID,
		UpdatedCount: updatedCount,
		Summary:      summary,
	})
}
