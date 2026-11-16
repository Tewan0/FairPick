package handlers

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/importer"
	"github.com/Tewan0/FairPick/pkg/response"
	"github.com/Tewan0/FairPick/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EnrollmentHandler struct {
	db *gorm.DB
}

func NewEnrollmentHandler(db *gorm.DB) *EnrollmentHandler {
	return &EnrollmentHandler{db: db}
}

type EnrolledStudentDTO struct {
	EnrollmentID uint64     `json:"enrollmentId"`
	CourseID     uint64     `json:"courseId"`
	StudentID    uint64     `json:"studentId"`
	StudentCode  string     `json:"studentCode"`
	FullName     string     `json:"fullName"`
	Status       string     `json:"status"`
	EnrolledAt   time.Time  `json:"enrolledAt"`
	WithdrawnAt  *time.Time `json:"withdrawnAt"`
}

type EnrollStudentRequest struct {
	StudentID   *uint64 `json:"studentId"`
	StudentCode string  `json:"studentCode"`
	FullName    string  `json:"fullName"`
}

type EnrollStudentResponseDTO struct {
	EnrollmentID           uint64     `json:"enrollmentId"`
	CourseID               uint64     `json:"courseId"`
	StudentID              uint64     `json:"studentId"`
	StudentCode            string     `json:"studentCode"`
	FullName               string     `json:"fullName"`
	Status                 string     `json:"status"`
	AddedToActiveSessionID *uint64    `json:"addedToActiveSessionId"`
	EnrolledAt             time.Time  `json:"enrolledAt"`
	WithdrawnAt            *time.Time `json:"withdrawnAt"`
}

type BulkEnrollRequest struct {
	Students []importer.BulkStudentItem `json:"students"`
	RawText  string                     `json:"rawText"`
}

type BulkImportSummaryDTO struct {
	TotalRowsRead           int `json:"totalRowsRead"`
	NewlyCreatedAndEnrolled int `json:"newlyCreatedAndEnrolled"`
	ExistingStudentEnrolled int `json:"existingStudentEnrolled"`
	ReactivatedEnrollment   int `json:"reactivatedEnrollment"`
	AlreadyEnrolledSkipped  int `json:"alreadyEnrolledSkipped"`
	InvalidRowsSkipped      int `json:"invalidRowsSkipped"`
}

type BulkImportResponseDTO struct {
	CourseID               uint64                `json:"courseId"`
	ActiveSessionIDUpdated *uint64               `json:"activeSessionIdUpdated"`
	Summary                BulkImportSummaryDTO  `json:"summary"`
	InvalidRows            []importer.InvalidRow `json:"invalidRows"`
}

var (
	errStudentNotFound       = errors.New("student_not_found")
	errMissingStudentName    = errors.New("missing_student_name")
	errAlreadyEnrolledActive = errors.New("already_enrolled_active")
)

// ListCourseStudents handles GET /api/courses/:courseId/students
func (h *EnrollmentHandler) ListCourseStudents(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
	}

	statusFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("status", models.EnrollmentStatusActive)))
	if statusFilter != models.EnrollmentStatusActive && statusFilter != models.EnrollmentStatusWithdrawn && statusFilter != "all" {
		return response.ValidationError(c, "พารามิเตอร์ status ไม่ถูกต้อง", map[string]string{
			"status": "อนุญาตเฉพาะ active, withdrawn หรือ all เท่านั้น",
		})
	}

	page, limit, offset := validator.ParsePagination(c, 100, 1000)
	search := validator.NormalizeRequiredString(c.Query("search"))

	query := h.db.Model(&models.Enrollment{}).
		Joins("JOIN students ON students.id = enrollments.student_id").
		Where("enrollments.course_id = ?", courseID)

	if statusFilter != "all" {
		query = query.Where("enrollments.status = ?", statusFilter)
	}
	if search != "" {
		like := validator.FormatSearchLike(search)
		query = query.Where("students.student_code ILIKE ? OR students.full_name ILIKE ?", like, like)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงจำนวนนักศึกษาในรายวิชาได้")
	}

	var enrollments []models.Enrollment
	if err := query.Preload("Student").
		Order("students.student_code ASC").
		Limit(limit).
		Offset(offset).
		Find(&enrollments).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายชื่อนักศึกษาในรายวิชาได้")
	}

	items := make([]EnrolledStudentDTO, 0, len(enrollments))
	for _, e := range enrollments {
		if e.Student == nil {
			continue
		}
		items = append(items, EnrolledStudentDTO{
			EnrollmentID: e.ID,
			CourseID:     e.CourseID,
			StudentID:    e.StudentID,
			StudentCode:  e.Student.StudentCode,
			FullName:     e.Student.FullName,
			Status:       e.Status,
			EnrolledAt:   e.EnrolledAt,
			WithdrawnAt:  e.WithdrawnAt,
		})
	}

	return response.Paginated(c, "ดึงรายชื่อนักศึกษาในรายวิชาสำเร็จ", items, page, limit, totalItems)
}

// EnrollStudent handles POST /api/courses/:courseId/enrollments
func (h *EnrollmentHandler) EnrollStudent(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
	}

	var req EnrollStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	code := validator.NormalizeRequiredString(req.StudentCode)
	name := validator.NormalizeRequiredString(req.FullName)

	if req.StudentID == nil && code == "" {
		return response.ValidationError(c, "ข้อมูลการลงทะเบียนไม่ครบถ้วน", map[string]string{
			"studentCode": "กรุณาระบุ studentId หรือ studentCode",
		})
	}
	if req.StudentID != nil && *req.StudentID == 0 {
		return response.ValidationError(c, "รหัสนักศึกษา (studentId) ไม่ถูกต้อง", map[string]string{
			"studentId": "studentId ต้องมากกว่า 0",
		})
	}
	if code != "" && len(code) > 30 {
		return response.ValidationError(c, "ข้อมูลนักศึกษาไม่ถูกต้อง", map[string]string{
			"studentCode": "รหัสนักศึกษาต้องไม่เกิน 30 ตัวอักษร",
		})
	}

	var student models.Student
	var enrollment models.Enrollment
	var addedActiveSessionID *uint64

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		if req.StudentID != nil {
			if err := tx.First(&student, *req.StudentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errStudentNotFound
				}
				return err
			}
		} else {
			err := tx.Where("student_code = ?", code).First(&student).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if name == "" {
					return errMissingStudentName
				}
				student = models.Student{
					StudentCode: code,
					FullName:    name,
				}
				if err := tx.Create(&student).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}

		err := tx.Where("course_id = ? AND student_id = ?", courseID, student.ID).First(&enrollment).Error
		if err == nil {
			if enrollment.Status == models.EnrollmentStatusActive {
				return errAlreadyEnrolledActive
			}
			// Reactivate withdrawn enrollment while preserving past history
			now := time.Now()
			enrollment.Status = models.EnrollmentStatusActive
			enrollment.WithdrawnAt = nil
			enrollment.UpdatedAt = now
			if err := tx.Model(&enrollment).Select("status", "withdrawn_at", "updated_at").Updates(&enrollment).Error; err != nil {
				return err
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			enrollment = models.Enrollment{
				CourseID:  courseID,
				StudentID: student.ID,
				Status:    models.EnrollmentStatusActive,
			}
			if err := tx.Create(&enrollment).Error; err != nil {
				return err
			}
		} else {
			return err
		}

		// If course has an active session, ensure an unconfirmed attendance record exists
		var activeSession models.ClassSession
		if err := tx.Where("course_id = ? AND status = ?", courseID, models.SessionStatusActive).First(&activeSession).Error; err == nil {
			att := models.Attendance{
				SessionID: activeSession.ID,
				CourseID:  courseID,
				StudentID: student.ID,
				Status:    models.AttendanceStatusUnconfirmed,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "session_id"}, {Name: "student_id"}},
				DoNothing: true,
			}).Create(&att).Error; err != nil {
				return err
			}
			sid := activeSession.ID
			addedActiveSessionID = &sid
		}

		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errStudentNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลนักศึกษาที่ระบุ")
		case errors.Is(txErr, errMissingStudentName):
			return response.ValidationError(c, "ไม่พบรหัสนักศึกษานี้ในระบบ กรุณาระบุชื่อ-นามสกุล (fullName) เพื่อสร้างนักศึกษาใหม่", map[string]string{
				"fullName": "กรุณาระบุชื่อ-นามสกุลนักศึกษา",
			})
		case errors.Is(txErr, errAlreadyEnrolledActive):
			return response.Conflict(
				c,
				response.CodeDuplicateResource,
				fmt.Sprintf("นักศึกษารหัส %s ลงทะเบียนในรายวิชานี้อยู่แล้ว", student.StudentCode),
			)
		default:
			return response.InternalError(c, "ไม่สามารถลงทะเบียนนักศึกษาเข้าสู่รายวิชาได้")
		}
	}

	return response.Created(c, "ลงทะเบียนนักศึกษาเข้าสู่รายวิชาสำเร็จ", EnrollStudentResponseDTO{
		EnrollmentID:           enrollment.ID,
		CourseID:               enrollment.CourseID,
		StudentID:              student.ID,
		StudentCode:            student.StudentCode,
		FullName:               student.FullName,
		Status:                 enrollment.Status,
		AddedToActiveSessionID: addedActiveSessionID,
		EnrolledAt:             enrollment.EnrolledAt,
		WithdrawnAt:            enrollment.WithdrawnAt,
	})
}

// WithdrawStudent handles DELETE /api/courses/:courseId/enrollments/:studentId
func (h *EnrollmentHandler) WithdrawStudent(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสอ้างอิงนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
	}

	var enrollment models.Enrollment
	if err := h.db.Where("course_id = ? AND student_id = ?", courseID, studentID).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลการลงทะเบียนของนักศึกษาในรายวิชานี้")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบข้อมูลการลงทะเบียนได้")
	}

	if enrollment.Status == models.EnrollmentStatusWithdrawn {
		return response.Unprocessable(c, response.CodeStudentNotEnrolled, "นักศึกษาถูกถอนออกจากรายวิชานี้ไปแล้ว")
	}

	// Check if student has any pick_history or confirmed/ended-session attendance in this course
	var pickCount int64
	h.db.Model(&models.PickHistory{}).
		Where("course_id = ? AND student_id = ?", courseID, studentID).
		Count(&pickCount)

	var confirmedAttCount int64
	h.db.Model(&models.Attendance{}).
		Where("course_id = ? AND student_id = ? AND status <> ?", courseID, studentID, models.AttendanceStatusUnconfirmed).
		Count(&confirmedAttCount)

	var endedSessionAttCount int64
	h.db.Model(&models.Attendance{}).
		Joins("JOIN class_sessions ON class_sessions.id = attendance.session_id").
		Where("attendance.course_id = ? AND attendance.student_id = ? AND class_sessions.status = ?", courseID, studentID, models.SessionStatusEnded).
		Count(&endedSessionAttCount)

	hasHistory := pickCount > 0 || confirmedAttCount > 0 || endedSessionAttCount > 0

	var action string
	var finalStatus string
	var withdrawnAt *time.Time

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Remove any unconfirmed attendance in currently active session if student has no picks in that session
		var activeSession models.ClassSession
		if err := tx.Where("course_id = ? AND status = ?", courseID, models.SessionStatusActive).First(&activeSession).Error; err == nil {
			var activePickCount int64
			tx.Model(&models.PickHistory{}).
				Where("session_id = ? AND student_id = ?", activeSession.ID, studentID).
				Count(&activePickCount)

			if activePickCount == 0 {
				if err := tx.Where("session_id = ? AND student_id = ? AND status = ?",
					activeSession.ID, studentID, models.AttendanceStatusUnconfirmed).
					Delete(&models.Attendance{}).Error; err != nil {
					return err
				}
			}
		}

		if hasHistory {
			now := time.Now()
			withdrawnAt = &now
			action = "withdrawn"
			finalStatus = models.EnrollmentStatusWithdrawn
			enrollment.Status = models.EnrollmentStatusWithdrawn
			enrollment.WithdrawnAt = &now
			enrollment.UpdatedAt = now
			return tx.Model(&enrollment).Select("status", "withdrawn_at", "updated_at").Updates(&enrollment).Error
		}

		// No history in this course -> can delete the enrollment row cleanly
		action = "deleted"
		finalStatus = "deleted"
		return tx.Delete(&models.Enrollment{}, enrollment.ID).Error
	})

	if err != nil {
		return response.InternalError(c, "ไม่สามารถถอนนักศึกษาออกจากรายวิชาได้")
	}

	msg := "นำนักศึกษาออกจากรายวิชาสำเร็จ"
	if hasHistory {
		msg = "ถอนนักศึกษาออกจากรายวิชาสำเร็จ (คงประวัติการเรียนย้อนหลังไว้)"
	}

	return response.OK(c, msg, fiber.Map{
		"courseId":         courseID,
		"studentId":        studentID,
		"action":           action,
		"status":           finalStatus,
		"historyPreserved": hasHistory,
		"withdrawnAt":      withdrawnAt,
	})
}

// ImportStudentsFile handles POST /api/courses/:courseId/students/import-file
func (h *EnrollmentHandler) ImportStudentsFile(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return response.ValidationError(c, "กรุณาแนบไฟล์ในฟิลด์ 'file' (.xlsx หรือ .csv)", map[string]string{
			"file": "ไม่พบไฟล์ที่อัปโหลด",
		})
	}

	const maxUploadSize = 5 * 1024 * 1024 // 5 MB max upload size per GO-UPLOAD-001 / GO-HTTP-002
	if fileHeader.Size > maxUploadSize {
		return response.ValidationError(c, "ไฟล์มีขนาดใหญ่เกินไป (จำกัดไม่เกิน 5 MB)", map[string]string{
			"file": "ขนาดไฟล์ต้องไม่เกิน 5 MB",
		})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return response.InvalidRequest(c, "ไม่สามารถเปิดไฟล์ที่อัปโหลดได้")
	}
	defer f.Close()

	content, err := io.ReadAll(io.LimitReader(f, maxUploadSize+1))
	if err != nil {
		return response.InvalidRequest(c, "ไม่สามารถอ่านเนื้อหาไฟล์ที่อัปโหลดได้")
	}
	if len(content) > maxUploadSize {
		return response.ValidationError(c, "ไฟล์มีขนาดใหญ่เกินไป (จำกัดไม่เกิน 5 MB)", map[string]string{
			"file": "ขนาดไฟล์ต้องไม่เกิน 5 MB",
		})
	}

	parsed, err := importer.ParseUploadedFile(fileHeader.Filename, content)
	if err != nil {
		return response.ValidationError(c, err.Error(), map[string]string{
			"file": err.Error(),
		})
	}

	if parsed.TotalRowsRead == 0 {
		return response.ValidationError(c, "ไม่พบข้อมูลรายชื่อนักศึกษาในไฟล์", map[string]string{
			"file": "ไฟล์ไม่มีแถวข้อมูลนักศึกษา",
		})
	}

	result, err := h.processBulkStudentImport(courseID, parsed)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถนำเข้ารายชื่อนักศึกษาได้")
	}

	return response.OK(c, "นำเข้ารายชื่อนักศึกษาสำเร็จ", result)
}

// BulkEnrollStudents handles POST /api/courses/:courseId/students/bulk
func (h *EnrollmentHandler) BulkEnrollStudents(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
	}

	var req BulkEnrollRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	if len(req.Students) == 0 && strings.TrimSpace(req.RawText) == "" {
		return response.ValidationError(c, "กรุณาระบุรายชื่อนักศึกษาใน students หรือ rawText อย่างน้อย 1 รายการ", map[string]string{
			"students": "ไม่มีข้อมูลรายชื่อนักศึกษา",
		})
	}

	parsed := importer.ParseBulkInput(req.Students, req.RawText)
	if parsed.TotalRowsRead == 0 {
		return response.ValidationError(c, "ไม่พบข้อมูลรายชื่อนักศึกษาที่สามารถประมวลผลได้", map[string]string{
			"rawText": "ข้อมูลว่างเปล่า",
		})
	}

	result, err := h.processBulkStudentImport(courseID, parsed)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถนำเข้ารายชื่อนักศึกษาแบบกลุ่มได้")
	}

	return response.OK(c, "นำเข้ารายชื่อนักศึกษาสำเร็จ", result)
}

func (h *EnrollmentHandler) processBulkStudentImport(courseID uint64, parsed *importer.ParseResult) (*BulkImportResponseDTO, error) {
	summary := BulkImportSummaryDTO{
		TotalRowsRead:      parsed.TotalRowsRead,
		InvalidRowsSkipped: len(parsed.InvalidRows),
	}

	var activeSessionIDUpdated *uint64

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var activeSession models.ClassSession
		hasActiveSession := false
		if err := tx.Where("course_id = ? AND status = ?", courseID, models.SessionStatusActive).First(&activeSession).Error; err == nil {
			hasActiveSession = true
		}

		seenInBatch := make(map[string]bool)

		for _, row := range parsed.ValidRows {
			if seenInBatch[row.StudentCode] {
				summary.AlreadyEnrolledSkipped++
				continue
			}
			seenInBatch[row.StudentCode] = true

			var student models.Student
			isNewStudent := false
			err := tx.Where("student_code = ?", row.StudentCode).First(&student).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				student = models.Student{
					StudentCode: row.StudentCode,
					FullName:    row.FullName,
				}
				if err := tx.Create(&student).Error; err != nil {
					return err
				}
				isNewStudent = true
			} else if err != nil {
				return err
			}

			var enrollment models.Enrollment
			err = tx.Where("course_id = ? AND student_id = ?", courseID, student.ID).First(&enrollment).Error
			enrolledOrReactivated := false

			if errors.Is(err, gorm.ErrRecordNotFound) {
				enrollment = models.Enrollment{
					CourseID:  courseID,
					StudentID: student.ID,
					Status:    models.EnrollmentStatusActive,
				}
				if err := tx.Create(&enrollment).Error; err != nil {
					return err
				}
				enrolledOrReactivated = true
				if isNewStudent {
					summary.NewlyCreatedAndEnrolled++
				} else {
					summary.ExistingStudentEnrolled++
				}
			} else if err == nil {
				if enrollment.Status == models.EnrollmentStatusActive {
					summary.AlreadyEnrolledSkipped++
				} else {
					now := time.Now()
					enrollment.Status = models.EnrollmentStatusActive
					enrollment.WithdrawnAt = nil
					enrollment.UpdatedAt = now
					if err := tx.Model(&enrollment).Select("status", "withdrawn_at", "updated_at").Updates(&enrollment).Error; err != nil {
						return err
					}
					enrolledOrReactivated = true
					summary.ReactivatedEnrollment++
				}
			} else {
				return err
			}

			if enrolledOrReactivated && hasActiveSession {
				att := models.Attendance{
					SessionID: activeSession.ID,
					CourseID:  courseID,
					StudentID: student.ID,
					Status:    models.AttendanceStatusUnconfirmed,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "session_id"}, {Name: "student_id"}},
					DoNothing: true,
				}).Create(&att).Error; err != nil {
					return err
				}
				sid := activeSession.ID
				activeSessionIDUpdated = &sid
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &BulkImportResponseDTO{
		CourseID:               courseID,
		ActiveSessionIDUpdated: activeSessionIDUpdated,
		Summary:                summary,
		InvalidRows:            parsed.InvalidRows,
	}, nil
}

