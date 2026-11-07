package handlers_test

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"testing"
)

func TestPhase3WeightedRandomMathAndHistoryWindow(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	// Create course
	status, res := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204301",
		"courseName": "Algorithms and Weighted Randomness",
		"section":    "001",
		"semester":   "1/2569",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 creating course, got %d: %v", status, res)
	}
	courseID := uint64(res["data"].(map[string]interface{})["id"].(float64))

	// Create 4 students enrolled in course + 1 unenrolled student
	var sIDs []uint64
	for i := 1; i <= 4; i++ {
		st, sRes := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
			"studentCode": fmt.Sprintf("66053000%d", i),
			"fullName":    fmt.Sprintf("นักศึกษา สุ่ม %d", i),
			"courseId":    courseID,
		})
		if st != http.StatusCreated {
			t.Fatalf("expected 201 creating student %d, got %d: %v", i, st, sRes)
		}
		sIDs = append(sIDs, uint64(sRes["data"].(map[string]interface{})["id"].(float64)))
	}

	// Build 5 ended historical sessions (Session 1..5) to verify ONLY the 4 most recent sessions (2..5) are used for H in Session 6!
	var historicalSessionIDs []uint64
	for sNum := 1; sNum <= 5; sNum++ {
		st, sRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
			"topic":       fmt.Sprintf("Historical Session %d", sNum),
			"initialMode": "weighted",
		})
		if st != http.StatusCreated {
			t.Fatalf("expected 201 starting session %d, got %d: %v", sNum, st, sRes)
		}
		sessID := uint64(sRes["data"].(map[string]interface{})["id"].(float64))
		historicalSessionIDs = append(historicalSessionIDs, sessID)

		if sNum == 1 {
			// In Session 1 (oldest, outside 4-session window when at Session 6): make only Student 1 present and pick Student 1 twice
			doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", sessID, sIDs[0]), map[string]interface{}{
				"status": "present",
			})
			for k := 0; k < 2; k++ {
				pSt, pRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessID), map[string]interface{}{
					"mode": "weighted",
				})
				if pSt != http.StatusCreated {
					t.Fatalf("expected 201 picking in session 1, got %d: %v", pSt, pRes)
				}
			}
		} else if sNum == 2 || sNum == 3 {
			// In Sessions 2 and 3 (inside 4-session window): make only Student 2 present and pick Student 2 once per session (total H=2 for Student 2)
			doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", sessID, sIDs[1]), map[string]interface{}{
				"status": "present",
			})
			pSt, pRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessID), map[string]interface{}{
				"mode": "no_repeat", // Even a no_repeat valid pick must be counted in H!
			})
			if pSt != http.StatusCreated {
				t.Fatalf("expected 201 picking student 2 in session %d, got %d: %v", sNum, pSt, pRes)
			}
		} else {
			// Sessions 4 & 5: mark Student 1 and Student 2 present
			doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sessID), map[string]interface{}{
				"markAllStatus": "present",
			})
		}

		// End session
		eSt, eRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sessID), map[string]interface{}{
			"markUnconfirmedAsAbsent": true,
		})
		if eSt != http.StatusOK {
			t.Fatalf("expected 200 ending session %d, got %d: %v", sNum, eSt, eRes)
		}
	}

	// Start Session 6
	status, s6Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Session 6 Active",
		"initialMode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 starting session 6, got %d: %v", status, s6Res)
	}
	session6ID := uint64(s6Res["data"].(map[string]interface{})["id"].(float64))

	// Initially all students in Session 6 are unconfirmed -> attempting to pick must fail with 422 NO_ELIGIBLE_STUDENTS
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", session6ID), map[string]interface{}{
		"mode": "weighted",
	})
	if status != http.StatusUnprocessableEntity || res["error"].(map[string]interface{})["code"] != "NO_ELIGIBLE_STUDENTS" {
		t.Fatalf("expected 422 NO_ELIGIBLE_STUDENTS when all unconfirmed, got %d: %v", status, res)
	}

	// Set attendance in Session 6:
	// - Student 1: present
	// - Student 2: late
	// - Student 3: absent (must NOT be in pool)
	// - Student 4: unconfirmed (must NOT be in pool, then withdrawn)
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", session6ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": sIDs[0], "status": "present"},
			{"studentId": sIDs[1], "status": "late"},
			{"studentId": sIDs[2], "status": "absent"},
		},
	})
	// Withdraw Student 4
	doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d/enrollments/%d", courseID, sIDs[3]), nil)

	// Check GET /api/sessions/:sessionId/picks/pool
	// In Session 6:
	// - previousSessionsUsed must be [Session5, Session4, Session3, Session2] (excluding Session 1!)
	// - Student 1: H = 0 (since Session 1 is outside the 4-session window), C = 0 -> W = 1 / (1 + 0 + 0) = 1.0
	// - Student 2: H = 2 (picked in Session 2 & 3), C = 0 -> W = 1 / (1 + 2 + 0) = 0.3333 (raw 1/3)
	// - Total raw weight = 1 + 1/3 = 4/3 -> Student 1 probability = 75.0%, Student 2 probability = 25.0%
	status, poolRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/picks/pool", session6ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting pick pool, got %d: %v", status, poolRes)
	}
	poolData := poolRes["data"].(map[string]interface{})
	prevUsed := poolData["previousSessionsUsed"].([]interface{})
	if len(prevUsed) != 4 || uint64(prevUsed[0].(float64)) != historicalSessionIDs[4] || uint64(prevUsed[3].(float64)) != historicalSessionIDs[1] {
		t.Fatalf("expected previousSessionsUsed to be sessions [5,4,3,2], got %v", prevUsed)
	}
	poolList := poolData["pool"].([]interface{})
	if len(poolList) != 2 {
		t.Fatalf("expected exactly 2 eligible students (present & late, excluding absent & withdrawn), got %d", len(poolList))
	}

	cand1 := poolList[0].(map[string]interface{})
	cand2 := poolList[1].(map[string]interface{})
	if int(cand1["recent4SessionsPickCount"].(float64)) != 0 || cand1["weightScore"].(float64) != 1.0 || cand1["probabilityPercent"].(float64) != 75.0 {
		t.Fatalf("unexpected candidate 1 pool metrics: %v", cand1)
	}
	if int(cand2["recent4SessionsPickCount"].(float64)) != 2 || cand2["weightScore"].(float64) != 0.3333 || cand2["probabilityPercent"].(float64) != 25.0 {
		t.Fatalf("unexpected candidate 2 pool metrics: %v", cand2)
	}

	// Execute a pick in Session 6 and verify C increases by 1 and weight drops according to W = 1 / (1 + H + 2C)
	status, pick1Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", session6ID), map[string]interface{}{
		"mode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 executing weighted pick, got %d: %v", status, pick1Res)
	}
	pick1Obj := pick1Res["data"].(map[string]interface{})["pick"].(map[string]interface{})
	firstPickedID := uint64(pick1Obj["student"].(map[string]interface{})["id"].(float64))
	firstPickID := uint64(pick1Obj["id"].(float64))

	// Check pool again: the picked student's currentSessionPickCount (C) must now be 1, and weight reduced!
	status, poolAfter1 := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/picks/pool", session6ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting pool after pick 1, got %d", status)
	}
	for _, rawCand := range poolAfter1["data"].(map[string]interface{})["pool"].([]interface{}) {
		cMap := rawCand.(map[string]interface{})
		sID := uint64(cMap["studentId"].(float64))
		if sID == firstPickedID {
			if int(cMap["currentSessionPickCount"].(float64)) != 1 {
				t.Fatalf("expected C=1 for picked student %d, got %v", sID, cMap["currentSessionPickCount"])
			}
			h := int(cMap["recent4SessionsPickCount"].(float64))
			expectedW := math.Round((1.0/float64(1+h+2*1))*10000) / 10000
			if cMap["weightScore"].(float64) != expectedW {
				t.Fatalf("expected weightScore=%v after C=1, got %v", expectedW, cMap["weightScore"])
			}
		}
	}

	// Cancel that first pick and verify C returns to 0 and weight is restored to 1.0 / 0.3333!
	status, cancelRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", firstPickID), map[string]interface{}{
		"cancelReason": "  ทดสอบยกเลิกผลสุ่ม  ",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 cancelling pick, got %d: %v", status, cancelRes)
	}
	cancelData := cancelRes["data"].(map[string]interface{})
	if cancelData["status"] != "cancelled" || cancelData["cancelReason"] != "ทดสอบยกเลิกผลสุ่ม" || cancelData["cancelledAt"] == nil {
		t.Fatalf("unexpected cancel pick data: %v", cancelData)
	}

	// Prevent cancelling the same pick twice -> 409 PICK_ALREADY_CANCELLED
	status, dupCancel := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", firstPickID), nil)
	if status != http.StatusConflict || dupCancel["error"].(map[string]interface{})["code"] != "PICK_ALREADY_CANCELLED" {
		t.Fatalf("expected 409 PICK_ALREADY_CANCELLED on second cancel, got %d: %v", status, dupCancel)
	}

	// Verify pool after cancel is back to C=0 for both students
	status, poolAfterCancel := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/picks/pool", session6ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting pool after cancel, got %d", status)
	}
	poolListAfterCancel := poolAfterCancel["data"].(map[string]interface{})["pool"].([]interface{})
	if int(poolListAfterCancel[0].(map[string]interface{})["currentSessionPickCount"].(float64)) != 0 ||
		int(poolListAfterCancel[1].(map[string]interface{})["currentSessionPickCount"].(float64)) != 0 {
		t.Fatalf("expected C=0 for both candidates after cancelling pick, got %v", poolListAfterCancel)
	}

	// Run 30 weighted picks in Session 6: verify ONLY eligible students (Student 1 and Student 2) are ever chosen,
	// and both can be picked multiple times while keeping weights dynamically balanced.
	pickCounts := make(map[uint64]int)
	for i := 0; i < 30; i++ {
		st, pRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", session6ID), map[string]interface{}{
			"mode": "weighted",
		})
		if st != http.StatusCreated {
			t.Fatalf("expected 201 on weighted pick %d, got %d: %v", i, st, pRes)
		}
		pObj := pRes["data"].(map[string]interface{})["pick"].(map[string]interface{})
		pickedID := uint64(pObj["student"].(map[string]interface{})["id"].(float64))
		if pickedID != sIDs[0] && pickedID != sIDs[1] {
			t.Fatalf("ineligible student %d was picked in weighted mode!", pickedID)
		}
		pickCounts[pickedID]++
	}
	if pickCounts[sIDs[0]] == 0 || pickCounts[sIDs[1]] == 0 {
		t.Fatalf("expected both eligible students to be picked across 30 weighted draws, got counts=%v", pickCounts)
	}
}

func TestPhase3NoRepeatRoundsModeSwitchAndConcurrency(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	// Create course and 3 students
	status, res := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204302",
		"courseName": "No Repeat & Concurrency Verification",
		"section":    "001",
		"semester":   "1/2569",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 creating course, got %d", status)
	}
	courseID := uint64(res["data"].(map[string]interface{})["id"].(float64))

	var sIDs []uint64
	for i := 1; i <= 3; i++ {
		st, sRes := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
			"studentCode": fmt.Sprintf("66054000%d", i),
			"fullName":    fmt.Sprintf("นักศึกษา ไม่ซ้ำ %d", i),
			"courseId":    courseID,
		})
		if st != http.StatusCreated {
			t.Fatalf("expected 201 creating student %d, got %d", i, st)
		}
		sIDs = append(sIDs, uint64(sRes["data"].(map[string]interface{})["id"].(float64)))
	}

	// Start Session 1 in no_repeat mode and mark all 3 students present
	status, sRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "No Repeat Round Test",
		"initialMode": "no_repeat",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 starting session, got %d", status)
	}
	sessionID := uint64(sRes["data"].(map[string]interface{})["id"].(float64))

	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sessionID), map[string]interface{}{
		"markAllStatus": "present",
	})

	// Round 1: Fire 5 concurrent requests in no_repeat mode for 3 eligible students!
	// Exactly 3 must succeed (201 Created) with 3 DISTINCT students, and 2 must receive 422 ROUND_COMPLETED!
	const workers = 5
	var wg sync.WaitGroup
	statuses := make([]int, workers)
	responses := make([]map[string]interface{}, workers)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			st, r := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
				"mode":             "no_repeat",
				"autoAdvanceRound": false,
			})
			statuses[idx] = st
			responses[idx] = r
		}(i)
	}
	wg.Wait()

	pickedInRound1 := make(map[uint64]uint64) // studentID -> pickID
	roundCompletedErrors := 0
	for i := 0; i < workers; i++ {
		switch statuses[i] {
		case http.StatusCreated:
			pObj := responses[i]["data"].(map[string]interface{})["pick"].(map[string]interface{})
			stID := uint64(pObj["student"].(map[string]interface{})["id"].(float64))
			pID := uint64(pObj["id"].(float64))
			if _, exists := pickedInRound1[stID]; exists {
				t.Fatalf("duplicate student %d picked in the same no_repeat round!", stID)
			}
			pickedInRound1[stID] = pID
		case http.StatusUnprocessableEntity:
			if responses[i]["error"].(map[string]interface{})["code"] != "ROUND_COMPLETED" {
				t.Fatalf("expected error code ROUND_COMPLETED, got %v", responses[i]["error"])
			}
			roundCompletedErrors++
		default:
			t.Fatalf("unexpected status %d in concurrent no_repeat picks: %v", statuses[i], responses[i])
		}
	}

	if len(pickedInRound1) != 3 || roundCompletedErrors != 2 {
		t.Fatalf("expected 3 unique picks and 2 ROUND_COMPLETED errors, got uniquePicks=%d errors=%d", len(pickedInRound1), roundCompletedErrors)
	}

	// Verify picking again still returns 422 ROUND_COMPLETED
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusUnprocessableEntity || res["error"].(map[string]interface{})["code"] != "ROUND_COMPLETED" {
		t.Fatalf("expected 422 ROUND_COMPLETED after all 3 picked, got %d: %v", status, res)
	}

	// Cancel one pick from Round 1 -> that student must immediately become eligible again in Round 1!
	var cancelledStudentID, cancelledPickID uint64
	for stID, pID := range pickedInRound1 {
		cancelledStudentID = stID
		cancelledPickID = pID
		break
	}
	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", cancelledPickID), map[string]interface{}{
		"cancelReason": "ขอข้ามชั่วคราว",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 cancelling pick in round 1, got %d: %v", status, res)
	}

	// Now pick again in Round 1: must pick the exact student whose pick was cancelled, completing Round 1 again!
	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 re-picking remaining student in round 1, got %d: %v", status, res)
	}
	repickedStudentID := uint64(res["data"].(map[string]interface{})["pick"].(map[string]interface{})["student"].(map[string]interface{})["id"].(float64))
	poolStateAfterRepick := res["data"].(map[string]interface{})["poolState"].(map[string]interface{})
	if repickedStudentID != cancelledStudentID || poolStateAfterRepick["isRoundCompleted"] != true || int(poolStateAfterRepick["remainingInRound"].(float64)) != 0 {
		t.Fatalf("expected repicked student=%d and isRoundCompleted=true, got student=%d poolState=%v", cancelledStudentID, repickedStudentID, poolStateAfterRepick)
	}

	// Start New Round (Round 2) via POST /api/sessions/:sessionId/picks/new-round
	status, nrRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks/new-round", sessionID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 starting new round, got %d: %v", status, nrRes)
	}
	nrData := nrRes["data"].(map[string]interface{})
	if int(nrData["previousRound"].(float64)) != 1 || int(nrData["currentRound"].(float64)) != 2 ||
		nrData["isRoundCompleted"] != false || int(nrData["remainingInRound"].(float64)) != 3 {
		t.Fatalf("unexpected start new round response: %v", nrData)
	}

	// Pick 1 student in Round 2, then switch to weighted mode, make 1 weighted pick, then switch back to no_repeat:
	// Verify currentRound is still 2 and remainingInRound in no_repeat is still 2!
	status, r2p1 := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 picking first student in round 2, got %d: %v", status, r2p1)
	}

	// Switch to weighted mode and pick 1 student
	status, wPick := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
		"mode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 picking in weighted mode mid-session, got %d: %v", status, wPick)
	}

	// Switch mode back to no_repeat via PATCH /api/sessions/:sessionId/mode
	status, modeRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/mode", sessionID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 switching mode back to no_repeat, got %d: %v", status, modeRes)
	}
	mData := modeRes["data"].(map[string]interface{})
	if int(mData["currentRound"].(float64)) != 2 || int(mData["remainingInRound"].(float64)) != 2 {
		t.Fatalf("expected currentRound=2 and remainingInRound=2 preserved across mode switch, got %v", mData)
	}

	// End session and verify that picking, starting new round, or cancelling a pick on an ended session all return 409 SESSION_ALREADY_ENDED
	validPickID := uint64(wPick["data"].(map[string]interface{})["pick"].(map[string]interface{})["id"].(float64))
	status, _ = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sessionID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 ending session, got %d", status)
	}

	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessionID), map[string]interface{}{
		"mode": "weighted",
	})
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on pick in ended session, got %d: %v", status, res)
	}

	status, res = doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks/new-round", sessionID), nil)
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on new-round in ended session, got %d: %v", status, res)
	}

	status, res = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", validPickID), nil)
	if status != http.StatusConflict || res["error"].(map[string]interface{})["code"] != "SESSION_ALREADY_ENDED" {
		t.Fatalf("expected 409 SESSION_ALREADY_ENDED on cancel pick in ended session, got %d: %v", status, res)
	}
}

func TestPhase3PickHistoryAndStatisticsIsolation(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	// Create Course A and Course B to test course isolation and full statistics accuracy
	_, cARes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204401",
		"courseName": "Course A Statistics",
		"section":    "001",
		"semester":   "1/2569",
	})
	courseAID := uint64(cARes["data"].(map[string]interface{})["id"].(float64))

	_, cBRes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204402",
		"courseName": "Course B Isolation",
		"section":    "001",
		"semester":   "1/2569",
	})
	courseBID := uint64(cBRes["data"].(map[string]interface{})["id"].(float64))

	// Enroll Student 1 & Student 2 in Course A from the start; Student 3 will enroll mid-semester (in Session 2)
	_, s1Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660550001",
		"fullName":    "สมชาย มาตลอด",
		"courseId":    courseAID,
	})
	s1ID := uint64(s1Res["data"].(map[string]interface{})["id"].(float64))

	_, s2Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660550002",
		"fullName":    "สมหญิง สายและขาด",
		"courseId":    courseAID,
	})
	s2ID := uint64(s2Res["data"].(map[string]interface{})["id"].(float64))

	_, s3Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660550003",
		"fullName":    "สมศักดิ์ ลงกลางเทอม",
	})
	s3ID := uint64(s3Res["data"].(map[string]interface{})["id"].(float64))

	// Also enroll Student 1 in Course B
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseBID), map[string]interface{}{
		"studentId": s1ID,
	})

	// --- Course A Session 1 ---
	// Student 1: present, Student 2: absent (Student 3 not enrolled yet!)
	_, sess1Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseAID), map[string]interface{}{
		"topic": "Course A - Week 1",
	})
	sessA1ID := uint64(sess1Res["data"].(map[string]interface{})["id"].(float64))
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sessA1ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": s1ID, "status": "present"},
			{"studentId": s2ID, "status": "absent"},
		},
	})
	// Pick Student 1 once (weighted) and once more then cancel
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessA1ID), map[string]interface{}{
		"mode": "weighted",
	})
	_, pCancelRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessA1ID), map[string]interface{}{
		"mode": "weighted",
	})
	cancelPickID := uint64(pCancelRes["data"].(map[string]interface{})["pick"].(map[string]interface{})["id"].(float64))
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", cancelPickID), map[string]interface{}{
		"cancelReason": "ไม่อยู่ที่โต๊ะ",
	})
	// End Course A Session 1
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sessA1ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})

	// --- Enroll Student 3 mid-semester before Course A Session 2 ---
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseAID), map[string]interface{}{
		"studentId": s3ID,
	})

	// --- Course A Session 2 ---
	// Student 1: present, Student 2: late, Student 3: unconfirmed (kept unconfirmed on session end!)
	_, sess2Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseAID), map[string]interface{}{
		"topic": "Course A - Week 2",
	})
	sessA2ID := uint64(sess2Res["data"].(map[string]interface{})["id"].(float64))
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sessA2ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": s1ID, "status": "present"},
			{"studentId": s2ID, "status": "late", "note": "มาสาย 5 นาที"},
		},
	})
	// Pick in no_repeat mode twice -> both Student 1 and Student 2 are picked once in no_repeat mode
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessA2ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessA2ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	// End Course A Session 2 keeping Student 3 as unconfirmed
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sessA2ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})

	// --- Course B Session 1 (to verify no cross-course data leakage) ---
	_, sessB1Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseBID), map[string]interface{}{
		"topic": "Course B - Week 1",
	})
	sessB1ID := uint64(sessB1Res["data"].(map[string]interface{})["id"].(float64))
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance/%d", sessB1ID, s1ID), map[string]interface{}{
		"status": "present",
	})
	doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sessB1ID), map[string]interface{}{
		"mode": "weighted",
	})
	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sessB1ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})

	// 1. Verify Session Pick History (GET /api/sessions/:sessionId/picks)
	status, s1PicksRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/picks", sessA1ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting session 1 picks, got %d: %v", status, s1PicksRes)
	}
	s1Summary := s1PicksRes["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(s1Summary["totalPicks"].(float64)) != 2 || int(s1Summary["validPicks"].(float64)) != 1 || int(s1Summary["cancelledPicks"].(float64)) != 1 {
		t.Fatalf("expected 2 total, 1 valid, 1 cancelled in session A1, got %v", s1Summary)
	}

	// 2. Verify Course Pick History (GET /api/courses/:courseId/picks) does not include Course B picks
	status, cAPicksRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/picks?status=valid", courseAID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting course A picks, got %d: %v", status, cAPicksRes)
	}
	cAPickItems := cAPicksRes["data"].([]interface{})
	if len(cAPickItems) != 3 {
		t.Fatalf("expected 3 valid picks in Course A (1 in S1 + 2 in S2), got %d", len(cAPickItems))
	}

	// 3. Verify Course Overview Statistics (GET /api/courses/:courseId/stats/overview)
	// In Course A (ended sessions 1 & 2):
	// - S1: 1 present, 1 absent (confirmed=2, attended=1)
	// - S2: 1 present, 1 late, 1 unconfirmed (confirmed=2, attended=2)
	// Total confirmed across ended sessions = 4, total attended = 3 -> averageAttendanceRatePercent = 75.0%
	// Total valid picks = 3 (1 weighted, 2 no_repeat), cancelled = 1, studentsNeverPickedCount = 1 (Student 3)
	status, ovRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/stats/overview", courseAID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting course A overview stats, got %d: %v", status, ovRes)
	}
	overall := ovRes["data"].(map[string]interface{})["overall"].(map[string]interface{})
	if int(overall["totalActiveStudents"].(float64)) != 3 ||
		int(overall["totalSessions"].(float64)) != 2 ||
		overall["averageAttendanceRatePercent"].(float64) != 75.0 ||
		int(overall["totalValidPicks"].(float64)) != 3 ||
		int(overall["totalCancelledPicks"].(float64)) != 1 ||
		int(overall["weightedValidPicks"].(float64)) != 1 ||
		int(overall["noRepeatValidPicks"].(float64)) != 2 ||
		int(overall["studentsNeverPickedCount"].(float64)) != 1 {
		t.Fatalf("unexpected course A overview stats: %v", overall)
	}

	// 4. Verify Student Stats Table (GET /api/courses/:courseId/stats/students)
	// - Student 1: 2 present -> 100.0%, 2 valid picks (1 weighted, 1 no_repeat), 1 cancelled
	// - Student 2: 1 late, 1 absent -> 50.0%, 1 valid pick (no_repeat)
	// - Student 3 (enrolled mid-semester in S2, left unconfirmed):
	//   totalSessions=1 (NOT 2! S1 not counted as absent!), confirmedSessionsCount=0, unconfirmedCount=1, attendanceRatePercent=0.0%
	status, stListRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/stats/students?sortBy=studentCode&sortOrder=asc", courseAID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting course A student stats, got %d: %v", status, stListRes)
	}
	stItems := stListRes["data"].([]interface{})
	if len(stItems) != 3 {
		t.Fatalf("expected 3 students in course A stats, got %d", len(stItems))
	}

	st1Stat := stItems[0].(map[string]interface{})
	st1Att := st1Stat["attendance"].(map[string]interface{})
	st1Picks := st1Stat["picks"].(map[string]interface{})
	if st1Att["attendanceRatePercent"].(float64) != 100.0 || int(st1Picks["totalValidPicks"].(float64)) != 2 || int(st1Picks["cancelledPicks"].(float64)) != 1 {
		t.Fatalf("unexpected student 1 stats: %v", st1Stat)
	}

	st2Stat := stItems[1].(map[string]interface{})
	st2Att := st2Stat["attendance"].(map[string]interface{})
	if st2Att["attendanceRatePercent"].(float64) != 50.0 || int(st2Att["lateCount"].(float64)) != 1 || int(st2Att["absentCount"].(float64)) != 1 {
		t.Fatalf("unexpected student 2 stats: %v", st2Stat)
	}

	st3Stat := stItems[2].(map[string]interface{})
	st3Att := st3Stat["attendance"].(map[string]interface{})
	if int(st3Att["totalSessions"].(float64)) != 1 ||
		int(st3Att["confirmedSessionsCount"].(float64)) != 0 ||
		int(st3Att["absentCount"].(float64)) != 0 ||
		int(st3Att["unconfirmedCount"].(float64)) != 1 ||
		st3Att["attendanceRatePercent"].(float64) != 0.0 {
		t.Fatalf("expected mid-semester Student 3 to have totalSessions=1, absent=0, unconfirmed=1, rate=0.0, got %v", st3Att)
	}

	// 5. Verify Individual Student Stats Timeline (GET /api/courses/:courseId/stats/students/:studentId)
	status, s1DetailRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/stats/students/%d", courseAID, s1ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 getting individual student 1 stats in course A, got %d: %v", status, s1DetailRes)
	}
	s1Detail := s1DetailRes["data"].(map[string]interface{})
	attHist := s1Detail["attendanceHistory"].([]interface{})
	pickHist := s1Detail["pickHistory"].([]interface{})
	if len(attHist) != 2 || len(pickHist) != 3 {
		t.Fatalf("expected 2 attendance history items and 3 pick history items (2 valid + 1 cancelled) in Course A for Student 1, got att=%d pick=%d", len(attHist), len(pickHist))
	}
}

