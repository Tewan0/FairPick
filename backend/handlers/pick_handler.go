package handlers

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/response"
	"github.com/Tewan0/FairPick/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PickHandler struct {
	db *gorm.DB
}

func NewPickHandler(db *gorm.DB) *PickHandler {
	return &PickHandler{db: db}
}

const WeightFormulaString = "W = 1 / (1 + H + 2C)"

type PickPoolCandidateDTO struct {
	StudentID                    uint64  `json:"studentId"`
	StudentCode                  string  `json:"studentCode"`
	FullName                     string  `json:"fullName"`
	AttendanceStatus             string  `json:"attendanceStatus"`
	Recent4SessionsPickCount     int64   `json:"recent4SessionsPickCount"`
	CurrentSessionPickCount      int64   `json:"currentSessionPickCount"`
	WeightScore                  float64 `json:"weightScore"`
	ProbabilityPercent           float64 `json:"probabilityPercent"`
	PickedInCurrentNoRepeatRound bool    `json:"pickedInCurrentNoRepeatRound"`
	rawWeight                    float64
}

type PickPoolResponseDTO struct {
	SessionID                uint64                 `json:"sessionId"`
	CourseID                 uint64                 `json:"courseId"`
	SessionNumber            int                    `json:"sessionNumber"`
	CurrentMode              string                 `json:"currentMode"`
	CurrentRound             int                    `json:"currentRound"`
	WeightFormula            string                 `json:"weightFormula"`
	PreviousSessionsUsed     []uint64               `json:"previousSessionsUsed"`
	TotalEligible            int                    `json:"totalEligible"`
	RemainingInNoRepeatRound int                    `json:"remainingInNoRepeatRound"`
	IsNoRepeatRoundCompleted bool                   `json:"isNoRepeatRoundCompleted"`
	Pool                     []PickPoolCandidateDTO `json:"pool"`
}

type ExecutePickRequest struct {
	Mode             *string `json:"mode"`
	AutoAdvanceRound bool    `json:"autoAdvanceRound"`
}

type PickedStudentBriefDTO struct {
	ID               uint64 `json:"id"`
	StudentCode      string `json:"studentCode"`
	FullName         string `json:"fullName"`
	AttendanceStatus string `json:"attendanceStatus"`
}

type ExecutedPickDetailDTO struct {
	ID            uint64                `json:"id"`
	CourseID      uint64                `json:"courseId"`
	SessionID     uint64                `json:"sessionId"`
	SessionNumber int                   `json:"sessionNumber"`
	Mode          string                `json:"mode"`
	RoundNumber   int                   `json:"roundNumber"`
	Status        string                `json:"status"`
	WeightScore   *float64              `json:"weightScore"`
	PickedAt      time.Time             `json:"pickedAt"`
	Student       PickedStudentBriefDTO `json:"student"`
}

type PickPoolStateDTO struct {
	CurrentMode      string `json:"currentMode"`
	CurrentRound     int    `json:"currentRound"`
	RoundReset       bool   `json:"roundReset"`
	IsRoundCompleted bool   `json:"isRoundCompleted"`
	TotalEligible    int    `json:"totalEligible"`
	RemainingInRound int    `json:"remainingInRound"`
}

type ExecutePickResponseDTO struct {
	Pick      ExecutedPickDetailDTO `json:"pick"`
	PoolState PickPoolStateDTO      `json:"poolState"`
}

type StartNewRoundResponseDTO struct {
	SessionID        uint64 `json:"sessionId"`
	PreviousRound    int    `json:"previousRound"`
	CurrentRound     int    `json:"currentRound"`
	IsRoundCompleted bool   `json:"isRoundCompleted"`
	TotalEligible    int    `json:"totalEligible"`
	RemainingInRound int    `json:"remainingInRound"`
}

type CancelPickRequest struct {
	CancelReason      *string `json:"cancelReason"`
	MarkStudentAbsent bool    `json:"markStudentAbsent"`
	MarkAsAbsent      bool    `json:"markAsAbsent"`
}

type CancelPickResponseDTO struct {
	ID           uint64     `json:"id"`
	CourseID     uint64     `json:"courseId"`
	SessionID    uint64     `json:"sessionId"`
	StudentID    uint64     `json:"studentId"`
	StudentCode  string     `json:"studentCode"`
	FullName     string     `json:"fullName"`
	Mode         string     `json:"mode"`
	RoundNumber  int        `json:"roundNumber"`
	Status       string     `json:"status"`
	PickedAt     time.Time  `json:"pickedAt"`
	CancelledAt  *time.Time `json:"cancelledAt"`
	CancelReason *string    `json:"cancelReason"`
}

type SessionPickHistorySummaryDTO struct {
	TotalPicks     int64 `json:"totalPicks"`
	ValidPicks     int64 `json:"validPicks"`
	CancelledPicks int64 `json:"cancelledPicks"`
}

type PickHistoryItemDTO struct {
	ID            uint64     `json:"id"`
	CourseID      uint64     `json:"courseId"`
	SessionID     uint64     `json:"sessionId"`
	SessionNumber int        `json:"sessionNumber"`
	SessionTopic  *string    `json:"sessionTopic,omitempty"`
	StudentID     uint64     `json:"studentId"`
	StudentCode   string     `json:"studentCode"`
	FullName      string     `json:"fullName"`
	Mode          string     `json:"mode"`
	RoundNumber   int        `json:"roundNumber"`
	Status        string     `json:"status"`
	WeightScore   *float64   `json:"weightScore"`
	PickedAt      time.Time  `json:"pickedAt"`
	CancelledAt   *time.Time `json:"cancelledAt"`
	CancelReason  *string    `json:"cancelReason"`
}

type SessionPickHistoryResponseDTO struct {
	SessionID uint64                       `json:"sessionId"`
	Summary   SessionPickHistorySummaryDTO `json:"summary"`
	Items     []PickHistoryItemDTO         `json:"items"`
}

var (
	errNoEligibleStudents   = errors.New("no_eligible_students")
	errRoundCompleted       = errors.New("round_completed")
	errPickNotFound         = errors.New("pick_not_found")
	errPickAlreadyCancelled = errors.New("pick_already_cancelled")
)

func round4(val float64) float64 {
	return math.Round(val*10000) / 10000
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}

// cryptoFloat64 returns a cryptographically secure random float64 in [0.0, 1.0).
func cryptoFloat64() (float64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	// Use top 53 bits for IEEE-754 double precision mantissa
	v := binary.BigEndian.Uint64(b[:]) >> 11
	return float64(v) / float64(uint64(1)<<53), nil
}

// cryptoIntn returns a cryptographically secure uniform random integer in [0, n).
func cryptoIntn(n int) (int, error) {
	if n <= 1 {
		return 0, nil
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// getPrevious4SessionIDs returns up to 4 previous session IDs in the same course with session_number < currentSessionNumber.
func getPrevious4SessionIDs(db *gorm.DB, courseID uint64, currentSessionNumber int) ([]uint64, error) {
	var ids []uint64
	err := db.Model(&models.ClassSession{}).
		Where("course_id = ? AND session_number < ?", courseID, currentSessionNumber).
		Order("session_number DESC").
		Limit(4).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []uint64{}
	}
	return ids, nil
}

// buildSessionPickPool computes the eligible candidate pool, H, C, W = 1/(1+H+2C), probabilities, and no_repeat round state.
func buildSessionPickPool(db *gorm.DB, session models.ClassSession) (PickPoolResponseDTO, error) {
	prevSessionIDs, err := getPrevious4SessionIDs(db, session.CourseID, session.SessionNumber)
	if err != nil {
		return PickPoolResponseDTO{}, err
	}

	type eligibleRow struct {
		StudentID        uint64
		StudentCode      string
		FullName         string
		AttendanceStatus string
	}

	var eligibleRows []eligibleRow
	err = db.Table("attendance").
		Select("attendance.student_id, students.student_code, students.full_name, attendance.status AS attendance_status").
		Joins("JOIN students ON students.id = attendance.student_id").
		Joins("JOIN enrollments ON enrollments.course_id = attendance.course_id AND enrollments.student_id = attendance.student_id").
		Where("attendance.session_id = ? AND attendance.course_id = ? AND attendance.status IN ? AND enrollments.status = ?",
			session.ID, session.CourseID, []string{models.AttendanceStatusPresent, models.AttendanceStatusLate}, models.EnrollmentStatusActive).
		Order("students.student_code ASC").
		Scan(&eligibleRows).Error
	if err != nil {
		return PickPoolResponseDTO{}, err
	}

	type studentCountRow struct {
		StudentID uint64
		Count     int64
	}

	// Count H: valid picks in the 4 previous sessions of the same course (both modes)
	recentMap := make(map[uint64]int64, len(eligibleRows))
	if len(prevSessionIDs) > 0 && len(eligibleRows) > 0 {
		var hRows []studentCountRow
		if err := db.Model(&models.PickHistory{}).
			Select("student_id, COUNT(*) AS count").
			Where("course_id = ? AND session_id IN ? AND status = ?", session.CourseID, prevSessionIDs, models.PickStatusValid).
			Group("student_id").
			Scan(&hRows).Error; err != nil {
			return PickPoolResponseDTO{}, err
		}
		for _, r := range hRows {
			recentMap[r.StudentID] = r.Count
		}
	}

	// Count C: valid picks in the current session (both modes)
	currentMap := make(map[uint64]int64, len(eligibleRows))
	if len(eligibleRows) > 0 {
		var cRows []studentCountRow
		if err := db.Model(&models.PickHistory{}).
			Select("student_id, COUNT(*) AS count").
			Where("session_id = ? AND status = ?", session.ID, models.PickStatusValid).
			Group("student_id").
			Scan(&cRows).Error; err != nil {
			return PickPoolResponseDTO{}, err
		}
		for _, r := range cRows {
			currentMap[r.StudentID] = r.Count
		}
	}

	// Find students already picked in current no_repeat round
	pickedInRoundMap := make(map[uint64]bool, len(eligibleRows))
	if len(eligibleRows) > 0 {
		var pickedIDs []uint64
		if err := db.Model(&models.PickHistory{}).
			Where("session_id = ? AND mode = ? AND round_number = ? AND status = ?",
				session.ID, models.PickModeNoRepeat, session.CurrentRound, models.PickStatusValid).
			Pluck("student_id", &pickedIDs).Error; err != nil {
			return PickPoolResponseDTO{}, err
		}
		for _, id := range pickedIDs {
			pickedInRoundMap[id] = true
		}
	}

	pool := make([]PickPoolCandidateDTO, 0, len(eligibleRows))
	var sumRawWeights float64
	remainingInNoRepeat := 0

	for _, er := range eligibleRows {
		h := recentMap[er.StudentID]
		c := currentMap[er.StudentID]
		rawW := 1.0 / float64(1+h+2*c)
		sumRawWeights += rawW

		pickedInRound := pickedInRoundMap[er.StudentID]
		if !pickedInRound {
			remainingInNoRepeat++
		}

		pool = append(pool, PickPoolCandidateDTO{
			StudentID:                    er.StudentID,
			StudentCode:                  er.StudentCode,
			FullName:                     er.FullName,
			AttendanceStatus:             er.AttendanceStatus,
			Recent4SessionsPickCount:     h,
			CurrentSessionPickCount:      c,
			WeightScore:                  round4(rawW),
			PickedInCurrentNoRepeatRound: pickedInRound,
			rawWeight:                    rawW,
		})
	}

	if sumRawWeights > 0 {
		for i := range pool {
			pool[i].ProbabilityPercent = round2((pool[i].rawWeight / sumRawWeights) * 100.0)
		}
	}

	totalEligible := len(pool)
	isRoundCompleted := totalEligible > 0 && remainingInNoRepeat == 0

	return PickPoolResponseDTO{
		SessionID:                session.ID,
		CourseID:                 session.CourseID,
		SessionNumber:            session.SessionNumber,
		CurrentMode:              session.CurrentMode,
		CurrentRound:             session.CurrentRound,
		WeightFormula:            WeightFormulaString,
		PreviousSessionsUsed:     prevSessionIDs,
		TotalEligible:            totalEligible,
		RemainingInNoRepeatRound: remainingInNoRepeat,
		IsNoRepeatRoundCompleted: isRoundCompleted,
		Pool:                     pool,
	}, nil
}

// GetPickPool handles GET /api/sessions/:sessionId/picks/pool
func (h *PickHandler) GetPickPool(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var session models.ClassSession
	if err := h.db.First(&session, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลคาบเรียนได้")
	}

	poolData, err := buildSessionPickPool(h.db, session)
	if err != nil {
		return response.InternalError(c, "ไม่สามารถคำนวณข้อมูล Pool การสุ่มได้")
	}

	return response.OK(c, "ดึงข้อมูล Pool การสุ่มสำเร็จ", poolData)
}

// ExecutePick handles POST /api/sessions/:sessionId/picks
func (h *PickHandler) ExecutePick(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var req ExecutePickRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
		}
	}

	var requestedMode string
	if req.Mode != nil {
		requestedMode = strings.ToLower(validator.NormalizeRequiredString(*req.Mode))
		if requestedMode != "" && requestedMode != models.PickModeWeighted && requestedMode != models.PickModeNoRepeat {
			return response.ValidationError(c, "โหมดการสุ่มไม่ถูกต้อง", map[string]string{
				"mode": "อนุญาตเฉพาะ weighted หรือ no_repeat เท่านั้น",
			})
		}
	}

	var result ExecutePickResponseDTO
	var completedRoundNumber int

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		var session models.ClassSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSessionNotFound
			}
			return err
		}

		if session.Status == models.SessionStatusEnded {
			return errSessionAlreadyEnded
		}

		now := time.Now()
		activeMode := session.CurrentMode
		if requestedMode != "" {
			activeMode = requestedMode
			if activeMode != session.CurrentMode {
				if err := tx.Model(&session).Updates(map[string]interface{}{
					"current_mode": activeMode,
					"updated_at":   now,
				}).Error; err != nil {
					return err
				}
				session.CurrentMode = activeMode
			}
		}

		poolData, err := buildSessionPickPool(tx, session)
		if err != nil {
			return err
		}

		if poolData.TotalEligible == 0 {
			return errNoEligibleStudents
		}

		var chosen PickPoolCandidateDTO
		var pickRoundNumber int
		var pickWeightScore *float64
		roundReset := false
		remainingAfterPick := poolData.RemainingInNoRepeatRound

		if activeMode == models.PickModeWeighted {
			var sumW float64
			for _, cand := range poolData.Pool {
				sumW += cand.rawWeight
			}
			u, err := cryptoFloat64()
			if err != nil {
				return err
			}
			target := u * sumW
			var cumulative float64
			chosen = poolData.Pool[len(poolData.Pool)-1]
			for _, cand := range poolData.Pool {
				cumulative += cand.rawWeight
				if target < cumulative {
					chosen = cand
					break
				}
			}

			pickRoundNumber = 1
			ws := chosen.WeightScore
			pickWeightScore = &ws
		} else {
			// Mode: no_repeat
			unpicked := make([]PickPoolCandidateDTO, 0, poolData.RemainingInNoRepeatRound)
			for _, cand := range poolData.Pool {
				if !cand.PickedInCurrentNoRepeatRound {
					unpicked = append(unpicked, cand)
				}
			}

			if len(unpicked) == 0 {
				if !req.AutoAdvanceRound {
					completedRoundNumber = session.CurrentRound
					return errRoundCompleted
				}
				// Auto-advance round when explicitly requested
				session.CurrentRound++
				if err := tx.Model(&session).Updates(map[string]interface{}{
					"current_round": session.CurrentRound,
					"updated_at":    now,
				}).Error; err != nil {
					return err
				}
				roundReset = true
				unpicked = poolData.Pool
			}

			idx, err := cryptoIntn(len(unpicked))
			if err != nil {
				return err
			}
			chosen = unpicked[idx]
			pickRoundNumber = session.CurrentRound
			pickWeightScore = nil
			remainingAfterPick = len(unpicked) - 1
		}

		newPick := models.PickHistory{
			CourseID:    session.CourseID,
			SessionID:   session.ID,
			StudentID:   chosen.StudentID,
			Mode:        activeMode,
			RoundNumber: pickRoundNumber,
			Status:      models.PickStatusValid,
			WeightScore: pickWeightScore,
			PickedAt:    now,
		}

		if err := tx.Create(&newPick).Error; err != nil {
			if validator.IsUniqueViolation(err) {
				completedRoundNumber = session.CurrentRound
				return errRoundCompleted
			}
			return err
		}

		isRoundCompleted := false
		if activeMode == models.PickModeNoRepeat {
			isRoundCompleted = poolData.TotalEligible > 0 && remainingAfterPick == 0
		} else {
			isRoundCompleted = poolData.TotalEligible > 0 && remainingAfterPick == 0
		}

		result = ExecutePickResponseDTO{
			Pick: ExecutedPickDetailDTO{
				ID:            newPick.ID,
				CourseID:      session.CourseID,
				SessionID:     session.ID,
				SessionNumber: session.SessionNumber,
				Mode:          newPick.Mode,
				RoundNumber:   newPick.RoundNumber,
				Status:        newPick.Status,
				WeightScore:   newPick.WeightScore,
				PickedAt:      newPick.PickedAt,
				Student: PickedStudentBriefDTO{
					ID:               chosen.StudentID,
					StudentCode:      chosen.StudentCode,
					FullName:         chosen.FullName,
					AttendanceStatus: chosen.AttendanceStatus,
				},
			},
			PoolState: PickPoolStateDTO{
				CurrentMode:      session.CurrentMode,
				CurrentRound:     session.CurrentRound,
				RoundReset:       roundReset,
				IsRoundCompleted: isRoundCompleted,
				TotalEligible:    poolData.TotalEligible,
				RemainingInRound: remainingAfterPick,
			},
		}

		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถสุ่มรายชื่อในคาบเรียนที่จบไปแล้วได้")
		case errors.Is(txErr, errNoEligibleStudents):
			return response.Unprocessable(c, response.CodeNoEligibleStudents, "ไม่สามารถสุ่มได้ เนื่องจากยังไม่มีนักศึกษาที่มีสถานะมาเรียน (present) หรือมาสาย (late) ในคาบนี้")
		case errors.Is(txErr, errRoundCompleted):
			return response.Unprocessable(c, response.CodeRoundCompleted,
				fmt.Sprintf("สุ่มครบทุกคนที่มีสิทธิ์ในรอบที่ %d แล้ว กรุณากดปุ่ม 'เริ่มรอบใหม่' เพื่อสุ่มรอบถัดไป", completedRoundNumber))
		default:
			return response.InternalError(c, "ไม่สามารถดำเนินการสุ่มรายชื่อนักศึกษาได้")
		}
	}

	return response.Created(c, "สุ่มรายชื่อนักศึกษาสำเร็จ", result)
}

// StartNewRound handles POST /api/sessions/:sessionId/picks/new-round
func (h *PickHandler) StartNewRound(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	var out StartNewRoundResponseDTO

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		var session models.ClassSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSessionNotFound
			}
			return err
		}

		if session.Status == models.SessionStatusEnded {
			return errSessionAlreadyEnded
		}

		attSummary, err := ComputeAttendanceSummary(tx, session.ID, session.CourseID)
		if err != nil {
			return err
		}

		if attSummary.EligibleForPick == 0 {
			return errNoEligibleStudents
		}

		prevRound := session.CurrentRound
		newRound := prevRound + 1
		now := time.Now()

		if err := tx.Model(&session).Updates(map[string]interface{}{
			"current_round": newRound,
			"updated_at":    now,
		}).Error; err != nil {
			return err
		}

		out = StartNewRoundResponseDTO{
			SessionID:        session.ID,
			PreviousRound:    prevRound,
			CurrentRound:     newRound,
			IsRoundCompleted: false,
			TotalEligible:    int(attSummary.EligibleForPick),
			RemainingInRound: int(attSummary.EligibleForPick),
		}
		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถเริ่มรอบการสุ่มใหม่ในคาบเรียนที่จบไปแล้วได้")
		case errors.Is(txErr, errNoEligibleStudents):
			return response.Unprocessable(c, response.CodeNoEligibleStudents, "ไม่สามารถเริ่มรอบใหม่ได้ เนื่องจากยังไม่มีนักศึกษาที่มีสถานะมาเรียน (present) หรือมาสาย (late) ในคาบนี้")
		default:
			return response.InternalError(c, "ไม่สามารถเริ่มรอบการสุ่มใหม่ได้")
		}
	}

	return response.OK(c, fmt.Sprintf("เริ่มรอบการสุ่มใหม่ (รอบที่ %d) สำเร็จ", out.CurrentRound), out)
}

// CancelPick handles PATCH /api/picks/:pickId/cancel
func (h *PickHandler) CancelPick(c *fiber.Ctx) error {
	pickID, err := validator.ParseUintParam(c, "pickId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสประวัติการสุ่ม (pickId) ไม่ถูกต้อง")
	}

	var req CancelPickRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.InvalidRequest(c, "รูปแบบข้อมูล JSON ไม่ถูกต้อง")
		}
	}

	cancelReason := validator.NormalizeOptionalString(req.CancelReason)
	if cancelReason != nil && len([]rune(*cancelReason)) > 255 {
		return response.ValidationError(c, "ข้อมูลการยกเลิกผลการสุ่มไม่ถูกต้อง", map[string]string{
			"cancelReason": "เหตุผลที่ยกเลิกต้องมีความยาวไม่เกิน 255 ตัวอักษร",
		})
	}

	markAbsent := req.MarkStudentAbsent || req.MarkAsAbsent

	var out CancelPickResponseDTO

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		var pick models.PickHistory
		if err := tx.First(&pick, pickID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errPickNotFound
			}
			return err
		}

		// Lock class_session first (consistent lock ordering: class_sessions -> pick_history -> attendance)
		var session models.ClassSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, pick.SessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSessionNotFound
			}
			return err
		}

		// Re-read pick with FOR UPDATE under the session lock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pick, pickID).Error; err != nil {
			return err
		}

		if pick.Status == models.PickStatusCancelled {
			return errPickAlreadyCancelled
		}

		if session.Status == models.SessionStatusEnded {
			return errSessionAlreadyEnded
		}

		now := time.Now()
		if err := tx.Model(&pick).Updates(map[string]interface{}{
			"status":        models.PickStatusCancelled,
			"cancelled_at":  &now,
			"cancel_reason": cancelReason,
			"updated_at":    now,
		}).Error; err != nil {
			return err
		}

		if markAbsent {
			if err := tx.Model(&models.Attendance{}).
				Where("session_id = ? AND student_id = ?", pick.SessionID, pick.StudentID).
				Updates(map[string]interface{}{
					"status":     models.AttendanceStatusAbsent,
					"checked_at": &now,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}

		var student models.Student
		if err := tx.First(&student, pick.StudentID).Error; err != nil {
			return err
		}

		out = CancelPickResponseDTO{
			ID:           pick.ID,
			CourseID:     pick.CourseID,
			SessionID:    pick.SessionID,
			StudentID:    pick.StudentID,
			StudentCode:  student.StudentCode,
			FullName:     student.FullName,
			Mode:         pick.Mode,
			RoundNumber:  pick.RoundNumber,
			Status:       models.PickStatusCancelled,
			PickedAt:     pick.PickedAt,
			CancelledAt:  &now,
			CancelReason: cancelReason,
		}
		return nil
	})

	if txErr != nil {
		switch {
		case errors.Is(txErr, errPickNotFound), errors.Is(txErr, errSessionNotFound):
			return response.NotFound(c, "ไม่พบรายการประวัติการสุ่มที่ระบุ")
		case errors.Is(txErr, errPickAlreadyCancelled):
			return response.Conflict(c, response.CodePickAlreadyCancel, "รายการสุ่มนี้ถูกยกเลิกไปแล้ว ไม่สามารถยกเลิกซ้ำได้")
		case errors.Is(txErr, errSessionAlreadyEnded):
			return response.Conflict(c, response.CodeSessionAlreadyEnded, "ไม่สามารถยกเลิกผลการสุ่มในคาบเรียนที่จบไปแล้วได้")
		default:
			return response.InternalError(c, "ไม่สามารถยกเลิกผลการสุ่มได้")
		}
	}

	return response.OK(c, "ยกเลิกผลการสุ่มสำเร็จ (ไม่นับเป็นจำนวนครั้งที่ถูกสุ่มจริง)", out)
}

// ListSessionPicks handles GET /api/sessions/:sessionId/picks
func (h *PickHandler) ListSessionPicks(c *fiber.Ctx) error {
	sessionID, err := validator.ParseUintParam(c, "sessionId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสคาบเรียน (sessionId) ไม่ถูกต้อง")
	}

	statusFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("status")))
	if statusFilter != "" && statusFilter != models.PickStatusValid && statusFilter != models.PickStatusCancelled {
		return response.ValidationError(c, "ตัวกรองสถานะการสุ่มไม่ถูกต้อง", map[string]string{
			"status": "อนุญาตเฉพาะ valid หรือ cancelled เท่านั้น",
		})
	}

	modeFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("mode")))
	if modeFilter != "" && modeFilter != models.PickModeWeighted && modeFilter != models.PickModeNoRepeat {
		return response.ValidationError(c, "ตัวกรองโหมดการสุ่มไม่ถูกต้อง", map[string]string{
			"mode": "อนุญาตเฉพาะ weighted หรือ no_repeat เท่านั้น",
		})
	}

	roundStr := validator.NormalizeRequiredString(c.Query("roundNumber"))
	var roundFilter int
	if roundStr != "" {
		r, err := strconv.Atoi(roundStr)
		if err != nil || r < 1 {
			return response.ValidationError(c, "ตัวกรองรอบการสุ่มไม่ถูกต้อง", map[string]string{
				"roundNumber": "roundNumber ต้องเป็นตัวเลขจำนวนเต็มบวก",
			})
		}
		roundFilter = r
	}

	var session models.ClassSession
	if err := h.db.First(&session, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบข้อมูลคาบเรียนที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถดึงข้อมูลคาบเรียนได้")
	}

	var validPicks, cancelledPicks int64
	if err := h.db.Model(&models.PickHistory{}).
		Where("session_id = ? AND status = ?", session.ID, models.PickStatusValid).
		Count(&validPicks).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลประวัติการสุ่มได้")
	}
	if err := h.db.Model(&models.PickHistory{}).
		Where("session_id = ? AND status = ?", session.ID, models.PickStatusCancelled).
		Count(&cancelledPicks).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถสรุปข้อมูลประวัติการสุ่มได้")
	}

	query := h.db.Table("pick_history").
		Select(`pick_history.id,
			pick_history.course_id,
			pick_history.session_id,
			class_sessions.session_number,
			pick_history.student_id,
			students.student_code,
			students.full_name,
			pick_history.mode,
			pick_history.round_number,
			pick_history.status,
			pick_history.weight_score,
			pick_history.picked_at,
			pick_history.cancelled_at,
			pick_history.cancel_reason`).
		Joins("JOIN students ON students.id = pick_history.student_id").
		Joins("JOIN class_sessions ON class_sessions.id = pick_history.session_id").
		Where("pick_history.session_id = ?", session.ID)

	if statusFilter != "" {
		query = query.Where("pick_history.status = ?", statusFilter)
	}
	if modeFilter != "" {
		query = query.Where("pick_history.mode = ?", modeFilter)
	}
	if roundFilter > 0 {
		query = query.Where("pick_history.round_number = ?", roundFilter)
	}

	var items []PickHistoryItemDTO
	if err := query.Order("pick_history.picked_at DESC, pick_history.id DESC").Scan(&items).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถดึงรายการประวัติการสุ่มได้")
	}
	if items == nil {
		items = []PickHistoryItemDTO{}
	}

	return response.OK(c, "ดึงประวัติการสุ่มประจำคาบสำเร็จ", SessionPickHistoryResponseDTO{
		SessionID: session.ID,
		Summary: SessionPickHistorySummaryDTO{
			TotalPicks:     validPicks + cancelledPicks,
			ValidPicks:     validPicks,
			CancelledPicks: cancelledPicks,
		},
		Items: items,
	})
}

// ListCoursePicks handles GET /api/courses/:courseId/picks
func (h *PickHandler) ListCoursePicks(c *fiber.Ctx) error {
	courseID, err := validator.ParseUintParam(c, "courseId")
	if err != nil {
		return response.InvalidRequest(c, "รหัสรายวิชา (courseId) ไม่ถูกต้อง")
	}

	var course models.Course
	if err := h.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "ไม่พบรายวิชาที่ระบุ")
		}
		return response.InternalError(c, "ไม่สามารถตรวจสอบข้อมูลรายวิชาได้")
	}

	statusFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("status")))
	if statusFilter != "" && statusFilter != models.PickStatusValid && statusFilter != models.PickStatusCancelled {
		return response.ValidationError(c, "ตัวกรองสถานะการสุ่มไม่ถูกต้อง", map[string]string{
			"status": "อนุญาตเฉพาะ valid หรือ cancelled เท่านั้น",
		})
	}

	modeFilter := strings.ToLower(validator.NormalizeRequiredString(c.Query("mode")))
	if modeFilter != "" && modeFilter != models.PickModeWeighted && modeFilter != models.PickModeNoRepeat {
		return response.ValidationError(c, "ตัวกรองโหมดการสุ่มไม่ถูกต้อง", map[string]string{
			"mode": "อนุญาตเฉพาะ weighted หรือ no_repeat เท่านั้น",
		})
	}

	var sessionFilter uint64
	if rawSessionID := validator.NormalizeRequiredString(c.Query("sessionId")); rawSessionID != "" {
		v, err := strconv.ParseUint(rawSessionID, 10, 64)
		if err != nil || v == 0 {
			return response.ValidationError(c, "ตัวกรองรหัสคาบเรียนไม่ถูกต้อง", map[string]string{
				"sessionId": "sessionId ต้องเป็นตัวเลขจำนวนเต็มบวก",
			})
		}
		sessionFilter = v
	}

	var studentFilter uint64
	if rawStudentID := validator.NormalizeRequiredString(c.Query("studentId")); rawStudentID != "" {
		v, err := strconv.ParseUint(rawStudentID, 10, 64)
		if err != nil || v == 0 {
			return response.ValidationError(c, "ตัวกรองรหัสนักศึกษาไม่ถูกต้อง", map[string]string{
				"studentId": "studentId ต้องเป็นตัวเลขจำนวนเต็มบวก",
			})
		}
		studentFilter = v
	}

	search := validator.NormalizeRequiredString(c.Query("search"))
	page, limit, offset := validator.ParsePagination(c, 20, 1000)

	baseQuery := h.db.Table("pick_history").
		Joins("JOIN students ON students.id = pick_history.student_id").
		Joins("JOIN class_sessions ON class_sessions.id = pick_history.session_id").
		Where("pick_history.course_id = ?", courseID)

	if sessionFilter > 0 {
		baseQuery = baseQuery.Where("pick_history.session_id = ?", sessionFilter)
	}
	if studentFilter > 0 {
		baseQuery = baseQuery.Where("pick_history.student_id = ?", studentFilter)
	}
	if modeFilter != "" {
		baseQuery = baseQuery.Where("pick_history.mode = ?", modeFilter)
	}
	if statusFilter != "" {
		baseQuery = baseQuery.Where("pick_history.status = ?", statusFilter)
	}
	if search != "" {
		like := "%" + search + "%"
		baseQuery = baseQuery.Where("(students.student_code ILIKE ? OR students.full_name ILIKE ?)", like, like)
	}

	var totalItems int64
	if err := baseQuery.Count(&totalItems).Error; err != nil {
		return response.InternalError(c, "ไม่สามารถนับจำนวนประวัติการสุ่มได้")
	}

	var items []PickHistoryItemDTO
	err = baseQuery.
		Select(`pick_history.id,
			pick_history.course_id,
			pick_history.session_id,
			class_sessions.session_number,
			class_sessions.topic AS session_topic,
			pick_history.student_id,
			students.student_code,
			students.full_name,
			pick_history.mode,
			pick_history.round_number,
			pick_history.status,
			pick_history.weight_score,
			pick_history.picked_at,
			pick_history.cancelled_at,
			pick_history.cancel_reason`).
		Order("pick_history.picked_at DESC, pick_history.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error
	if err != nil {
		return response.InternalError(c, "ไม่สามารถดึงประวัติการสุ่มของรายวิชาได้")
	}
	if items == nil {
		items = []PickHistoryItemDTO{}
	}

	return response.Paginated(c, "ดึงประวัติการสุ่มของรายวิชาสำเร็จ", items, page, limit, totalItems)
}

