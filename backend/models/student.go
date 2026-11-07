package models

import "time"

type Student struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StudentCode string    `gorm:"column:student_code;type:varchar(30);not null;uniqueIndex:uq_students_student_code" json:"studentCode"`
	FullName    string    `gorm:"column:full_name;type:varchar(255);not null;index:idx_students_full_name" json:"fullName"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Enrollments []Enrollment `gorm:"foreignKey:StudentID;references:ID" json:"enrollments,omitempty"`
}

func (Student) TableName() string {
	return "students"
}

