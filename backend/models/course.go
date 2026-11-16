package models

import "time"

type Course struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CourseCode *string   `gorm:"column:course_code;type:varchar(10)" json:"courseCode"`
	CourseName *string   `gorm:"column:course_name;type:varchar(255)" json:"courseName"`
	Section    *string   `gorm:"column:section;type:varchar(50)" json:"section"`
	Semester   *string   `gorm:"column:semester;type:varchar(50)" json:"semester"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Enrollments   []Enrollment   `gorm:"foreignKey:CourseID;references:ID" json:"enrollments,omitempty"`
	ClassSessions []ClassSession `gorm:"foreignKey:CourseID;references:ID" json:"classSessions,omitempty"`
}

func (Course) TableName() string {
	return "courses"
}

