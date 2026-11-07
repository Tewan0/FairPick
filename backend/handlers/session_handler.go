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

type SessionHandler struct {
	db *gorm.DB
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

type AttendanceSummaryDTO struct {
	TotalEnrolled   int64 `json:"totalEnrolled"`
	Present         int64 `json:"present"`
	Late            int64 `json:"late"`
	Absent          int64 `json:"absent"`
	Unconfirmed     int64 `json:"unconfirmed"`
	ConfirmedCount  int64 `json:"confirmedCount"`
	EligibleForPick int64 `json:"eligibleForPick"`
}

type SessionPickSummaryDTO struct {
	TotalValidPicks         int64  `json:"totalValidPicks"`
	TotalCancelledPicks     int64  `json:"totalCancelledPicks"`
	RemainingInCurrentRound *int64 `json:"remainingInCurrentRound,omitempty"`
}

type SessionListItemDTO struct {
	ID                uint64                `json:"id"`
	CourseID          uint64                `json:"courseId"`
	SessionNumber     int                   `json:"sessionNumber"`
	Topic             *string               `json:"topic"`
	CurrentMode       string                `json:"currentMode"`
	CurrentRound      int                   `json:"currentRound"`
	Status            string                `json:"status"`
	StartedAt         time.Time             `json:"startedAt"`
	EndedAt           *time.Time            `json:"endedAt"`
	AttendanceSummary AttendanceSummaryDTO  `json:"attendanceSummary"`
	PickSummary       SessionPickSummaryDTO `json:"pickSummary"`
}

type StartSessionRequest struct {
	Topic       *string `json:"topic"`
	InitialMode *string `json:"initialMode"`
}

type StartSessionResponseDTO struct {
	ID                uint64               `json:"id"`
	CourseID          uint64               `json:"courseId"`
	SessionNumber     int                  `json:"sessionNumber"`
	Topic             *string              `json:"topic"`
	CurrentMode       string               `json:"currentMode"`
	CurrentRound      int                  `json:"currentRound"`
	Status            string               `json:"status"`
	StartedAt         time.Time            `json:"startedAt"`
	EndedAt           *time.Time           `json:"endedAt"`
	AttendanceSummary AttendanceSummaryDTO `json:"attendanceSummary"`
}

type SessionDetailDTO struct {
	ID                uint64                `json:"id"`
	CourseID          uint64                `json:"courseId"`
	CourseCode        *string               `json:"courseCode"`
	CourseName        *string               `json:"courseName"`
	Section           *string               `json:"section"`
	Semester          *string               `json:"semester"`
	SessionNumber     int                   `json:"sessionNumber"`
	Topic             *string               `json:"topic"`
	CurrentMode       string                `json:"currentMode"`
	CurrentRound      int                   `json:"currentRound"`
	Status            string                `json:"status"`
	StartedAt         time.Time             `json:"startedAt"`
	EndedAt           *time.Time            `json:"endedAt"`
	AttendanceSummary AttendanceSummaryDTO  `json:"attendanceSummary"`
	PickSummary       SessionPickSummaryDTO `json:"pickSummary"`
}

type EndSessionRequest struct {
	MarkUnconfirmedAsAbsent bool `json:"markUnconfirmedAsAbsent"`
}

type EndSessionResponseDTO struct {
	ID                          uint64               `json:"id"`
	CourseID                    uint64               `json:"courseId"`
	SessionNumber               int                  `json:"sessionNumber"`
	Status                      string               `json:"status"`
	StartedAt                   time.Time            `json:"startedAt"`
	EndedAt                     time.Time            `json:"endedAt"`
	MarkUnconfirmedAsAbsentUsed bool                 `json:"markUnconfirmedAsAbsentUsed"`
	UnconfirmedRemainingCount   int64                `json:"unconfirmedRemainingCount"`
	HasUnconfirmedWarning       bool                 `json:"hasUnconfirmedWarning"`
	WarningMessage              *string              `json:"warningMessage"`
	AttendanceSummary           AttendanceSummaryDTO `json:"attendanceSummary"`
}

type SwitchModeRequest struct {
	Mode string `json:"mode"`
}

type SwitchModeResponseDTO struct {
	SessionID        uint64 `json:"sessionId"`
	CurrentMode      string `json:"currentMode"`
	CurrentRound     int    `json:"currentRound"`
	EligibleCount    int64  `json:"eligibleCount"`
	RemainingInRound int64  `json:"remainingInRound"`
	IsRoundCompleted bool   `json:"isRoundCompleted"`
}

var (
	errCourseNotFound      = errors.New("course_not_found")
	errActiveSessionExists = errors.New("active_session_exists")
	errSessionNotFound     = errors.New("session_not_found")
	errSessionAlreadyEnded = errors.New("session_already_ended")
)

// ListCourseSessions handles GET /api/courses/:courseId/sessions
func (h *SessionHandler) ListCourseSessions(c *fiber.Ctx) error {
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

	statusFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("status")))
	if statusFilter != "" && statusFilter != models.SessionStatusActive && statusFilter != models.SessionStatusEnded {
		return response.ValidationError(c, "พารามิเตอร์ status ไม่ถูกต้อง", map[string]string{
			"status": "อนุญาตเฉพาะ active หรือ ended เท่านั้น",
		})
	}

	page, limit, offset := validator.ParsePagination(c, 20, 1000)

	query := h.db.Model(&models.ClassSession{}).Where("course_id = ?", courseID)
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงจำนวนคาบเรียนได้")
	}

	var sessions []models.ClassSession
	if err := query.Order("session_number DESC").Limit(limit).Offset(offset).Find(&sessions).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายการคาบเรียนได้")
	}

	items := make([]SessionListItemDTO, 0, len(sessions))
	for _, s := range sessions {
		attSummary, err := ComputeAttendanceSummary(h.db, s.ID, s.CourseID)
		if err != nil {
			return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการเข้าเรียนของคาบเรียนได้")
		}
		pickSummary, err := ComputeSessionPickSummary(h.db, s, false)
		if err != nil {
			return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการสุ่มของคาบเรียนได้")
		}
		items = append(items, SessionListItemDTO{
			ID:                s.ID,
			CourseID:          s.CourseID,
			SessionNumber:     s.SessionNumber,
			Topic:             s.Topic,
			CurrentMode:       s.CurrentMode,
			CurrentRound:      s.CurrentRound,
			Status:            s.Status,
			StartedAt:         s.StartedAt,
			EndedAt:           s.EndedAt,
			AttendanceSummary: attSummary,
			PickSummary:       pickSummary,
		})
	}

	return response.Paginated(c, "ดึงรายการคาบเรียนสำเร็จ", items, page, limit, totalItems)
}

// StartSession handles POST /api/courses/:courseId/sessions
func (h *SessionHandler) StartSession(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var req StartSessionRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
		}
	}

	topic := validator.NormalizeOptionalString(req.Topic)
	if topic != nil && len(*topic) > 255 {
		return response.ValidationError(c, "ข้อมูลคาบเรียนไม่ถูกต้อง", map[string]string{
			"topic": "หัวข้อคาบเรียนต้องไม่เกิน 255 ตัวอักษร",
		})
	}

	initialMode := models.PickModeWeighted
	if req.InitialMode != nil {
		trimmedMode := strings.ToLower(validator.NormalizeRequiredString(*req.InitialMode))
		if trimmedMode != "" {
			if trimmedMode != models.PickModeWeighted && trimmedMode != models.PickModeNoRepeat {
				return response.ValidationError(c, "โหมดการสุ่มเริ่มต้นไม่ถูกต้อง", map[string]string{
					"initialMode": "อนุญาตเฉพาะ weighted หรือ no_repeat เท่านั้น",
				})
			}
			initialMode = trimmedMode
		}
	}

	var newSession models.ClassSession
	var activeEnrollmentsCount int64

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		// Lock the course row with FOR UPDATE to serialize concurrent StartSession requests for the same course
		var course models.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errCourseNotFound
			}
			return err
		}

		// Ensure no active session already exists for this course
		var existingActiveCount int64
		if err := tx.Model(&models.ClassSession{}).
			Where("course_id = ? AND status = ?", courseID, models.SessionStatusActive).
			Count(&existingActiveCount).Error; err != nil {
			return err
		}
		if existingActiveCount > 0 {
			return errActiveSessionExists
		}

		// Calculate next session_number for this course
		var maxSessionNumber int
		if err := tx.Model(&models.ClassSession{}).
			Where("course_id = ?", courseID).
			Select("COALESCE(MAX(session_number), 0)").
			Scan(&maxSessionNumber).Error; err != nil {
			return err
		}

		newSession = models.ClassSession{
			CourseID:      courseID,
			SessionNumber: maxSessionNumber + 1,
			Topic:         topic,
			CurrentMode:   initialMode,
			CurrentRound:  1,
			Status:        models.SessionStatusActive,
			StartedAt:     time.Now(),
		}
		if err := tx.Create(&newSession).Error; err != nil {
			return err
		}

		// Fetch all active enrollments for this course and create initial unconfirmed attendance rows
		var enrollments []models.Enrollment
		if err := tx.Where("course_id = ? AND status = ?", courseID, models.EnrollmentStatusActive).
			Order("student_id ASC").
			Find(&enrollments).Error; err != nil {
			return err
		}

		activeEnrollmentsCount = int64(len(enrollments))
		if activeEnrollmentsCount > 0 {
			attendances := make([]models.Attendance, 0, len(enrollments))
			for _, e := range enrollments {
				attendances = append(attendances, models.Attendance{
					SessionID: newSession.ID,
					CourseID:  courseID,
					StudentID: e.StudentID,
					Status:    models.AttendanceStatusUnconfirmed,
				})
			}
			if err := tx.Create(&attendances).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errCourseNotFound):
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		case errors.Is(txErr, errActiveSessionExists) || validator.IsUniqueViolation(txErr):
			return response.Conflict(c, response.CodeActiveSessionExists, "รายวิชานี้มีคาบเรียนที่กำลังดำเนินการ (active) อยู่แล้ว กรุณาจบคาบเรียนเดิมก่อน")
		default:
			return response.InternalError(c, "ไม่สามารถเริ่มคาบเรียนใหม่ได้")
		}
	}

	dto := StartSessionResponseDTO{
		ID:            newSession.ID,
		CourseID:      newSession.CourseID,
		SessionNumber: newSession.SessionNumber,
		Topic:         newSession.Topic,
		CurrentMode:   newSession.CurrentMode,
		CurrentRound:  newSession.CurrentRound,
		Status:        newSession.Status,
		StartedAt:     newSession.StartedAt,
		EndedAt:       newSession.EndedAt,
		AttendanceSummary: AttendanceSummaryDTO{
			TotalEnrolled:   activeEnrollmentsCount,
			Present:         0,
			Late:            0,
			Absent:          0,
			Unconfirmed:     activeEnrollmentsCount,
			ConfirmedCount:  0,
			EligibleForPick: 0,
		},
	}

	msg := fmt.Sprintf("เริ่มคาบเรียนที่ %d สำเร็จ และสร้างใบเช็กชื่อเริ่มต้นเรียบร้อยแล้ว", newSession.SessionNumber)
	return response.Created(c, msg, dto)
}

// GetSession handles GET /api/sessions/:sessionId
func (h *SessionHandler) GetSession(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var session models.ClassSession
	if err := h.db.Preload("Course").First(&session, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลคาบเรียนได้")
	}

	attSummary, err := ComputeAttendanceSummary(h.db, session.ID, session.CourseID)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการเข้าเรียนได้")
	}

	pickSummary, err := ComputeSessionPickSummary(h.db, session, true)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการสุ่มได้")
	}

	var courseCode, courseName, section, semester *string
	if session.Course != nil {
		courseCode = session.Course.CourseCode
		courseName = session.Course.CourseName
		section = session.Course.Section
		semester = session.Course.Semester
	}

	detail := SessionDetailDTO{
		ID:                session.ID,
		CourseID:          session.CourseID,
		CourseCode:        courseCode,
		CourseName:        courseName,
		Section:           section,
		Semester:          semester,
		SessionNumber:     session.SessionNumber,
		Topic:             session.Topic,
		CurrentMode:       session.CurrentMode,
		CurrentRound:      session.CurrentRound,
		Status:            session.Status,
		StartedAt:         session.StartedAt,
		EndedAt:           session.EndedAt,
		AttendanceSummary: attSummary,
		PickSummary:       pickSummary,
	}

	return response.OK(c, "ดึงข้อมูลคาบเรียนสำเร็จ", detail)
}

// EndSession handles PATCH /api/sessions/:sessionId/end
func (h *SessionHandler) EndSession(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var req EndSessionRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
		}
	}

	var session models.ClassSession
	var endedAt time.Time
	var attSummary AttendanceSummaryDTO

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

		endedAt = time.Now()

		if req.MarkUnconfirmedAsAbsent {
			if err := tx.Model(&models.Attendance{}).
				Where("session_id = ? AND status = ?", session.ID, models.AttendanceStatusUnconfirmed).
				Updates(map[string]interface{}{
					"status":     models.AttendanceStatusAbsent,
					"checked_at": endedAt,
					"updated_at": endedAt,
				}).Error; err != nil {
				return err
			}
		}

		session.Status = models.SessionStatusEnded
		session.EndedAt = &endedAt
		session.UpdatedAt = endedAt

		if err := tx.Model(&session).
			Select("status", "ended_at", "updated_at").
			Updates(&session).Error; err != nil {
			return err
		}

		var sumErr error
		attSummary, sumErr = ComputeAttendanceSummary(tx, session.ID, session.CourseID)
		return sumErr
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "คาบเรียนนี้จบไปแล้ว ไม่สามารถจบคาบเรียนซ้ำได้")
		default:
			return response.InternalError(c, "ไม่สามารถจบคาบเรียนได้")
		}
	}

	unconfirmedRemaining := attSummary.Unconfirmed
	hasWarning := unconfirmedRemaining > 0
	var warningMsg *string
	msg := "จบคาบเรียนสำเร็จ"

	if hasWarning {
		w := fmt.Sprintf(
			"ยังมีนักศึกษาที่ไม่ได้ยืนยันสถานะการเข้าเรียน (unconfirmed) จำนวน %d คน โดยระบบคงสถานะไว้ตามจริงและไม่นำมาคิดเป็นขาดเรียน (absent)",
			unconfirmedRemaining,
		)
		warningMsg = &w
		msg = fmt.Sprintf("จบคาบเรียนสำเร็จ (มีนักศึกษาที่ยังไม่ได้ยืนยันสถานะ %d คน คงสถานะเป็น unconfirmed ตามจริง)", unconfirmedRemaining)
	}

	dto := EndSessionResponseDTO{
		ID:                          session.ID,
		CourseID:                    session.CourseID,
		SessionNumber:               session.SessionNumber,
		Status:                      session.Status,
		StartedAt:                   session.StartedAt,
		EndedAt:                     endedAt,
		MarkUnconfirmedAsAbsentUsed: req.MarkUnconfirmedAsAbsent,
		UnconfirmedRemainingCount:   unconfirmedRemaining,
		HasUnconfirmedWarning:       hasWarning,
		WarningMessage:              warningMsg,
		AttendanceSummary:           attSummary,
	}

	return response.OK(c, msg, dto)
}

// SwitchMode handles PATCH /api/sessions/:sessionId/mode
func (h *SessionHandler) SwitchMode(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var req SwitchModeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	mode := strings.ToLower(validator.NormalizeRequiredString(req.Mode))
	if mode != models.PickModeWeighted && mode != models.PickModeNoRepeat {
		return response.ValidationError(c, "โหมดการสุ่มไม่ถูกต้อง", map[string]string{
			"mode": "อนุญาตเฉพาะ weighted หรือ no_repeat เท่านั้น",
		})
	}

	var session models.ClassSession
	var eligibleCount int64
	var remainingInRound int64

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

		session.CurrentMode = mode
		session.UpdatedAt = time.Now()

		if err := tx.Model(&session).Select("current_mode", "updated_at").Updates(&session).Error; err != nil {
			return err
		}

		attSummary, err := ComputeAttendanceSummary(tx, session.ID, session.CourseID)
		if err != nil {
			return err
		}
		eligibleCount = attSummary.EligibleForPick

		rem, err := ComputeRemainingInCurrentNoRepeatRound(tx, session.ID, session.CourseID, session.CurrentRound)
		if err != nil {
			return err
		}
		remainingInRound = rem
		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถเปลี่ยนโหมดการสุ่มในคาบเรียนที่จบไปแล้วได้")
		default:
			return response.InternalError(c, "ไม่สามารถเปลี่ยนโหมดการสุ่มได้")
		}
	}

	isRoundCompleted := session.CurrentMode == models.PickModeNoRepeat && eligibleCount > 0 && remainingInRound == 0

	return response.OK(c, fmt.Sprintf("เปลี่ยนโหมดการสุ่มเป็น %s สำเร็จ", mode), SwitchModeResponseDTO{
		SessionID:        session.ID,
		CurrentMode:      session.CurrentMode,
		CurrentRound:     session.CurrentRound,
		EligibleCount:    eligibleCount,
		RemainingInRound: remainingInRound,
		IsRoundCompleted: isRoundCompleted,
	})
}

// ComputeAttendanceSummary calculates attendance counts for a session.
// EligibleForPick counts students with status IN ('present', 'late') whose enrollment in the course is 'active'.
func ComputeAttendanceSummary(db *gorm.DB, sessionID, courseID uint64) (AttendanceSummaryDTO, error) {
	type statusCountRow struct {
		Status string
		Count  int64
	}

	var rows []statusCountRow
	if err := db.Model(&models.Attendance{}).
		Select("status, COUNT(*) as count").
		Where("session_id = ?", sessionID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return AttendanceSummaryDTO{}, err
	}

	var summary AttendanceSummaryDTO
	for _, r := range rows {
		switch r.Status {
		case models.AttendanceStatusPresent:
			summary.Present = r.Count
		case models.AttendanceStatusLate:
			summary.Late = r.Count
		case models.AttendanceStatusAbsent:
			summary.Absent = r.Count
		case models.AttendanceStatusUnconfirmed:
			summary.Unconfirmed = r.Count
		}
	}

	summary.TotalEnrolled = summary.Present + summary.Late + summary.Absent + summary.Unconfirmed
	summary.ConfirmedCount = summary.Present + summary.Late + summary.Absent

	var eligible int64
	if err := db.Model(&models.Attendance{}).
		Joins("JOIN enrollments ON enrollments.course_id = attendance.course_id AND enrollments.student_id = attendance.student_id").
		Where("attendance.session_id = ? AND attendance.course_id = ? AND attendance.status IN ? AND enrollments.status = ?",
			sessionID, courseID, []string{models.AttendanceStatusPresent, models.AttendanceStatusLate}, models.EnrollmentStatusActive).
		Count(&eligible).Error; err != nil {
		return AttendanceSummaryDTO{}, err
	}
	summary.EligibleForPick = eligible

	return summary, nil
}

// ComputeSessionPickSummary calculates valid/cancelled pick counts and optionally remaining students in the current no_repeat round.
func ComputeSessionPickSummary(db *gorm.DB, session models.ClassSession, includeRemaining bool) (SessionPickSummaryDTO, error) {
	var validPicks, cancelledPicks int64

	if err := db.Model(&models.PickHistory{}).
		Where("session_id = ? AND status = ?", session.ID, models.PickStatusValid).
		Count(&validPicks).Error; err != nil {
		return SessionPickSummaryDTO{}, err
	}

	if err := db.Model(&models.PickHistory{}).
		Where("session_id = ? AND status = ?", session.ID, models.PickStatusCancelled).
		Count(&cancelledPicks).Error; err != nil {
		return SessionPickSummaryDTO{}, err
	}

	out := SessionPickSummaryDTO{
		TotalValidPicks:     validPicks,
		TotalCancelledPicks: cancelledPicks,
	}

	if includeRemaining {
		rem, err := ComputeRemainingInCurrentNoRepeatRound(db, session.ID, session.CourseID, session.CurrentRound)
		if err != nil {
			return SessionPickSummaryDTO{}, err
		}
		out.RemainingInCurrentRound = &rem
	}

	return out, nil
}

// ComputeRemainingInCurrentNoRepeatRound returns the count of eligible (present/late & active enrollment)
// students in the session who have not yet been picked with mode='no_repeat', round_number=currentRound, status='valid'.
func ComputeRemainingInCurrentNoRepeatRound(db *gorm.DB, sessionID, courseID uint64, currentRound int) (int64, error) {
	var remaining int64
	err := db.Model(&models.Attendance{}).
		Joins("JOIN enrollments ON enrollments.course_id = attendance.course_id AND enrollments.student_id = attendance.student_id").
		Where("attendance.session_id = ? AND attendance.course_id = ? AND attendance.status IN ? AND enrollments.status = ?",
			sessionID, courseID, []string{models.AttendanceStatusPresent, models.AttendanceStatusLate}, models.EnrollmentStatusActive).
		Where(`NOT EXISTS (
			SELECT 1 FROM pick_history ph
			WHERE ph.session_id = attendance.session_id
			  AND ph.student_id = attendance.student_id
			  AND ph.mode = ?
			  AND ph.round_number = ?
			  AND ph.status = ?
		)`, models.PickModeNoRepeat, currentRound, models.PickStatusValid).
		Count(&remaining).Error
	return remaining, err
}

