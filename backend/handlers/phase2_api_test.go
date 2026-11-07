package handlers_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/Tewan0/FairPick/models"
)

func TestPhase2SessionsAndAttendanceLifecycle(t *testing.T) {
	app, db := setupPhase1TestApp(t)

	// 1. Create a course
	status, res := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "001102",
		"courseName": "Fundamental English 2",
		"section":    "001",
		"semester":   "2/2569",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 creating course, got %d: %v", status, res)
	}
	courseID := uint64(res["data"].(map[string]interface{})["id"].(float64))

	// 2. Test starting and ending a session when course has 0 enrolled students
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Orientation (Empty Class)",
		"initialMode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 starting session with 0 students, got %d: %v", status, res)
	}
	session1Data := res["data"].(map[string]interface{})
	session1ID := uint64(session1Data["id"].(float64))
	if int(session1Data["sessionNumber"].(float64)) != 1 {
		t.Fatalf("expected sessionNumber=1, got %v", session1Data["sessionNumber"])
	}
	attSum1 := session1Data["attendanceSummary"].(map[string]interface{})
	if int(attSum1["totalEnrolled"].(float64)) != 0 {
		t.Fatalf("expected totalEnrolled=0 for empty course, got %v", attSum1["totalEnrolled"])
	}

	// End session 1
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", session1ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 ending session 1, got %d: %v", status, res)
	}

	// 3. Enroll 4 students, and withdraw student 4 BEFORE starting Session 2
	var studentIDs []uint64
	for i := 1; i <= 4; i++ {
		st, sRes := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
			"studentCode": fmt.Sprintf("66052000%d", i),
			"fullName":    fmt.Sprintf("นักศึกษา ทดสอบ %d", i),
			"courseId":    courseID,
		})
		if st != http.StatusCreated {
			t.Fatalf("expected 201 creating student %d, got %d: %v", i, st, sRes)
		}
		studentIDs = append(studentIDs, uint64(sRes["data"].(map[string]interface{})["id"].(float64)))
	}

	// Withdraw student 4 before starting Session 2
	status, res = doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d/enrollments/%d", courseID, studentIDs[3]), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 withdrawing student 4, got %d: %v", status, res)
	}

	// 4. Start Session 2: must get sessionNumber=2, create unconfirmed attendance ONLY for the 3 active students (excluding withdrawn student 4)
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Unit 2: Academic Reading",
		"initialMode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 starting session 2, got %d: %v", status, res)
	}
	session2Data := res["data"].(map[string]interface{})
	session2ID := uint64(session2Data["id"].(float64))
	if int(session2Data["sessionNumber"].(float64)) != 2 {
		t.Fatalf("expected sessionNumber=2, got %v", session2Data["sessionNumber"])
	}
	attSum2 := session2Data["attendanceSummary"].(map[string]interface{})
	if int(attSum2["totalEnrolled"].(float64)) != 3 || int(attSum2["unconfirmed"].(float64)) != 3 || int(attSum2["eligibleForPick"].(float64)) != 0 {
		t.Fatalf("expected 3 unconfirmed and 0 eligible for session 2, got %v", attSum2)
	}

	// Verify activeSession appears in GET /api/courses/:courseId
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d", courseID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting course details, got %d", status)
	}
	activeSessionObj, ok := res["data"].(map[string]interface{})["activeSession"].(map[string]interface{})
	if !ok || uint64(activeSessionObj["id"].(float64)) != session2ID {
		t.Fatalf("expected activeSession.id=%d on course detail, got %v", session2ID, res["data"].(map[string]interface{})["activeSession"])
	}

	// Verify deleting a course that has session history is rejected with 409 COURSE_HAS_HISTORY
	status, res = doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d", courseID), nil)
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "COURSE_HAS_HISTORY" {
		t.Fatalf("expected 409 COURSE_HAS_HISTORY when deleting course with sessions, got %d: %v", status, res)
	}

	// 5. Prevent starting another active session in the same course -> 409 ACTIVE_SESSION_EXISTS
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic": "Duplicate Active Session",
	})
	if status != http.StatusConflict {
		t.Fatalf("expected 409 ACTIVE_SESSION_EXISTS, got %d: %v", status, res)
	}
	errObj := res["error"].(map[string]interface{})
	if errObj["code"] != "ACTIVE_SESSION_EXISTS" {
		t.Fatalf("expected code ACTIVE_SESSION_EXISTS, got %v", errObj["code"])
	}

	// 6. Check attendance sheet for Session 2 (GET /api/sessions/:sessionId/attendance)
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/attendance", session2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting attendance sheet, got %d: %v", status, res)
	}
	attSheet := res["data"].(map[string]interface{})
	if attSheet["hasUnconfirmedWarning"] != true {
		t.Fatalf("expected hasUnconfirmedWarning=true when unconfirmed > 0")
	}
	records := attSheet["records"].([]interface{})
	if len(records) != 3 {
		t.Fatalf("expected 3 attendance records (withdrawn student 4 excluded), got %d", len(records))
	}

	// 7. Individual Attendance Check-in (PATCH /api/sessions/:sessionId/attendance/:studentId)
	// Student 1 -> present (eligible = true)
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", session2ID, studentIDs[0]), map[string]interface{}{
		"status": "present",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 updating student 1 attendance, got %d: %v", status, res)
	}
	upd1 := res["data"].(map[string]interface{})
	if upd1["status"] != "present" || upd1["isEligibleForPick"] != true || upd1["checkedAt"] == nil {
		t.Fatalf("expected present & isEligibleForPick=true & non-nil checkedAt, got %v", upd1)
	}

	// Student 2 -> late with note (eligible = true)
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", session2ID, studentIDs[1]), map[string]interface{}{
		"status": "late",
		"note":   "  มาสาย 10 นาที  ",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 updating student 2 attendance, got %d: %v", status, res)
	}
	upd2 := res["data"].(map[string]interface{})
	if upd2["status"] != "late" || upd2["isEligibleForPick"] != true || upd2["note"] != "มาสาย 10 นาที" {
		t.Fatalf("expected late & isEligibleForPick=true & normalized note, got %v", upd2)
	}

	// Verify filter by status=late and search on GET /api/sessions/:sessionId/attendance
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/attendance?status=late&search=660520002", session2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 filtering attendance sheet, got %d: %v", status, res)
	}
	filteredRecords := res["data"].(map[string]interface{})["records"].([]interface{})
	if len(filteredRecords) != 1 {
		t.Fatalf("expected 1 late record matching search, got %d", len(filteredRecords))
	}

	// 8. Forbid checking in a withdrawn or non-enrolled student -> 422 STUDENT_NOT_ENROLLED
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", session2ID, studentIDs[3]), map[string]interface{}{
		"status": "present",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 STUDENT_NOT_ENROLLED for withdrawn student, got %d: %v", status, res)
	}
	if res["error"].(map[string]interface{})["code"] != "STUDENT_NOT_ENROLLED" {
		t.Fatalf("expected code STUDENT_NOT_ENROLLED, got %v", res["error"])
	}

	// Create an unenrolled student 5
	status, s5Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660520099",
		"fullName":    "นักศึกษา นอกวิชา",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 creating student 5, got %d", status)
	}
	student5ID := uint64(s5Res["data"].(map[string]interface{})["id"].(float64))

	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", session2ID, student5ID), map[string]interface{}{
		"status": "present",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 STUDENT_NOT_ENROLLED for unenrolled student 5, got %d: %v", status, res)
	}

	// Also reject unenrolled student in Bulk items -> 422 STUDENT_NOT_ENROLLED
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", session2ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": student5ID, "status": "present"},
		},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 STUDENT_NOT_ENROLLED in bulk update, got %d: %v", status, res)
	}

	// 9. Test Mid-Session Enrollment: enrolling Student 5 while Session 2 is active automatically creates unconfirmed attendance
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseID), map[string]interface{}{
		"studentId": student5ID,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 enrolling student 5 mid-session, got %d: %v", status, res)
	}
	enrData := res["data"].(map[string]interface{})
	if uint64(enrData["addedToActiveSessionId"].(float64)) != session2ID {
		t.Fatalf("expected addedToActiveSessionId=%d, got %v", session2ID, enrData["addedToActiveSessionId"])
	}

	// Verify session 2 now has 4 attendance records (1 present, 1 late, 2 unconfirmed)
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d", session2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting session 2, got %d: %v", status, res)
	}
	sumAfterMidEnroll := res["data"].(map[string]interface{})["attendanceSummary"].(map[string]interface{})
	if int(sumAfterMidEnroll["totalEnrolled"].(float64)) != 4 || int(sumAfterMidEnroll["unconfirmed"].(float64)) != 2 {
		t.Fatalf("expected totalEnrolled=4 and unconfirmed=2 after mid-session enrollment, got %v", sumAfterMidEnroll)
	}

	// Withdraw student 5 mid-session while still unconfirmed -> unconfirmed row in active session is cleaned up
	status, res = doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d/enrollments/%d", courseID, student5ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 withdrawing student 5 mid-session, got %d: %v", status, res)
	}
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d", session2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting session 2, got %d", status)
	}
	sumAfterMidWithdraw := res["data"].(map[string]interface{})["attendanceSummary"].(map[string]interface{})
	if int(sumAfterMidWithdraw["totalEnrolled"].(float64)) != 3 || int(sumAfterMidWithdraw["unconfirmed"].(float64)) != 1 {
		t.Fatalf("expected totalEnrolled=3 and unconfirmed=1 after mid-session withdraw of unconfirmed student, got %v", sumAfterMidWithdraw)
	}

	// 10. Switch Mode between weighted and no_repeat without losing history (PATCH /api/sessions/:sessionId/mode)
	// Insert a valid pick record in session 2 to verify pick history and round are preserved across mode switches
	pick := models.PickHistory{
		SessionID:   session2ID,
		CourseID:    courseID,
		StudentID:   studentIDs[0],
		Mode:        models.PickModeNoRepeat,
		RoundNumber: 1,
		Status:      models.PickStatusValid,
	}
	if err := db.Create(&pick).Error; err != nil {
		t.Fatalf("failed inserting test pick history: %v", err)
	}

	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/mode", session2ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 switching mode to no_repeat, got %d: %v", status, res)
	}
	modeData := res["data"].(map[string]interface{})
	if modeData["currentMode"] != "no_repeat" || int(modeData["currentRound"].(float64)) != 1 ||
		int(modeData["eligibleCount"].(float64)) != 2 || int(modeData["remainingInRound"].(float64)) != 1 {
		t.Fatalf("unexpected mode switch response: %v", modeData)
	}

	// Switch back to weighted and confirm pick history is still intact
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/mode", session2ID), map[string]interface{}{
		"mode": "weighted",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 switching mode back to weighted, got %d: %v", status, res)
	}
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d", session2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting session 2, got %d", status)
	}
	pickSum := res["data"].(map[string]interface{})["pickSummary"].(map[string]interface{})
	if int(pickSum["totalValidPicks"].(float64)) != 1 {
		t.Fatalf("expected totalValidPicks=1 preserved after mode switch, got %v", pickSum)
	}

	// 11. End Session 2 with markUnconfirmedAsAbsent = false (keep unconfirmed as-is and return warning)
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", session2ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 ending session 2, got %d: %v", status, res)
	}
	end2Data := res["data"].(map[string]interface{})
	if end2Data["status"] != "ended" || end2Data["endedAt"] == nil ||
		int(end2Data["unconfirmedRemainingCount"].(float64)) != 1 ||
		end2Data["hasUnconfirmedWarning"] != true || end2Data["warningMessage"] == nil {
		t.Fatalf("expected ended session with unconfirmed warning, got %v", end2Data)
	}

	// 12. Forbid any modifications to ended Session 2 -> 409 SESSION_ALREADY_ENDED
	// 12a. End again
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", session2ID), nil)
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on ending ended session, got %d: %v", status, res)
	}
	// 12b. Switch mode on ended session
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/mode", session2ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on switching mode of ended session, got %d: %v", status, res)
	}
	// 12c. Update single attendance on ended session
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", session2ID, studentIDs[0]), map[string]interface{}{
		"status": "absent",
	})
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on single attendance update of ended session, got %d: %v", status, res)
	}
	// 12d. Bulk update attendance on ended session
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", session2ID), map[string]interface{}{
		"markAllStatus": "present",
	})
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on bulk attendance update of ended session, got %d: %v", status, res)
	}

	// 13. Start Session 3 after Session 2 ended -> sessionNumber = 3, test Bulk Update & markUnconfirmedAsAbsent = true
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Unit 3: Listening",
		"initialMode": "no_repeat",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 starting session 3, got %d: %v", status, res)
	}
	session3Data := res["data"].(map[string]interface{})
	session3ID := uint64(session3Data["id"].(float64))
	if int(session3Data["sessionNumber"].(float64)) != 3 || session3Data["currentMode"] != "no_repeat" {
		t.Fatalf("expected sessionNumber=3 and mode=no_repeat, got %v", session3Data)
	}

	// Bulk update markAllStatus = present
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", session3ID), map[string]interface{}{
		"markAllStatus": "present",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 on bulk markAllStatus=present, got %d: %v", status, res)
	}
	bulkSum := res["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(bulkSum["present"].(float64)) != 3 || int(bulkSum["eligibleForPick"].(float64)) != 3 || int(bulkSum["unconfirmed"].(float64)) != 0 {
		t.Fatalf("expected 3 present and 0 unconfirmed after markAllStatus, got %v", bulkSum)
	}

	// Bulk update specific items: set student 2 to late and student 3 back to unconfirmed
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", session3ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": studentIDs[1], "status": "late", "note": "ติดธุระ"},
			{"studentId": studentIDs[2], "status": "unconfirmed"},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 on bulk items update, got %d: %v", status, res)
	}
	bulkSum2 := res["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(bulkSum2["present"].(float64)) != 1 || int(bulkSum2["late"].(float64)) != 1 || int(bulkSum2["unconfirmed"].(float64)) != 1 {
		t.Fatalf("expected 1 present, 1 late, 1 unconfirmed, got %v", bulkSum2)
	}

	// End Session 3 with markUnconfirmedAsAbsent = true -> converts student 3 from unconfirmed to absent
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", session3ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": true,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 ending session 3 with markUnconfirmedAsAbsent=true, got %d: %v", status, res)
	}
	end3Data := res["data"].(map[string]interface{})
	if int(end3Data["unconfirmedRemainingCount"].(float64)) != 0 || end3Data["hasUnconfirmedWarning"] != false || end3Data["warningMessage"] != nil {
		t.Fatalf("expected 0 unconfirmed remaining and no warning when markUnconfirmedAsAbsent=true, got %v", end3Data)
	}
	end3Sum := end3Data["attendanceSummary"].(map[string]interface{})
	if int(end3Sum["absent"].(float64)) != 1 || int(end3Sum["unconfirmed"].(float64)) != 0 {
		t.Fatalf("expected absent=1 and unconfirmed=0, got %v", end3Sum)
	}

	// 14. List sessions of the course (GET /api/courses/:courseId/sessions)
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/sessions?status=ended", courseID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 listing course sessions, got %d: %v", status, res)
	}
	sessionList := res["data"].([]interface{})
	if len(sessionList) != 3 {
		t.Fatalf("expected 3 ended sessions in list, got %d", len(sessionList))
	}
}

func TestPhase2ConcurrentStartAndEndSessionRaceCondition(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	status, res := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204200",
		"courseName": "Concurrency Test Course",
		"section":    "001",
		"semester":   "1/2569",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 creating course, got %d: %v", status, res)
	}
	courseID := uint64(res["data"].(map[string]interface{})["id"].(float64))

	// Fire 5 concurrent POST /api/courses/:courseId/sessions requests
	const workers = 5
	var wg sync.WaitGroup
	statuses := make([]int, workers)
	responses := make([]map[string]interface{}, workers)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			st, r := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
				"topic": fmt.Sprintf("Concurrent Attempt %d", idx+1),
			})
			statuses[idx] = st
			responses[idx] = r
		}(i)
	}
	wg.Wait()

	createdCount := 0
	conflictCount := 0
	var activeSessionID uint64
	for i := 0; i < workers; i++ {
		switch statuses[i] {
		case http.StatusCreated:
			createdCount++
			activeSessionID = uint64(responses[i]["data"].(map[string]interface{})["id"].(float64))
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected status %d in concurrent start session: %v", statuses[i], responses[i])
		}
	}

	if createdCount != 1 || conflictCount != workers-1 {
		t.Fatalf("expected exactly 1 Created (201) and %d Conflict (409), got created=%d conflict=%d", workers-1, createdCount, conflictCount)
	}

	// Fire 5 concurrent PATCH /api/sessions/:sessionId/end requests
	endStatuses := make([]int, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			st, _ := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", activeSessionID), map[string]interface{}{
				"markUnconfirmedAsAbsent": true,
			})
			endStatuses[idx] = st
		}(i)
	}
	wg.Wait()

	okCount := 0
	endConflictCount := 0
	for i := 0; i < workers; i++ {
		switch endStatuses[i] {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			endConflictCount++
		default:
			t.Fatalf("unexpected status %d in concurrent end session", endStatuses[i])
		}
	}

	if okCount != 1 || endConflictCount != workers-1 {
		t.Fatalf("expected exactly 1 OK (200) and %d Conflict (409) when ending session concurrently, got ok=%d conflict=%d", workers-1, okCount, endConflictCount)
	}
}
