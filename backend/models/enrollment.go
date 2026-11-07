package models

import "time"

const (
	EnrollmentStatusActive    = "active"
	EnrollmentStatusWithdrawn = "withdrawn"
)

type Enrollment struct {
	ID          uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CourseID    uint64     `gorm:"column:course_id;not null;uniqueIndex:uq_enrollments_course_student,priority:1;index:idx_enrollments_course_status,priority:1" json:"courseId"`
	StudentID   uint64     `gorm:"column:student_id;not null;uniqueIndex:uq_enrollments_course_student,priority:2;index:idx_enrollments_student_id" json:"studentId"`
	Status      string     `gorm:"column:status;type:varchar(20);not null;default:'active';index:idx_enrollments_course_status,priority:2" json:"status"`
	EnrolledAt  time.Time  `gorm:"column:enrolled_at;type:timestamptz;not null;default:now()" json:"enrolledAt"`
	WithdrawnAt *time.Time `gorm:"column:withdrawn_at;type:timestamptz" json:"withdrawnAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Course  *Course  `gorm:"foreignKey:CourseID;references:ID" json:"course,omitempty"`
	Student *Student `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

