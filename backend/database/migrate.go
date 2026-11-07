package database

import (
	"fmt"

	"github.com/Tewan0/FairPick/models"
	"gorm.io/gorm"
)

// Migrate runs GORM AutoMigrate for all 6 FairPick models followed by idempotent
// PostgreSQL 18 SQL statements for Partial Unique Indexes, CHECK constraints,
// and Single/Composite Foreign Keys.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}

	// Acquire PostgreSQL session-level advisory lock so concurrent Migrate() calls serialize safely
	_ = db.Exec("SELECT pg_advisory_lock(88442211)")
	defer db.Exec("SELECT pg_advisory_unlock(88442211)")

	if err := autoMigrateModels(db); err != nil {
		return fmt.Errorf("auto migrate models failed: %w", err)
	}

	if err := applyPostgreSQLConstraints(db); err != nil {
		return fmt.Errorf("apply postgresql constraints failed: %w", err)
	}

	return nil
}

func autoMigrateModels(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Course{},
		&models.Student{},
		&models.Enrollment{},
		&models.ClassSession{},
		&models.Attendance{},
		&models.PickHistory{},
	)
}

func applyPostgreSQLConstraints(db *gorm.DB) error {
	statements := []string{
		// 1. Partial Unique Indexes on courses (PostgreSQL 15+ / 18 NULLS NOT DISTINCT)
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_courses_code_section_semester
		 ON courses (course_code, section, semester)
		 NULLS NOT DISTINCT
		 WHERE course_code IS NOT NULL;`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uq_courses_name_section_semester_no_code
		 ON courses (LOWER(BTRIM(course_name)), section, semester)
		 NULLS NOT DISTINCT
		 WHERE course_code IS NULL;`,

		// 2. Partial Unique Index limiting at most 1 active session per course
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_class_sessions_one_active_per_course
		 ON class_sessions (course_id)
		 WHERE status = 'active';`,

		// 3. Partial Unique Index preventing duplicate student pick within the same no_repeat round (when valid)
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_pick_history_no_repeat_valid
		 ON pick_history (session_id, round_number, student_id)
		 WHERE mode = 'no_repeat' AND status = 'valid';`,

		// 4. Partial Index for fast lookup of valid picks in recent 4 sessions & current session
		`CREATE INDEX IF NOT EXISTS idx_pick_history_course_session_valid
		 ON pick_history (course_id, session_id, student_id)
		 WHERE status = 'valid';`,

		// 5. Idempotent CHECK constraints and Foreign Keys (Single & Composite)
		`DO $$
BEGIN
    -- courses CHECK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_courses_no_blank_strings') THEN
        ALTER TABLE courses ADD CONSTRAINT chk_courses_no_blank_strings CHECK (
            (course_code IS NULL OR BTRIM(course_code) <> '') AND
            (course_name IS NULL OR BTRIM(course_name) <> '') AND
            (section IS NULL OR BTRIM(section) <> '') AND
            (semester IS NULL OR BTRIM(semester) <> '')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_courses_code_or_name') THEN
        ALTER TABLE courses ADD CONSTRAINT chk_courses_code_or_name CHECK (
            course_code IS NOT NULL OR course_name IS NOT NULL
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_courses_code_format') THEN
        ALTER TABLE courses ADD CONSTRAINT chk_courses_code_format CHECK (
            course_code IS NULL OR course_code ~ '^[0-9]{6}$'
        );
    END IF;

    -- students CHECK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_students_not_empty') THEN
        ALTER TABLE students ADD CONSTRAINT chk_students_not_empty CHECK (
            BTRIM(student_code) <> '' AND BTRIM(full_name) <> ''
        );
    END IF;

    -- enrollments CHECK & FK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_enrollments_status') THEN
        ALTER TABLE enrollments ADD CONSTRAINT chk_enrollments_status CHECK (
            status IN ('active', 'withdrawn')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_enrollments_course') THEN
        ALTER TABLE enrollments ADD CONSTRAINT fk_enrollments_course
            FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_enrollments_student') THEN
        ALTER TABLE enrollments ADD CONSTRAINT fk_enrollments_student
            FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE RESTRICT;
    END IF;

    -- class_sessions CHECK & FK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_class_sessions_status') THEN
        ALTER TABLE class_sessions ADD CONSTRAINT chk_class_sessions_status CHECK (
            status IN ('active', 'ended')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_class_sessions_mode') THEN
        ALTER TABLE class_sessions ADD CONSTRAINT chk_class_sessions_mode CHECK (
            current_mode IN ('weighted', 'no_repeat')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_class_sessions_round') THEN
        ALTER TABLE class_sessions ADD CONSTRAINT chk_class_sessions_round CHECK (
            current_round >= 1
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_class_sessions_topic_not_blank') THEN
        ALTER TABLE class_sessions ADD CONSTRAINT chk_class_sessions_topic_not_blank CHECK (
            topic IS NULL OR BTRIM(topic) <> ''
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_class_sessions_course') THEN
        ALTER TABLE class_sessions ADD CONSTRAINT fk_class_sessions_course
            FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE RESTRICT;
    END IF;

    -- attendance CHECK & Composite FK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_attendance_status') THEN
        ALTER TABLE attendance ADD CONSTRAINT chk_attendance_status CHECK (
            status IN ('present', 'absent', 'late', 'unconfirmed')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_attendance_note_not_blank') THEN
        ALTER TABLE attendance ADD CONSTRAINT chk_attendance_note_not_blank CHECK (
            note IS NULL OR BTRIM(note) <> ''
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_attendance_session_course') THEN
        ALTER TABLE attendance ADD CONSTRAINT fk_attendance_session_course
            FOREIGN KEY (session_id, course_id) REFERENCES class_sessions(id, course_id) ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_attendance_enrollment') THEN
        ALTER TABLE attendance ADD CONSTRAINT fk_attendance_enrollment
            FOREIGN KEY (course_id, student_id) REFERENCES enrollments(course_id, student_id) ON DELETE RESTRICT;
    END IF;

    -- pick_history CHECK & Composite FK constraints
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pick_history_mode') THEN
        ALTER TABLE pick_history ADD CONSTRAINT chk_pick_history_mode CHECK (
            mode IN ('weighted', 'no_repeat')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pick_history_status') THEN
        ALTER TABLE pick_history ADD CONSTRAINT chk_pick_history_status CHECK (
            status IN ('valid', 'cancelled')
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pick_history_round') THEN
        ALTER TABLE pick_history ADD CONSTRAINT chk_pick_history_round CHECK (
            round_number >= 1
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pick_history_reason_not_blank') THEN
        ALTER TABLE pick_history ADD CONSTRAINT chk_pick_history_reason_not_blank CHECK (
            cancel_reason IS NULL OR BTRIM(cancel_reason) <> ''
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_pick_history_session_course') THEN
        ALTER TABLE pick_history ADD CONSTRAINT fk_pick_history_session_course
            FOREIGN KEY (session_id, course_id) REFERENCES class_sessions(id, course_id) ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_pick_history_enrollment') THEN
        ALTER TABLE pick_history ADD CONSTRAINT fk_pick_history_enrollment
            FOREIGN KEY (course_id, student_id) REFERENCES enrollments(course_id, student_id) ON DELETE RESTRICT;
    END IF;
END $$;`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

