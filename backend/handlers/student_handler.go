package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/response"
	"github.com/Tewan0/FairPick/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StudentHandler struct {
	db *gorm.DB
}

func NewStudentHandler(db *gorm.DB) *StudentHandler {
	return &StudentHandler{db: db}
}

type CreateStudentRequest struct {
	StudentCode string  `json:"studentCode"`
	FullName    string  `json:"fullName"`
	CourseID    *uint64 `json:"courseId"`
}

type UpdateStudentRequest struct {
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
}

type StudentListItemDTO struct {
	ID                     uint64    `json:"id"`
	StudentCode            string    `json:"studentCode"`
	FullName               string    `json:"fullName"`
	ActiveEnrollmentsCount int64     `json:"activeEnrollmentsCount"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type StudentCreatedDTO struct {
	ID                     uint64    `json:"id"`
	StudentCode            string    `json:"studentCode"`
	FullName               string    `json:"fullName"`
	EnrolledInCourseID     *uint64   `json:"enrolledInCourseId"`
	AddedToActiveSessionID *uint64   `json:"addedToActiveSessionId"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type StudentCourseMembershipDTO struct {
	CourseID         uint64     `json:"courseId"`
	CourseCode       *string    `json:"courseCode"`
	CourseName       *string    `json:"courseName"`
	Section          *string    `json:"section"`
	Semester         *string    `json:"semester"`
	EnrollmentStatus string     `json:"enrollmentStatus"`
	EnrolledAt       time.Time  `json:"enrolledAt"`
	WithdrawnAt      *time.Time `json:"withdrawnAt"`
}

type StudentDetailDTO struct {
	ID          uint64                       `json:"id"`
	StudentCode string                       `json:"studentCode"`
	FullName    string                       `json:"fullName"`
	Courses     []StudentCourseMembershipDTO `json:"courses"`
	CreatedAt   time.Time                    `json:"createdAt"`
	UpdatedAt   time.Time                    `json:"updatedAt"`
}

// ListStudents handles GET /api/students
func (h *StudentHandler) ListStudents(c *fiber.Ctx) error {
	page, limit, offset := validator.ParsePagination(c, 20, 1000)
	search := validator.NormalizeRequiredString(c.Query("search"))

	query := h.db.Model(&models.Student{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("student_code ILIKE ? OR full_name ILIKE ?", like, like)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงจำนวนนักศึกษาได้")
	}

	var students []models.Student
	if err := query.Order("student_code ASC").Limit(limit).Offset(offset).Find(&students).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายชื่อนักศึกษาได้")
	}

	items := make([]StudentListItemDTO, 0, len(students))
	for _, s := range students {
		var activeCount int64
		h.db.Model(&models.Enrollment{}).
			Where("student_id = ? AND status = ?", s.ID, models.EnrollmentStatusActive).
			Count(&activeCount)

		items = append(items, StudentListItemDTO{
			ID:                     s.ID,
			StudentCode:            s.StudentCode,
			FullName:               s.FullName,
			ActiveEnrollmentsCount: activeCount,
			CreatedAt:              s.CreatedAt,
			UpdatedAt:              s.UpdatedAt,
		})
	}

	return response.Paginated(c, "ดึงรายชื่อนักศึกษาสำเร็จ", items, page, limit, totalItems)
}

// CreateStudent handles POST /api/students
func (h *StudentHandler) CreateStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	code := validator.NormalizeRequiredString(req.StudentCode)
	name := validator.NormalizeRequiredString(req.FullName)

	details := validateStudentFields(code, name)
	if len(details) > 0 {
		return response.ValidationError(c, "ข้อมูลนักศึกษาไม่ถูกต้อง", details)
	}

	var existingCount int64
	if err := h.db.Model(&models.Student{}).Where("student_code = ?", code).Count(&existingCount).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถตรวจสอบรหัสนักศึกษาซ้ำได้")
	}
	if existingCount > 0 {
		return response.Conflict(
			c,
			response.CodeDuplicateResource,
			fmt.Sprintf("รหัสนักศึกษา %s มีอยู่ในระบบแล้ว", code),
		)
	}

	if req.CourseID != nil {
		if *req.CourseID == 0 {
			return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
		}
		var course models.Course
		if err := h.db.First(&course, *req.CourseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return response.NotFound(c, "ไม่พบรายวิชาที่ต้องการลงทะเบียน")
			}
			return response.InternalError(c, "ไม่สามารถตรวจสอบรายวิชาได้")
		}
	}

	student := models.Student{
		StudentCode: code,
		FullName:    name,
	}

	var enrolledCourseID *uint64
	var addedActiveSessionID *uint64

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&student).Error; err != nil {
			return err
		}

		if req.CourseID != nil {
			enrollment := models.Enrollment{
				CourseID:  *req.CourseID,
				StudentID: student.ID,
				Status:    models.EnrollmentStatusActive,
			}
			if err := tx.Create(&enrollment).Error; err != nil {
				return err
			}
			cid := *req.CourseID
			enrolledCourseID = &cid

			// If course has an active session, automatically create unconfirmed attendance row
			var activeSession models.ClassSession
			if err := tx.Where("course_id = ? AND status = ?", cid, models.SessionStatusActive).First(&activeSession).Error; err == nil {
				att := models.Attendance{
					SessionID: activeSession.ID,
					CourseID:  cid,
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
		}

		return nil
	})

	if err != nil {
		if validator.IsUniqueViolation(err) {
			return response.Conflict(
				c,
				response.CodeDuplicateResource,
				fmt.Sprintf("รหัสนักศึกษา %s มีอยู่ในระบบแล้ว", code),
			)
		}
		return response.InternalError(c, "ไม่สามารถเพิ่มข้อมูลนักศึกษาได้")
	}

	return response.Created(c, "เพิ่มข้อมูลนักศึกษาสำเร็จ", StudentCreatedDTO{
		ID:                     student.ID,
		StudentCode:            student.StudentCode,
		FullName:               student.FullName,
		EnrolledInCourseID:     enrolledCourseID,
		AddedToActiveSessionID: addedActiveSessionID,
		CreatedAt:              student.CreatedAt,
		UpdatedAt:              student.UpdatedAt,
	})
}

// GetStudent handles GET /api/students/:studentId
func (h *StudentHandler) GetStudent(c *fiber.Ctx) error {
	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสอ้างอิงนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var student models.Student
	if err := h.db.First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลนักศึกษาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลนักศึกษาได้")
	}

	var enrollments []models.Enrollment
	if err := h.db.Preload("Course").Where("student_id = ?", student.ID).Order("id ASC").Find(&enrollments).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลรายวิชาของนักศึกษาได้")
	}

	courses := make([]StudentCourseMembershipDTO, 0, len(enrollments))
	for _, e := range enrollments {
		if e.Course == nil {
			continue
		}
		courses = append(courses, StudentCourseMembershipDTO{
			CourseID:         e.CourseID,
			CourseCode:       e.Course.CourseCode,
			CourseName:       e.Course.CourseName,
			Section:          e.Course.Section,
			Semester:         e.Course.Semester,
			EnrollmentStatus: e.Status,
			EnrolledAt:       e.EnrolledAt,
			WithdrawnAt:      e.WithdrawnAt,
		})
	}

	return response.OK(c, "ดึงข้อมูลนักศึกษาสำเร็จ", StudentDetailDTO{
		ID:          student.ID,
		StudentCode: student.StudentCode,
		FullName:    student.FullName,
		Courses:     courses,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	})
}

// UpdateStudent handles PUT /api/students/:studentId
func (h *StudentHandler) UpdateStudent(c *fiber.Ctx) error {
	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสอ้างอิงนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var student models.Student
	if err := h.db.First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลนักศึกษาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลนักศึกษาได้")
	}

	var req UpdateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
	}

	code := validator.NormalizeRequiredString(req.StudentCode)
	name := validator.NormalizeRequiredString(req.FullName)

	details := validateStudentFields(code, name)
	if len(details) > 0 {
		return response.ValidationError(c, "ข้อมูลนักศึกษาไม่ถูกต้อง", details)
	}

	var dupCount int64
	if err := h.db.Model(&models.Student{}).
		Where("student_code = ? AND id <> ?", code, student.ID).
		Count(&dupCount).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถตรวจสอบรหัสนักศึกษาซ้ำได้")
	}
	if dupCount > 0 {
		return response.Conflict(
			c,
			response.CodeDuplicateResource,
			fmt.Sprintf("รหัสนักศึกษา %s มีอยู่ในระบบแล้ว", code),
		)
	}

	student.StudentCode = code
	student.FullName = name
	student.UpdatedAt = time.Now()

	if err := h.db.Model(&student).Select("student_code", "full_name", "updated_at").Updates(&student).Error; err != nil {
		if validator.IsUniqueViolation(err) {
			return response.Conflict(
				c,
				response.CodeDuplicateResource,
				fmt.Sprintf("รหัสนักศึกษา %s มีอยู่ในระบบแล้ว", code),
			)
		}
		return response.InternalError(c, "ไม่สามารถแก้ไขข้อมูลนักศึกษาได้")
	}

	return response.OK(c, "แก้ไขข้อมูลนักศึกษาสำเร็จ", fiber.Map{
		"id":          student.ID,
		"studentCode": student.StudentCode,
		"fullName":    student.FullName,
		"createdAt":   student.CreatedAt,
		"updatedAt":   student.UpdatedAt,
	})
}

// DeleteStudent handles DELETE /api/students/:studentId
func (h *StudentHandler) DeleteStudent(c *fiber.Ctx) error {
	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสอ้างอิงนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var student models.Student
	if err := h.db.First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลนักศึกษาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลนักศึกษาได้")
	}

	var enrollCount, attCount, pickCount int64
	h.db.Model(&models.Enrollment{}).Where("student_id = ?", studentID).Count(&enrollCount)
	h.db.Model(&models.Attendance{}).Where("student_id = ?", studentID).Count(&attCount)
	h.db.Model(&models.PickHistory{}).Where("student_id = ?", studentID).Count(&pickCount)

	if enrollCount > 0 || attCount > 0 || pickCount > 0 {
		return response.Conflict(
			c,
			response.CodeStudentHasHistory,
			"ไม่สามารถลบข้อมูลนักศึกษาได้ เนื่องจากมีประวัติการลงทะเบียนหรือประวัติการเรียนในระบบ (กรุณาใช้การถอนรายวิชาแทน)",
		)
	}

	if err := h.db.Delete(&models.Student{}, studentID).Error; err != nil {
		if validator.IsForeignKeyRestriction(err) {
			return response.Conflict(
				c,
				response.CodeStudentHasHistory,
				"ไม่สามารถลบข้อมูลนักศึกษาได้ เนื่องจากมีประวัติการลงทะเบียนหรือประวัติการเรียนในระบบ (กรุณาใช้การถอนรายวิชาแทน)",
			)
		}
		return response.InternalError(c, "ไม่สามารถลบข้อมูลนักศึกษาได้")
	}

	return response.OK(c, "ลบข้อมูลนักศึกษาสำเร็จ", fiber.Map{
		"deletedStudentId": studentID,
	})
}

func validateStudentFields(studentCode, fullName string) map[string]string {
	details := make(map[string]string)
	if studentCode == "" {
		details["studentCode"] = "กรุณาระบุรหัสนักศึกษา"
	} else if len(studentCode) > 30 {
		details["studentCode"] = "รหัสนักศึกษาต้องไม่เกิน 30 ตัวอักษร"
	}

	if fullName == "" {
		details["fullName"] = "กรุณาระบุชื่อ-นามสกุลนักศึกษา"
	} else if len(fullName) > 255 {
		details["fullName"] = "ชื่อ-นามสกุลต้องไม่เกิน 255 ตัวอักษร"
	}
	return details
}

