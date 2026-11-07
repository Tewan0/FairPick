package models

import "time"

const (
	SessionStatusActive = "active"
	SessionStatusEnded  = "ended"

	PickModeWeighted = "weighted"
	PickModeNoRepeat = "no_repeat"
)

type ClassSession struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement;uniqueIndex:uq_class_sessions_id_course,priority:1" json:"id"`
	CourseID      uint64     `gorm:"column:course_id;not null;uniqueIndex:uq_class_sessions_id_course,priority:2;uniqueIndex:uq_class_sessions_course_number,priority:1;index:idx_class_sessions_course_number_desc,priority:1" json:"courseId"`
	SessionNumber int        `gorm:"column:session_number;not null;uniqueIndex:uq_class_sessions_course_number,priority:2;index:idx_class_sessions_course_number_desc,priority:2,sort:desc" json:"sessionNumber"`
	Topic         *string    `gorm:"column:topic;type:varchar(255)" json:"topic"`
	CurrentMode   string     `gorm:"column:current_mode;type:varchar(20);not null;default:'weighted'" json:"currentMode"`
	CurrentRound  int        `gorm:"column:current_round;not null;default:1" json:"currentRound"`
	Status        string     `gorm:"column:status;type:varchar(20);not null;default:'active'" json:"status"`
	StartedAt     time.Time  `gorm:"column:started_at;type:timestamptz;not null;default:now()" json:"startedAt"`
	EndedAt       *time.Time `gorm:"column:ended_at;type:timestamptz" json:"endedAt"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Course      *Course       `gorm:"foreignKey:CourseID;references:ID" json:"course,omitempty"`
	Attendances []Attendance  `gorm:"foreignKey:SessionID;references:ID" json:"attendances,omitempty"`
	PickHistory []PickHistory `gorm:"foreignKey:SessionID;references:ID" json:"pickHistory,omitempty"`
}

func (ClassSession) TableName() string {
	return "class_sessions"
}

