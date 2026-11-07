package models

import "time"

const (
	AttendanceStatusPresent     = "present"
	AttendanceStatusAbsent      = "absent"
	AttendanceStatusLate        = "late"
	AttendanceStatusUnconfirmed = "unconfirmed"
)

type Attendance struct {
	ID        uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionID uint64     `gorm:"column:session_id;not null;uniqueIndex:uq_attendance_session_student,priority:1;index:idx_attendance_session_eligible,priority:1" json:"sessionId"`
	CourseID  uint64     `gorm:"column:course_id;not null;index:idx_attendance_course_student,priority:1" json:"courseId"`
	StudentID uint64     `gorm:"column:student_id;not null;uniqueIndex:uq_attendance_session_student,priority:2;index:idx_attendance_course_student,priority:2" json:"studentId"`
	Status    string     `gorm:"column:status;type:varchar(20);not null;default:'unconfirmed';index:idx_attendance_session_eligible,priority:2" json:"status"`
	CheckedAt *time.Time `gorm:"column:checked_at;type:timestamptz" json:"checkedAt"`
	Note      *string    `gorm:"column:note;type:varchar(255)" json:"note"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updatedAt"`

	Session *ClassSession `gorm:"foreignKey:SessionID;references:ID" json:"session,omitempty"`
	Course  *Course       `gorm:"foreignKey:CourseID;references:ID" json:"course,omitempty"`
	Student *Student      `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
}

func (Attendance) TableName() string {
	return "attendance"
}

