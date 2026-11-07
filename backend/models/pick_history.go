package models

import "time"

const (
	PickStatusValid     = "valid"
	PickStatusCancelled = "cancelled"
)

type PickHistory struct {
	ID           uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CourseID     uint64     `gorm:"column:course_id;not null" json:"courseId"`
	SessionID    uint64     `gorm:"column:session_id;not null;index:idx_pick_history_session_picked_at,priority:1" json:"sessionId"`
	StudentID    uint64     `gorm:"column:student_id;not null" json:"studentId"`
	Mode         string     `gorm:"column:mode;type:varchar(20);not null" json:"mode"`
	RoundNumber  int        `gorm:"column:round_number;not null;default:1" json:"roundNumber"`
	Status       string     `gorm:"column:status;type:varchar(20);not null;default:'valid'" json:"status"`
	WeightScore  *float64   `gorm:"column:weight_score;type:numeric(10,4)" json:"weightScore"`
	PickedAt     time.Time  `gorm:"column:picked_at;type:timestamptz;not null;default:now();index:idx_pick_history_session_picked_at,priority:2,sort:desc" json:"pickedAt"`
	CancelledAt  *time.Time `gorm:"column:cancelled_at;type:timestamptz" json:"cancelledAt"`
	CancelReason *string    `gorm:"column:cancel_reason;type:varchar(255)" json:"cancelReason"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Course  *Course       `gorm:"foreignKey:CourseID;references:ID" json:"course,omitempty"`
	Session *ClassSession `gorm:"foreignKey:SessionID;references:ID" json:"session,omitempty"`
	Student *Student      `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
}

func (PickHistory) TableName() string {
	return "pick_history"
}

