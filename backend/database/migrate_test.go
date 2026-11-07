package database_test

import (
	"fmt"
	"testing"

	"github.com/Tewan0/FairPick/database"
	"github.com/Tewan0/FairPick/models"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, _, err := database.ConnectTestDB()
	if err != nil {
		t.Fatalf("failed to connect and migrate test database: %v", err)
	}
	t.Cleanup(func() {
		database.CloseTestDB(db)
	})
	if err := database.ResetTestData(db); err != nil {
		t.Fatalf("failed to reset test database: %v", err)
	}
	return db
}

func strPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func assertExecFails(t *testing.T, tx *gorm.DB, savepoint string, fn func(tx *gorm.DB) error, description string) {
	t.Helper()
	if err := tx.SavePoint(savepoint).Error; err != nil {
		t.Fatalf("failed to create savepoint %s: %v", savepoint, err)
	}
	err := fn(tx)
	if err == nil {
		t.Fatalf("expected error for [%s], but operation succeeded", description)
	}
	if rbErr := tx.RollbackTo(savepoint).Error; rbErr != nil {
		t.Fatalf("failed to rollback to savepoint %s: %v", savepoint, rbErr)
	}
	t.Logf("PASS [%s]: rejected as expected (%v)", description, err)
}

func TestMigrateIdempotencyAndVerifySchema(t *testing.T) {
	db := setupTestDB(t)

	// Re-run Migrate a second time to verify idempotency
	if err := database.Migrate(db); err != nil {
		t.Fatalf("second Migrate() call failed (not idempotent): %v", err)
	}

	// Verify all 6 tables exist in public schema
	expectedTables := []string{
		"courses",
		"students",
		"enrollments",
		"class_sessions",
		"attendance",
		"pick_history",
	}

	for _, table := range expectedTables {
		var exists bool
		err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = ?
			)`, table).Scan(&exists).Error
		if err != nil {
			t.Fatalf("failed checking table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist in database", table)
		}
		t.Logf("Verified table exists: %s", table)
	}

	// Verify all expected CHECK and FOREIGN KEY constraints exist
	expectedConstraints := []string{
		"chk_courses_no_blank_strings",
		"chk_courses_code_or_name",
		"chk_courses_code_format",
		"chk_students_not_empty",
		"chk_enrollments_status",
		"fk_enrollments_course",
		"fk_enrollments_student",
		"chk_class_sessions_status",
		"chk_class_sessions_mode",
		"chk_class_sessions_round",
		"chk_class_sessions_topic_not_blank",
		"fk_class_sessions_course",
		"chk_attendance_status",
		"chk_attendance_note_not_blank",
		"fk_attendance_session_course",
		"fk_attendance_enrollment",
		"chk_pick_history_mode",
		"chk_pick_history_status",
		"chk_pick_history_round",
		"chk_pick_history_reason_not_blank",
		"fk_pick_history_session_course",
		"fk_pick_history_enrollment",
	}

	for _, con := range expectedConstraints {
		var exists bool
		err := db.Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = ?)`, con).Scan(&exists).Error
		if err != nil {
			t.Fatalf("failed checking constraint %s: %v", con, err)
		}
		if !exists {
			t.Fatalf("expected constraint %s to exist in pg_constraint", con)
		}
		t.Logf("Verified constraint exists: %s", con)
	}

	// Verify all expected Unique, Partial Unique, and Performance Indexes exist
	expectedIndexes := []string{
		"uq_courses_code_section_semester",
		"uq_courses_name_section_semester_no_code",
		"uq_students_student_code",
		"idx_students_full_name",
		"uq_enrollments_course_student",
		"idx_enrollments_course_status",
		"idx_enrollments_student_id",
		"uq_class_sessions_id_course",
		"uq_class_sessions_course_number",
		"idx_class_sessions_course_number_desc",
		"uq_class_sessions_one_active_per_course",
		"uq_attendance_session_student",
		"idx_attendance_session_eligible",
		"idx_attendance_course_student",
		"uq_pick_history_no_repeat_valid",
		"idx_pick_history_course_session_valid",
		"idx_pick_history_session_picked_at",
	}

	for _, idx := range expectedIndexes {
		var exists bool
		err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'public' AND indexname = ?
			)`, idx).Scan(&exists).Error
		if err != nil {
			t.Fatalf("failed checking index %s: %v", idx, err)
		}
		if !exists {
			t.Fatalf("expected index %s to exist in pg_indexes", idx)
		}
		t.Logf("Verified index exists: %s", idx)
	}
}

func TestDatabaseConstraintsAndRelationships(t *testing.T) {
	db := setupTestDB(t)

	// Run all data tests inside a transaction and rollback at the end so the database stays clean
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("failed to begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	// -------------------------------------------------------------------------
	// 1. Test Course Constraints
	// -------------------------------------------------------------------------
	courseA := models.Course{
		CourseCode: strPtr("001101"),
		CourseName: strPtr("Fundamental English 1"),
		Section:    strPtr("001"),
		Semester:   strPtr("1/2569"),
	}
	if err := tx.Create(&courseA).Error; err != nil {
		t.Fatalf("failed to create valid courseA: %v", err)
	}
	if courseA.CourseCode == nil || *courseA.CourseCode != "001101" {
		t.Fatalf("expected leading zero in course_code '001101' to be preserved")
	}

	// Same course_code, different section -> must succeed
	courseA2 := models.Course{
		CourseCode: strPtr("001101"),
		CourseName: strPtr("Fundamental English 1"),
		Section:    strPtr("002"),
		Semester:   strPtr("1/2569"),
	}
	if err := tx.Create(&courseA2).Error; err != nil {
		t.Fatalf("expected same course_code with different section to succeed, got: %v", err)
	}

	// Duplicate (course_code, section, semester) even with different course_name -> must fail uq_courses_code_section_semester
	assertExecFails(t, tx, "sp_dup_course_code", func(tx *gorm.DB) error {
		dup := models.Course{
			CourseCode: strPtr("001101"),
			CourseName: strPtr("Another English Title"),
			Section:    strPtr("001"),
			Semester:   strPtr("1/2569"),
		}
		return tx.Create(&dup).Error
	}, "duplicate (course_code, section, semester) with different course_name")

	// Course with neither course_code nor course_name -> must fail chk_courses_code_or_name
	assertExecFails(t, tx, "sp_empty_course", func(tx *gorm.DB) error {
		invalid := models.Course{
			Section:  strPtr("001"),
			Semester: strPtr("1/2569"),
		}
		return tx.Create(&invalid).Error
	}, "course with neither course_code nor course_name")

	// Course with invalid course_code format (not 6 digits) -> must fail chk_courses_code_format
	assertExecFails(t, tx, "sp_bad_course_code", func(tx *gorm.DB) error {
		invalid := models.Course{
			CourseCode: strPtr("12345"),
			CourseName: strPtr("Invalid Code Course"),
		}
		return tx.Create(&invalid).Error
	}, "course with 5-digit course_code")

	// Course with blank string "" -> must fail chk_courses_no_blank_strings
	assertExecFails(t, tx, "sp_blank_course_str", func(tx *gorm.DB) error {
		invalid := models.Course{
			CourseCode: strPtr("001102"),
			CourseName: strPtr("   "),
		}
		return tx.Create(&invalid).Error
	}, "course with whitespace-only course_name")

	// -------------------------------------------------------------------------
	// 2. Test Student & Enrollment Constraints
	// -------------------------------------------------------------------------
	student1 := models.Student{
		StudentCode: "660510001",
		FullName:    "กิตติพงศ์ ใจดี",
	}
	if err := tx.Create(&student1).Error; err != nil {
		t.Fatalf("failed to create student1: %v", err)
	}

	student2 := models.Student{
		StudentCode: "660510002",
		FullName:    "ชนิกานต์ รักเรียน",
	}
	if err := tx.Create(&student2).Error; err != nil {
		t.Fatalf("failed to create student2: %v", err)
	}

	// Duplicate student_code -> must fail uq_students_student_code
	assertExecFails(t, tx, "sp_dup_student", func(tx *gorm.DB) error {
		dup := models.Student{
			StudentCode: "660510001",
			FullName:    "ชื่อซ้ำ รหัสซ้ำ",
		}
		return tx.Create(&dup).Error
	}, "duplicate student_code")

	// Enroll student1 in courseA
	enrollment1 := models.Enrollment{
		CourseID:  courseA.ID,
		StudentID: student1.ID,
		Status:    models.EnrollmentStatusActive,
	}
	if err := tx.Create(&enrollment1).Error; err != nil {
		t.Fatalf("failed to enroll student1 in courseA: %v", err)
	}

	// Enroll student1 in courseA again -> must fail uq_enrollments_course_student
	assertExecFails(t, tx, "sp_dup_enrollment", func(tx *gorm.DB) error {
		dupEnroll := models.Enrollment{
			CourseID:  courseA.ID,
			StudentID: student1.ID,
			Status:    models.EnrollmentStatusActive,
		}
		return tx.Create(&dupEnroll).Error
	}, "duplicate enrollment in same course")

	// Enroll student2 in courseA2 (so student2 is NOT enrolled in courseA)
	enrollment2InA2 := models.Enrollment{
		CourseID:  courseA2.ID,
		StudentID: student2.ID,
		Status:    models.EnrollmentStatusActive,
	}
	if err := tx.Create(&enrollment2InA2).Error; err != nil {
		t.Fatalf("failed to enroll student2 in courseA2: %v", err)
	}

	// -------------------------------------------------------------------------
	// 3. Test ClassSession Constraints
	// -------------------------------------------------------------------------
	session1 := models.ClassSession{
		CourseID:      courseA.ID,
		SessionNumber: 1,
		Topic:         strPtr("Week 1: Introduction"),
		CurrentMode:   models.PickModeNoRepeat,
		CurrentRound:  1,
		Status:        models.SessionStatusActive,
	}
	if err := tx.Create(&session1).Error; err != nil {
		t.Fatalf("failed to create session1: %v", err)
	}

	// Second active session in the same course -> must fail uq_class_sessions_one_active_per_course
	assertExecFails(t, tx, "sp_second_active_session", func(tx *gorm.DB) error {
		session2 := models.ClassSession{
			CourseID:      courseA.ID,
			SessionNumber: 2,
			CurrentMode:   models.PickModeWeighted,
			CurrentRound:  1,
			Status:        models.SessionStatusActive,
		}
		return tx.Create(&session2).Error
	}, "second active session in the same course")

	// -------------------------------------------------------------------------
	// 4. Test Attendance & PickHistory Enrollment Integrity (Composite FKs)
	// -------------------------------------------------------------------------
	// Valid attendance for enrolled student1 in courseA
	att1 := models.Attendance{
		SessionID: session1.ID,
		CourseID:  courseA.ID,
		StudentID: student1.ID,
		Status:    models.AttendanceStatusPresent,
	}
	if err := tx.Create(&att1).Error; err != nil {
		t.Fatalf("failed to create valid attendance for student1: %v", err)
	}

	// Duplicate attendance in same session -> must fail uq_attendance_session_student
	assertExecFails(t, tx, "sp_dup_attendance", func(tx *gorm.DB) error {
		dupAtt := models.Attendance{
			SessionID: session1.ID,
			CourseID:  courseA.ID,
			StudentID: student1.ID,
			Status:    models.AttendanceStatusLate,
		}
		return tx.Create(&dupAtt).Error
	}, "duplicate attendance for same student in same session")

	// Attendance for student2 (who is NOT enrolled in courseA) -> must fail fk_attendance_enrollment
	assertExecFails(t, tx, "sp_unenrolled_attendance", func(tx *gorm.DB) error {
		unenrolledAtt := models.Attendance{
			SessionID: session1.ID,
			CourseID:  courseA.ID,
			StudentID: student2.ID,
			Status:    models.AttendanceStatusPresent,
		}
		return tx.Create(&unenrolledAtt).Error
	}, "attendance for student not enrolled in courseA")

	// Attendance with mismatched (session_id, course_id) -> must fail fk_attendance_session_course
	assertExecFails(t, tx, "sp_mismatched_session_course_att", func(tx *gorm.DB) error {
		badSessionCourse := models.Attendance{
			SessionID: session1.ID,
			CourseID:  courseA2.ID, // session1 belongs to courseA, not courseA2!
			StudentID: student2.ID,
			Status:    models.AttendanceStatusPresent,
		}
		return tx.Create(&badSessionCourse).Error
	}, "attendance with mismatched session_id and course_id")

	// PickHistory for student2 (NOT enrolled in courseA) -> must fail fk_pick_history_enrollment
	assertExecFails(t, tx, "sp_unenrolled_pick", func(tx *gorm.DB) error {
		unenrolledPick := models.PickHistory{
			CourseID:    courseA.ID,
			SessionID:   session1.ID,
			StudentID:   student2.ID,
			Mode:        models.PickModeWeighted,
			RoundNumber: 1,
			Status:      models.PickStatusValid,
			WeightScore: floatPtr(1.0),
		}
		return tx.Create(&unenrolledPick).Error
	}, "pick_history for student not enrolled in courseA")

	// Valid PickHistory in no_repeat mode (round 1) for student1
	pick1 := models.PickHistory{
		CourseID:    courseA.ID,
		SessionID:   session1.ID,
		StudentID:   student1.ID,
		Mode:        models.PickModeNoRepeat,
		RoundNumber: 1,
		Status:      models.PickStatusValid,
	}
	if err := tx.Create(&pick1).Error; err != nil {
		t.Fatalf("failed to create valid pick1: %v", err)
	}

	// Duplicate valid pick in same no_repeat round -> must fail uq_pick_history_no_repeat_valid
	assertExecFails(t, tx, "sp_dup_no_repeat_pick", func(tx *gorm.DB) error {
		dupPick := models.PickHistory{
			CourseID:    courseA.ID,
			SessionID:   session1.ID,
			StudentID:   student1.ID,
			Mode:        models.PickModeNoRepeat,
			RoundNumber: 1,
			Status:      models.PickStatusValid,
		}
		return tx.Create(&dupPick).Error
	}, "duplicate valid pick in same no_repeat round")

	// Cancel pick1 -> then picking student1 again in round 1 must succeed!
	if err := tx.Model(&pick1).Update("status", models.PickStatusCancelled).Error; err != nil {
		t.Fatalf("failed to cancel pick1: %v", err)
	}
	pick1Retry := models.PickHistory{
		CourseID:    courseA.ID,
		SessionID:   session1.ID,
		StudentID:   student1.ID,
		Mode:        models.PickModeNoRepeat,
		RoundNumber: 1,
		Status:      models.PickStatusValid,
	}
	if err := tx.Create(&pick1Retry).Error; err != nil {
		t.Fatalf("expected re-picking student1 in round 1 after cancellation to succeed, got: %v", err)
	}

	// -------------------------------------------------------------------------
	// 5. Test Deletion Protection (Referential Integrity)
	// -------------------------------------------------------------------------
	// 5.1 Course WITH session history (courseA) CANNOT be deleted
	assertExecFails(t, tx, "sp_delete_course_with_history", func(tx *gorm.DB) error {
		return tx.Delete(&models.Course{}, courseA.ID).Error
	}, "delete course that already has class_sessions history")

	// 5.2 Enrollment WITH attendance/pick_history CANNOT be hard-deleted
	assertExecFails(t, tx, "sp_delete_enrollment_with_history", func(tx *gorm.DB) error {
		return tx.Delete(&models.Enrollment{}, enrollment1.ID).Error
	}, "hard-delete enrollment that has attendance/pick_history")

	// 5.3 Student WITH enrollments CANNOT be hard-deleted
	assertExecFails(t, tx, "sp_delete_student_with_enrollment", func(tx *gorm.DB) error {
		return tx.Delete(&models.Student{}, student1.ID).Error
	}, "hard-delete student that has enrollments")

	// 5.4 Course WITHOUT session history (courseA2) CAN be deleted cleanly (cascading its empty enrollment)
	if err := tx.Delete(&models.Course{}, courseA2.ID).Error; err != nil {
		t.Fatalf("expected deleting courseA2 (which has no class_sessions) to succeed, got: %v", err)
	}
	var remainingEnrollmentsInA2 int64
	if err := tx.Model(&models.Enrollment{}).Where("course_id = ?", courseA2.ID).Count(&remainingEnrollmentsInA2).Error; err != nil {
		t.Fatalf("failed counting enrollments for deleted courseA2: %v", err)
	}
	if remainingEnrollmentsInA2 != 0 {
		t.Fatalf("expected enrollments in courseA2 to be cascade-deleted, found %d", remainingEnrollmentsInA2)
	}
	t.Logf("PASS: Course without session history deleted cleanly (enrollments cascaded: %s)", fmt.Sprintf("remaining=%d", remainingEnrollmentsInA2))
}

