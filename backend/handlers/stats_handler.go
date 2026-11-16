package handlers

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/response"
	"github.com/Tewan0/FairPick/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type StatsHandler struct {
	db *gorm.DB
}

func NewStatsHandler(db *gorm.DB) *StatsHandler {
	return &StatsHandler{db: db}
}

type StatsCourseBriefDTO struct {
	ID         uint64  `json:"id"`
	CourseCode *string `json:"courseCode"`
	CourseName *string `json:"courseName"`
	Section    *string `json:"section"`
	Semester   *string `json:"semester"`
}

type CourseOverallStatsDTO struct {
	TotalActiveStudents          int64   `json:"totalActiveStudents"`
	TotalWithdrawnStudents       int64   `json:"totalWithdrawnStudents"`
	TotalSessions                int64   `json:"totalSessions"`
	AverageAttendanceRatePercent float64 `json:"averageAttendanceRatePercent"`
	TotalValidPicks              int64   `json:"totalValidPicks"`
	TotalCancelledPicks          int64   `json:"totalCancelledPicks"`
	WeightedValidPicks           int64   `json:"weightedValidPicks"`
	NoRepeatValidPicks           int64   `json:"noRepeatValidPicks"`
	StudentsNeverPickedCount     int64   `json:"studentsNeverPickedCount"`
}

type SessionStatBreakdownDTO struct {
	SessionID             uint64    `json:"sessionId"`
	SessionNumber         int       `json:"sessionNumber"`
	Topic                 *string   `json:"topic"`
	StartedAt             time.Time `json:"startedAt"`
	Status                string    `json:"status"`
	PresentCount          int64     `json:"presentCount"`
	LateCount             int64     `json:"lateCount"`
	AbsentCount           int64     `json:"absentCount"`
	UnconfirmedCount      int64     `json:"unconfirmedCount"`
	ConfirmedCount        int64     `json:"confirmedCount"`
	AttendanceRatePercent float64   `json:"attendanceRatePercent"`
	ValidPicksCount       int64     `json:"validPicksCount"`
}

type CourseOverviewResponseDTO struct {
	Course           StatsCourseBriefDTO       `json:"course"`
	Overall          CourseOverallStatsDTO     `json:"overall"`
	SessionBreakdown []SessionStatBreakdownDTO `json:"sessionBreakdown"`
}

type StudentAttendanceStatDTO struct {
	TotalSessions          int64   `json:"totalSessions"`
	ConfirmedSessionsCount int64   `json:"confirmedSessionsCount"`
	PresentCount           int64   `json:"presentCount"`
	LateCount              int64   `json:"lateCount"`
	AbsentCount            int64   `json:"absentCount"`
	UnconfirmedCount       int64   `json:"unconfirmedCount"`
	AttendedCount          int64   `json:"attendedCount"`
	AttendanceRatePercent  float64 `json:"attendanceRatePercent"`
}

type StudentPickStatDTO struct {
	TotalValidPicks          int64      `json:"totalValidPicks"`
	WeightedValidPicks       int64      `json:"weightedValidPicks"`
	NoRepeatValidPicks       int64      `json:"noRepeatValidPicks"`
	CancelledPicks           int64      `json:"cancelledPicks"`
	Recent4SessionsPickCount int64      `json:"recent4SessionsPickCount"`
	LastPickedAt             *time.Time `json:"lastPickedAt"`
}

type StudentCourseStatItemDTO struct {
	StudentID        uint64                   `json:"studentId"`
	StudentCode      string                   `json:"studentCode"`
	FullName         string                   `json:"fullName"`
	EnrollmentStatus string                   `json:"enrollmentStatus"`
	Attendance       StudentAttendanceStatDTO `json:"attendance"`
	Picks            StudentPickStatDTO       `json:"picks"`
}

type StudentEnrollmentDetailDTO struct {
	ID               uint64     `json:"id"`
	StudentCode      string     `json:"studentCode"`
	FullName         string     `json:"fullName"`
	EnrollmentStatus string     `json:"enrollmentStatus"`
	EnrolledAt       time.Time  `json:"enrolledAt"`
	WithdrawnAt      *time.Time `json:"withdrawnAt"`
}

type StudentDetailPickSummaryDTO struct {
	TotalValidPicks    int64 `json:"totalValidPicks"`
	WeightedValidPicks int64 `json:"weightedValidPicks"`
	NoRepeatValidPicks int64 `json:"noRepeatValidPicks"`
	CancelledPicks     int64 `json:"cancelledPicks"`
}

type StudentAttendanceHistoryItemDTO struct {
	SessionID     uint64     `json:"sessionId"`
	SessionNumber int        `json:"sessionNumber"`
	Topic         *string    `json:"topic"`
	StartedAt     time.Time  `json:"startedAt"`
	Status        string     `json:"status"`
	CheckedAt     *time.Time `json:"checkedAt"`
	Note          *string    `json:"note"`
}

type StudentPickHistoryItemDTO struct {
	PickID        uint64     `json:"pickId"`
	SessionID     uint64     `json:"sessionId"`
	SessionNumber int        `json:"sessionNumber"`
	Mode          string     `json:"mode"`
	RoundNumber   int        `json:"roundNumber"`
	Status        string     `json:"status"`
	WeightScore   *float64   `json:"weightScore"`
	PickedAt      time.Time  `json:"pickedAt"`
	CancelledAt   *time.Time `json:"cancelledAt"`
	CancelReason  *string    `json:"cancelReason"`
}

type StudentCourseDetailStatsResponseDTO struct {
	Course            StatsCourseBriefDTO               `json:"course"`
	Student           StudentEnrollmentDetailDTO        `json:"student"`
	AttendanceSummary StudentAttendanceStatDTO          `json:"attendanceSummary"`
	PickSummary       StudentDetailPickSummaryDTO       `json:"pickSummary"`
	AttendanceHistory []StudentAttendanceHistoryItemDTO `json:"attendanceHistory"`
	PickHistory       []StudentPickHistoryItemDTO       `json:"pickHistory"`
}

func computeAttendanceRate(present, late, absent int64) (attended, confirmed int64, rate float64) {
	attended = present + late
	confirmed = present + late + absent
	if confirmed == 0 {
		return attended, 0, 0.0
	}
	rate = round2((float64(attended) / float64(confirmed)) * 100.0)
	return attended, confirmed, rate
}

// GetCourseOverviewStats handles GET /api/courses/:courseId/stats/overview
func (h *StatsHandler) GetCourseOverviewStats(c *fiber.Ctx) error {
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

	var activeStudents, withdrawnStudents int64
	if err := h.db.Model(&models.Enrollment{}).
		Where("course_id = ? AND status = ?", courseID, models.EnrollmentStatusActive).
		Count(&activeStudents).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนนักศึกษาได้")
	}
	if err := h.db.Model(&models.Enrollment{}).
		Where("course_id = ? AND status = ?", courseID, models.EnrollmentStatusWithdrawn).
		Count(&withdrawnStudents).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนนักศึกษาที่ถอนได้")
	}

	var sessions []models.ClassSession
	if err := h.db.Where("course_id = ?", courseID).
		Order("session_number DESC").
		Find(&sessions).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายการคาบเรียนได้")
	}

	type sessionStatusAgg struct {
		SessionID uint64
		Status    string
		Count     int64
	}
	var attAggs []sessionStatusAgg
	if err := h.db.Model(&models.Attendance{}).
		Select("session_id, status, COUNT(*) AS count").
		Where("course_id = ?", courseID).
		Group("session_id, status").
		Scan(&attAggs).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการเข้าเรียนรายคาบได้")
	}

	type sessionPickAgg struct {
		SessionID uint64
		Count     int64
	}
	var pickAggs []sessionPickAgg
	if err := h.db.Model(&models.PickHistory{}).
		Select("session_id, COUNT(*) AS count").
		Where("course_id = ? AND status = ?", courseID, models.PickStatusValid).
		Group("session_id").
		Scan(&pickAggs).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลการสุ่มรายคาบได้")
	}

	attBySession := make(map[uint64]map[string]int64, len(sessions))
	for _, a := range attAggs {
		if attBySession[a.SessionID] == nil {
			attBySession[a.SessionID] = make(map[string]int64)
		}
		attBySession[a.SessionID][a.Status] = a.Count
	}

	validPicksBySession := make(map[uint64]int64, len(sessions))
	for _, p := range pickAggs {
		validPicksBySession[p.SessionID] = p.Count
	}

	var endedPresent, endedLate, endedAbsent int64
	breakdown := make([]SessionStatBreakdownDTO, 0, len(sessions))

	for _, s := range sessions {
		counts := attBySession[s.ID]
		present := counts[models.AttendanceStatusPresent]
		late := counts[models.AttendanceStatusLate]
		absent := counts[models.AttendanceStatusAbsent]
		unconfirmed := counts[models.AttendanceStatusUnconfirmed]
		_, confirmed, rate := computeAttendanceRate(present, late, absent)

		if s.Status == models.SessionStatusEnded {
			endedPresent += present
			endedLate += late
			endedAbsent += absent
		}

		breakdown = append(breakdown, SessionStatBreakdownDTO{
			SessionID:             s.ID,
			SessionNumber:         s.SessionNumber,
			Topic:                 s.Topic,
			StartedAt:             s.StartedAt,
			Status:                s.Status,
			PresentCount:          present,
			LateCount             : late,
			AbsentCount:           absent,
			UnconfirmedCount:      unconfirmed,
			ConfirmedCount:        confirmed,
			AttendanceRatePercent: rate,
			ValidPicksCount:       validPicksBySession[s.ID],
		})
	}

	_, _, avgEndedAttendanceRate := computeAttendanceRate(endedPresent, endedLate, endedAbsent)

	var weightedValid, noRepeatValid, totalCancelled int64
	if err := h.db.Model(&models.PickHistory{}).
		Where("course_id = ? AND status = ? AND mode = ?", courseID, models.PickStatusValid, models.PickModeWeighted).
		Count(&weightedValid).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนการสุ่มโหมด weighted ได้")
	}
	if err := h.db.Model(&models.PickHistory{}).
		Where("course_id = ? AND status = ? AND mode = ?", courseID, models.PickStatusValid, models.PickModeNoRepeat).
		Count(&noRepeatValid).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนการสุ่มโหมด no_repeat ได้")
	}
	if err := h.db.Model(&models.PickHistory{}).
		Where("course_id = ? AND status = ?", courseID, models.PickStatusCancelled).
		Count(&totalCancelled).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนการสุ่มที่ยกเลิกได้")
	}

	var neverPickedCount int64
	err = h.db.Model(&models.Enrollment{}).
		Where("enrollments.course_id = ? AND enrollments.status = ?", courseID, models.EnrollmentStatusActive).
		Where(`NOT EXISTS (
			SELECT 1 FROM pick_history ph
			WHERE ph.course_id = enrollments.course_id
			  AND ph.student_id = enrollments.student_id
			  AND ph.status = ?
		)`, models.PickStatusValid).
		Count(&neverPickedCount).Error
	if err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนนักศึกษาที่ยังไม่เคยถูกสุ่มได้")
	}

	return response.OK(c, "ดึงสถิติภาพรวมของรายวิชาสำเร็จ", CourseOverviewResponseDTO{
		Course: StatsCourseBriefDTO{
			ID:         course.ID,
			CourseCode: course.CourseCode,
			CourseName: course.CourseName,
			Section:    course.Section,
			Semester:   course.Semester,
		},
		Overall: CourseOverallStatsDTO{
			TotalActiveStudents:          activeStudents,
			TotalWithdrawnStudents:       withdrawnStudents,
			TotalSessions:                int64(len(sessions)),
			AverageAttendanceRatePercent: avgEndedAttendanceRate,
			TotalValidPicks:              weightedValid + noRepeatValid,
			TotalCancelledPicks:          totalCancelled,
			WeightedValidPicks:           weightedValid,
			NoRepeatValidPicks:           noRepeatValid,
			StudentsNeverPickedCount:     neverPickedCount,
		},
		SessionBreakdown: breakdown,
	})
}

// ListCourseStudentStats handles GET /api/courses/:courseId/stats/students
func (h *StatsHandler) ListCourseStudentStats(c *fiber.Ctx) error {
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

	enrollmentStatus := strings.ToLower(validator.NormalizeRequiredString(c.Query("enrollmentStatus")))
	if enrollmentStatus == "" {
		enrollmentStatus = models.EnrollmentStatusActive
	}
	if enrollmentStatus != models.EnrollmentStatusActive &&
		enrollmentStatus != models.EnrollmentStatusWithdrawn &&
		enrollmentStatus != "all" {
		return response.ValidationError(c, "พารามิเตอร์ enrollmentStatus ไม่ถูกต้อง", map[string]string{
			"enrollmentStatus": "อนุญาตเฉพาะ active, withdrawn หรือ all เท่านั้น",
		})
	}

	sortBy := validator.NormalizeRequiredString(c.Query("sortBy"))
	if sortBy == "" {
		sortBy = "studentCode"
	}
	if sortBy != "studentCode" && sortBy != "attendanceRate" && sortBy != "validPickCount" {
		return response.ValidationError(c, "พารามิเตอร์ sortBy ไม่ถูกต้อง", map[string]string{
			"sortBy": "อนุญาตเฉพาะ studentCode, attendanceRate หรือ validPickCount เท่านั้น",
		})
	}

	sortOrder := strings.ToLower(validator.NormalizeRequiredString(c.Query("sortOrder")))
	if sortOrder == "" {
		sortOrder = "asc"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		return response.ValidationError(c, "พารามิเตอร์ sortOrder ไม่ถูกต้อง", map[string]string{
			"sortOrder": "อนุญาตเฉพาะ asc หรือ desc เท่านั้น",
		})
	}

	search := validator.NormalizeRequiredString(c.Query("search"))

	type enrolledStudentRow struct {
		StudentID        uint64
		StudentCode      string
		FullName         string
		EnrollmentStatus string
	}

	enrQuery := h.db.Table("enrollments").
		Select("enrollments.student_id, students.student_code, students.full_name, enrollments.status AS enrollment_status").
		Joins("JOIN students ON students.id = enrollments.student_id").
		Where("enrollments.course_id = ?", courseID)

	if enrollmentStatus != "all" {
		enrQuery = enrQuery.Where("enrollments.status = ?", enrollmentStatus)
	}
	if search != "" {
		like := validator.FormatSearchLike(search)
		enrQuery = enrQuery.Where("(students.student_code ILIKE ? OR students.full_name ILIKE ?)", like, like)
	}

	var students []enrolledStudentRow
	if err := enrQuery.Order("students.student_code ASC").Scan(&students).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายชื่อนักศึกษาในรายวิชาได้")
	}

	if len(students) == 0 {
		return response.OK(c, "ดึงสถิตินักศึกษาในรายวิชาสำเร็จ", []StudentCourseStatItemDTO{})
	}

	// Official attendance statistics count only ended sessions where the student was enrolled (has an attendance record)
	type studentAttAgg struct {
		StudentID uint64
		Status    string
		Count     int64
	}
	var attAggs []studentAttAgg
	if err := h.db.Table("attendance").
		Select("attendance.student_id, attendance.status, COUNT(*) AS count").
		Joins("JOIN class_sessions ON class_sessions.id = attendance.session_id").
		Where("attendance.course_id = ? AND class_sessions.status = ?", courseID, models.SessionStatusEnded).
		Group("attendance.student_id, attendance.status").
		Scan(&attAggs).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถคำนวณสถิติการเข้าเรียนของนักศึกษาได้")
	}

	attByStudent := make(map[uint64]map[string]int64, len(students))
	for _, a := range attAggs {
		if attByStudent[a.StudentID] == nil {
			attByStudent[a.StudentID] = make(map[string]int64)
		}
		attByStudent[a.StudentID][a.Status] = a.Count
	}

	// Pick statistics per student in this course
	type studentPickAgg struct {
		StudentID          uint64
		WeightedValidPicks int64
		NoRepeatValidPicks int64
		CancelledPicks     int64
		LastPickedAt       *time.Time
	}
	var pickAggs []studentPickAgg
	err = h.db.Table("pick_history").
		Select(`student_id,
			SUM(CASE WHEN status = 'valid' AND mode = 'weighted' THEN 1 ELSE 0 END) AS weighted_valid_picks,
			SUM(CASE WHEN status = 'valid' AND mode = 'no_repeat' THEN 1 ELSE 0 END) AS no_repeat_valid_picks,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) AS cancelled_picks,
			MAX(CASE WHEN status = 'valid' THEN picked_at ELSE NULL END) AS last_picked_at`).
		Where("course_id = ?", courseID).
		Group("student_id").
		Scan(&pickAggs).Error
	if err != nil {
		return response.InternalError(c, "ไม่สามารถคำนวณสถิติการถูกสุ่มของนักศึกษาได้")
	}

	picksByStudent := make(map[uint64]studentPickAgg, len(pickAggs))
	for _, p := range pickAggs {
		picksByStudent[p.StudentID] = p
	}

	// Recent 4 sessions in this course
	var recent4SessionIDs []uint64
	if err := h.db.Model(&models.ClassSession{}).
		Where("course_id = ?", courseID).
		Order("session_number DESC").
		Limit(4).
		Pluck("id", &recent4SessionIDs).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงข้อมูล 4 คาบล่าสุดได้")
	}

	recent4ByStudent := make(map[uint64]int64)
	if len(recent4SessionIDs) > 0 {
		type recentCountRow struct {
			StudentID uint64
			Count     int64
		}
		var recentRows []recentCountRow
		if err := h.db.Model(&models.PickHistory{}).
			Select("student_id, COUNT(*) AS count").
			Where("course_id = ? AND session_id IN ? AND status = ?", courseID, recent4SessionIDs, models.PickStatusValid).
			Group("student_id").
			Scan(&recentRows).Error; err != nil {
			return response.InternalError(c, "ไม่สามารถคำนวณสถิติการสุ่ม 4 คาบล่าสุดได้")
		}
		for _, r := range recentRows {
			recent4ByStudent[r.StudentID] = r.Count
		}
	}

	items := make([]StudentCourseStatItemDTO, 0, len(students))
	for _, s := range students {
		ac := attByStudent[s.StudentID]
		present := ac[models.AttendanceStatusPresent]
		late := ac[models.AttendanceStatusLate]
		absent := ac[models.AttendanceStatusAbsent]
		unconfirmed := ac[models.AttendanceStatusUnconfirmed]
		attended, confirmed, rate := computeAttendanceRate(present, late, absent)
		totalStudentSessions := present + late + absent + unconfirmed

		pk := picksByStudent[s.StudentID]
		totalValid := pk.WeightedValidPicks + pk.NoRepeatValidPicks

		items = append(items, StudentCourseStatItemDTO{
			StudentID:        s.StudentID,
			StudentCode:      s.StudentCode,
			FullName:         s.FullName,
			EnrollmentStatus: s.EnrollmentStatus,
			Attendance: StudentAttendanceStatDTO{
				TotalSessions:          totalStudentSessions,
				ConfirmedSessionsCount: confirmed,
				PresentCount:           present,
				LateCount:              late,
				AbsentCount:            absent,
				UnconfirmedCount:       unconfirmed,
				AttendedCount:          attended,
				AttendanceRatePercent:  rate,
			},
			Picks: StudentPickStatDTO{
				TotalValidPicks:          totalValid,
				WeightedValidPicks:       pk.WeightedValidPicks,
				NoRepeatValidPicks:       pk.NoRepeatValidPicks,
				CancelledPicks:           pk.CancelledPicks,
				Recent4SessionsPickCount: recent4ByStudent[s.StudentID],
				LastPickedAt:             pk.LastPickedAt,
			},
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "attendanceRate":
			if items[i].Attendance.AttendanceRatePercent == items[j].Attendance.AttendanceRatePercent {
				return items[i].StudentCode < items[j].StudentCode
			}
			less = items[i].Attendance.AttendanceRatePercent < items[j].Attendance.AttendanceRatePercent
		case "validPickCount":
			if items[i].Picks.TotalValidPicks == items[j].Picks.TotalValidPicks {
				return items[i].StudentCode < items[j].StudentCode
			}
			less = items[i].Picks.TotalValidPicks < items[j].Picks.TotalValidPicks
		default:
			less = items[i].StudentCode < items[j].StudentCode
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})

	return response.OK(c, "ดึงสถิตินักศึกษาในรายวิชาสำเร็จ", items)
}

// GetStudentCourseStats handles GET /api/courses/:courseId/stats/students/:studentId
func (h *StatsHandler) GetStudentCourseStats(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	studentID, err := validator.ParseUintParam(c, "studentId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสนักศึกษา (studentId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลรายวิชาได้")
	}

	var student models.Student
	if err := h.db.First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลนักศึกษาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลนักศึกษาได้")
	}

	var enrollment models.Enrollment
	if err := h.db.Where("course_id = ? AND student_id = ?", courseID, studentID).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลการลงทะเบียนของนักศึกษาในรายวิชานี้")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลการลงทะเบียนได้")
	}

	// Query attendance history in this course joined with class_sessions
	type attHistoryRow struct {
		SessionID     uint64
		SessionNumber int
		Topic         *string
		StartedAt     time.Time
		SessionStatus string
		Status        string
		CheckedAt     *time.Time
		Note          *string
	}
	var attRows []attHistoryRow
	err = h.db.Table("attendance").
		Select(`attendance.session_id,
			class_sessions.session_number,
			class_sessions.topic,
			class_sessions.started_at,
			class_sessions.status AS session_status,
			attendance.status,
			attendance.checked_at,
			attendance.note`).
		Joins("JOIN class_sessions ON class_sessions.id = attendance.session_id").
		Where("attendance.course_id = ? AND attendance.student_id = ?", courseID, studentID).
		Order("class_sessions.session_number DESC").
		Scan(&attRows).Error
	if err != nil {
		return response.InternalError(c, "ไม่สามารถดึงประวัติการเข้าเรียนของนักศึกษาได้")
	}

	var present, late, absent, unconfirmed int64
	attHistory := make([]StudentAttendanceHistoryItemDTO, 0, len(attRows))
	for _, r := range attRows {
		// Official attendance summary counts only ended sessions where the student had an attendance record
		if r.SessionStatus == models.SessionStatusEnded {
			switch r.Status {
			case models.AttendanceStatusPresent:
				present++
			case models.AttendanceStatusLate:
				late++
			case models.AttendanceStatusAbsent:
				absent++
			case models.AttendanceStatusUnconfirmed:
				unconfirmed++
			}
		}

		attHistory = append(attHistory, StudentAttendanceHistoryItemDTO{
			SessionID:     r.SessionID,
			SessionNumber: r.SessionNumber,
			Topic:         r.Topic,
			StartedAt:     r.StartedAt,
			Status:        r.Status,
			CheckedAt:     r.CheckedAt,
			Note:          r.Note,
		})
	}

	attended, confirmed, rate := computeAttendanceRate(present, late, absent)
	totalEndedSessionsForStudent := present + late + absent + unconfirmed

	// Query pick history in this course for this student
	var pickRows []StudentPickHistoryItemDTO
	err = h.db.Table("pick_history").
		Select(`pick_history.id AS pick_id,
			pick_history.session_id,
			class_sessions.session_number,
			pick_history.mode,
			pick_history.round_number,
			pick_history.status,
			pick_history.weight_score,
			pick_history.picked_at,
			pick_history.cancelled_at,
			pick_history.cancel_reason`).
		Joins("JOIN class_sessions ON class_sessions.id = pick_history.session_id").
		Where("pick_history.course_id = ? AND pick_history.student_id = ?", courseID, studentID).
		Order("pick_history.picked_at DESC, pick_history.id DESC").
		Scan(&pickRows).Error
	if err != nil {
		return response.InternalError(c, "ไม่สามารถดึงประวัติการถูกสุ่มของนักศึกษาได้")
	}
	if pickRows == nil {
		pickRows = []StudentPickHistoryItemDTO{}
	}

	var weightedValid, noRepeatValid, cancelledPicks int64
	for _, p := range pickRows {
		if p.Status == models.PickStatusValid {
			if p.Mode == models.PickModeWeighted {
				weightedValid++
			} else if p.Mode == models.PickModeNoRepeat {
				noRepeatValid++
			}
		} else if p.Status == models.PickStatusCancelled {
			cancelledPicks++
		}
	}

	return response.OK(c, "ดึงสถิติรายบุคคลสำเร็จ", StudentCourseDetailStatsResponseDTO{
		Course: StatsCourseBriefDTO{
			ID:         course.ID,
			CourseCode: course.CourseCode,
			CourseName: course.CourseName,
			Section:    course.Section,
			Semester:   course.Semester,
		},
		Student: StudentEnrollmentDetailDTO{
			ID:               student.ID,
			StudentCode:      student.StudentCode,
			FullName:         student.FullName,
			EnrollmentStatus: enrollment.Status,
			EnrolledAt:       enrollment.EnrolledAt,
			WithdrawnAt:      enrollment.WithdrawnAt,
		},
		AttendanceSummary: StudentAttendanceStatDTO{
			TotalSessions:          totalEndedSessionsForStudent,
			ConfirmedSessionsCount: confirmed,
			PresentCount:           present,
			LateCount:              late,
			AbsentCount:            absent,
			UnconfirmedCount:       unconfirmed,
			AttendedCount:          attended,
			AttendanceRatePercent:  rate,
		},
		PickSummary: StudentDetailPickSummaryDTO{
			TotalValidPicks:    weightedValid + noRepeatValid,
			WeightedValidPicks: weightedValid,
			NoRepeatValidPicks: noRepeatValid,
			CancelledPicks:     cancelledPicks,
		},
		AttendanceHistory: attHistory,
		PickHistory:       pickRows,
	})
}

