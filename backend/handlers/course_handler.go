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
)

type CourseHandler struct {
	db *gorm.DB
}

func NewCourseHandler(db *gorm.DB) *CourseHandler {
	return &CourseHandler{db: db}
}

type CourseUpsertRequest struct {
	CourseCode *string `json:"courseCode"`
	CourseName *string `json:"courseName"`
	Section    *string `json:"section"`
	Semester   *string `json:"semester"`
}

type CourseListItemDTO struct {
	ID              uint64    `json:"id"`
	CourseCode      *string   `json:"courseCode"`
	CourseName      *string   `json:"courseName"`
	Section         *string   `json:"section"`
	Semester        *string   `json:"semester"`
	EnrolledCount   int64     `json:"enrolledCount"`
	TotalSessions   int64     `json:"totalSessions"`
	ActiveSessionID *uint64   `json:"activeSessionId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ActiveSessionBriefDTO struct {
	ID            uint64    `json:"id"`
	SessionNumber int       `json:"sessionNumber"`
	Topic         *string   `json:"topic"`
	CurrentMode   string    `json:"currentMode"`
	CurrentRound  int       `json:"currentRound"`
	Status        string    `json:"status"`
	StartedAt     time.Time `json:"startedAt"`
}

type CourseDetailDTO struct {
	ID            uint64                 `json:"id"`
	CourseCode    *string                `json:"courseCode"`
	CourseName    *string                `json:"courseName"`
	Section       *string                `json:"section"`
	Semester      *string                `json:"semester"`
	EnrolledCount int64                  `json:"enrolledCount"`
	TotalSessions int64                  `json:"totalSessions"`
	ActiveSession *ActiveSessionBriefDTO `json:"activeSession"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

// ListCourses handles GET /api/courses
func (h *CourseHandler) ListCourses(c *fiber.Ctx) error {
	page, limit, offset := validator.ParsePagination(c, 20, 1000)
	search := validator.NormalizeRequiredString(c.Query("search"))
	semester := validator.NormalizeRequiredString(c.Query("semester"))

	query := h.db.Model(&models.Course{})

	if search != "" {
		like := validator.FormatSearchLike(search)
		query = query.Where("course_code ILIKE ? OR course_name ILIKE ? OR section ILIKE ?", like, like, like)
	}
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงจำนวนรายวิชาได้")
	}

	var courses []models.Course
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&courses).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลรายวิชาได้")
	}

	items := make([]CourseListItemDTO, 0, len(courses))
	for _, course := range courses {
		dto, err := h.buildCourseListItemDTO(course)
		if err != nil {
			return response.InternalError(c, "ไม่สามารถสรุปข้อมูลรายวิชาได้")
		}
		items = append(items, dto)
	}

	return response.Paginated(c, "ดึงข้อมูลรายวิชาสำเร็จ", items, page, limit, totalItems)
}

// CreateCourse handles POST /api/courses
func (h *CourseHandler) CreateCourse(c *fiber.Ctx) error {
	var req CourseUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	code := validator.NormalizeOptionalString(req.CourseCode)
	name := validator.NormalizeOptionalString(req.CourseName)
	section := validator.NormalizeOptionalString(req.Section)
	semester := validator.NormalizeOptionalString(req.Semester)

	if details := validator.ValidateCourseFields(code, name, section, semester); details != nil {
		return response.ValidationError(c, "ข้อมูลรายวิชาไม่ถูกต้อง", details)
	}

	isDup, dupMsg, err := h.checkDuplicateCourse(0, code, name, section, semester)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถตรวจสอบข้อมูลรายวิชาซ้ำได้")
	}
	if isDup {
		return response.Conflict(c, response.CodeDuplicateResource, dupMsg)
	}

	course := models.Course{
		CourseCode: code,
		CourseName: name,
		Section:    section,
		Semester:   semester,
	}

	if err := h.db.Create(&course).Error; err != nil {
		if validator.IsUniqueViolation(err) {
			return response.Conflict(c, response.CodeDuplicateResource, "รายวิชานี้มีอยู่ในระบบแล้ว")
		}
		return response.InternalError(c, "ไม่สามารถสร้างรายวิชาได้")
	}

	dto, err := h.buildCourseListItemDTO(course)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถอ่านข้อมูลรายวิชาที่สร้างได้")
	}

	return response.Created(c, "สร้างรายวิชาสำเร็จ", dto)
}

// GetCourse handles GET /api/courses/:courseId
func (h *CourseHandler) GetCourse(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงรายละเอียดรายวิชาได้")
	}

	var enrolledCount int64
	h.db.Model(&models.Enrollment{}).
		Where("course_id = ? AND status = ?", course.ID, models.EnrollmentStatusActive).
		Count(&enrolledCount)

	var totalSessions int64
	h.db.Model(&models.ClassSession{}).
		Where("course_id = ?", course.ID).
		Count(&totalSessions)

	var activeSession models.ClassSession
	var activeBrief *ActiveSessionBriefDTO
	if err := h.db.Where("course_id = ? AND status = ?", course.ID, models.SessionStatusActive).First(&activeSession).Error; err == nil {
		activeBrief = &ActiveSessionBriefDTO{
			ID:            activeSession.ID,
			SessionNumber: activeSession.SessionNumber,
			Topic:         activeSession.Topic,
			CurrentMode:   activeSession.CurrentMode,
			CurrentRound:  activeSession.CurrentRound,
			Status:        activeSession.Status,
			StartedAt:     activeSession.StartedAt,
		}
	}

	detail := CourseDetailDTO{
		ID:            course.ID,
		CourseCode:    course.CourseCode,
		CourseName:    course.CourseName,
		Section:       course.Section,
		Semester:      course.Semester,
		EnrolledCount: enrolledCount,
		TotalSessions: totalSessions,
		ActiveSession: activeBrief,
		CreatedAt:     course.CreatedAt,
		UpdatedAt:     course.UpdatedAt,
	}

	return response.OK(c, "ดึงรายละเอียดรายวิชาสำเร็จ", detail)
}

// UpdateCourse handles PUT /api/courses/:courseId
func (h *CourseHandler) UpdateCourse(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลรายวิชาได้")
	}

	var req CourseUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	code := validator.NormalizeOptionalString(req.CourseCode)
	name := validator.NormalizeOptionalString(req.CourseName)
	section := validator.NormalizeOptionalString(req.Section)
	semester := validator.NormalizeOptionalString(req.Semester)

	if details := validator.ValidateCourseFields(code, name, section, semester); details != nil {
		return response.ValidationError(c, "ข้อมูลรายวิชาไม่ถูกต้อง", details)
	}

	isDup, dupMsg, err := h.checkDuplicateCourse(course.ID, code, name, section, semester)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถตรวจสอบข้อมูลรายวิชาซ้ำได้")
	}
	if isDup {
		return response.Conflict(c, response.CodeDuplicateResource, dupMsg)
	}

	course.CourseCode = code
	course.CourseName = name
	course.Section = section
	course.Semester = semester
	course.UpdatedAt = time.Now()

	if err := h.db.Model(&course).Select("course_code", "course_name", "section", "semester", "updated_at").Updates(&course).Error; err != nil {
		if validator.IsUniqueViolation(err) {
			return response.Conflict(c, response.CodeDuplicateResource, "ข้อมูลรายวิชาซ้ำกับรายวิชาที่มีอยู่ในระบบ")
		}
		return response.InternalError(c, "ไม่สามารถแก้ไขข้อมูลรายวิชาได้")
	}

	dto, err := h.buildCourseListItemDTO(course)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถอ่านข้อมูลรายวิชาที่แก้ไขได้")
	}

	return response.OK(c, "แก้ไขข้อมูลรายวิชาสำเร็จ", dto)
}

// DeleteCourse handles DELETE /api/courses/:courseId
func (h *CourseHandler) DeleteCourse(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลรายวิชาได้")
	}

	// Check if the course has any class_sessions, attendance, or pick_history
	var sessionCount, attendanceCount, pickCount int64
	h.db.Model(&models.ClassSession{}).Where("course_id = ?", courseID).Count(&sessionCount)
	h.db.Model(&models.Attendance{}).Where("course_id = ?", courseID).Count(&attendanceCount)
	h.db.Model(&models.PickHistory{}).Where("course_id = ?", courseID).Count(&pickCount)

	if sessionCount > 0 || attendanceCount > 0 || pickCount > 0 {
		return response.Conflict(
			c,
			response.CodeCourseHasHistory,
			"ไม่สามารถลบรายวิชานี้ได้ เนื่องจากมีประวัติคาบเรียน ข้อมูลการเข้าเรียน หรือประวัติการสุ่มอยู่ในระบบแล้ว",
		)
	}

	if err := h.db.Delete(&models.Course{}, courseID).Error; err != nil {
		if validator.IsForeignKeyRestriction(err) {
			return response.Conflict(
				c,
				response.CodeCourseHasHistory,
				"ไม่สามารถลบรายวิชานี้ได้ เนื่องจากมีประวัติคาบเรียน ข้อมูลการเข้าเรียน หรือประวัติการสุ่มอยู่ในระบบแล้ว",
			)
		}
		return response.InternalError(c, "ไม่สามารถลบรายวิชาได้")
	}

	return response.OK(c, "ลบรายวิชาสำเร็จ", fiber.Map{
		"deletedCourseId": courseID,
	})
}

func (h *CourseHandler) buildCourseListItemDTO(course models.Course) (CourseListItemDTO, error) {
	var enrolledCount int64
	if err := h.db.Model(&models.Enrollment{}).
		Where("course_id = ? AND status = ?", course.ID, models.EnrollmentStatusActive).
		Count(&enrolledCount).Error; err != nil {
		return CourseListItemDTO{}, err
	}

	var totalSessions int64
	if err := h.db.Model(&models.ClassSession{}).
		Where("course_id = ?", course.ID).
		Count(&totalSessions).Error; err != nil {
		return CourseListItemDTO{}, err
	}

	var activeSession models.ClassSession
	var activeID *uint64
	if err := h.db.Select("id").
		Where("course_id = ? AND status = ?", course.ID, models.SessionStatusActive).
		First(&activeSession).Error; err == nil {
		id := activeSession.ID
		activeID = &id
	}

	return CourseListItemDTO{
		ID:              course.ID,
		CourseCode:      course.CourseCode,
		CourseName:      course.CourseName,
		Section:         course.Section,
		Semester:        course.Semester,
		EnrolledCount:   enrolledCount,
		TotalSessions:   totalSessions,
		ActiveSessionID: activeID,
		CreatedAt:       course.CreatedAt,
		UpdatedAt:       course.UpdatedAt,
	}, nil
}

func (h *CourseHandler) checkDuplicateCourse(excludeID uint64, code, name, section, semester *string) (bool, string, error) {
	query := h.db.Model(&models.Course{})
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	if section == nil {
		query = query.Where("section IS NULL")
	} else {
		query = query.Where("section = ?", *section)
	}

	if semester == nil {
		query = query.Where("semester IS NULL")
	} else {
		query = query.Where("semester = ?", *semester)
	}

	if code != nil {
		query = query.Where("course_code = ?", *code)
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, "", err
		}
		if count > 0 {
			secStr := "-"
			if section != nil {
				secStr = *section
			}
			semStr := "-"
			if semester != nil {
				semStr = *semester
			}
			return true, fmt.Sprintf("มีรายวิชารหัส %s กลุ่ม %s ในภาคการศึกษา %s อยู่ในระบบแล้ว", *code, secStr, semStr), nil
		}
		return false, "", nil
	}

	// When course_code is nil, check duplicate (LOWER(BTRIM(course_name)), section, semester)
	if name != nil {
		query = query.Where("course_code IS NULL AND LOWER(BTRIM(course_name)) = ?", strings.ToLower(*name))
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, "", err
		}
		if count > 0 {
			return true, fmt.Sprintf("มีรายวิชาชื่อ %s ในกลุ่มและภาคการศึกษาเดียวกันอยู่ในระบบแล้ว", *name), nil
		}
	}

	return false, "", nil
}

