package handlers_test

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/importer"
)

func TestPhase4CORSPreflightAndOriginRestriction(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	// 1. Preflight OPTIONS from allowed Frontend origin http://localhost:3000
	req := httptest.NewRequest(http.MethodOptions, "/api/courses", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Accept")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("CORS preflight request failed: %v", err)
	}
	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:3000" {
		t.Fatalf("expected Access-Control-Allow-Origin='http://localhost:3000', got %q", allowOrigin)
	}
	allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(allowMethods, m) {
			t.Fatalf("expected Access-Control-Allow-Methods to include %s, got %q", m, allowMethods)
		}
	}
	allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowHeaders), "content-type") {
		t.Fatalf("expected Access-Control-Allow-Headers to include Content-Type, got %q", allowHeaders)
	}

	// 2. Actual GET request from http://localhost:3000 includes Access-Control-Allow-Origin: http://localhost:3000
	getReq := httptest.NewRequest(http.MethodGet, "/api/courses", nil)
	getReq.Header.Set("Origin", "http://localhost:3000")
	getResp, err := app.Test(getReq, 5000)
	if err != nil {
		t.Fatalf("CORS GET request failed: %v", err)
	}
	defer getResp.Body.Close()
	if got := getResp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected Access-Control-Allow-Origin='http://localhost:3000' on GET, got %q", got)
	}

	// 3. Disallowed origin must NOT receive Access-Control-Allow-Origin or wildcard '*'
	evilReq := httptest.NewRequest(http.MethodGet, "/api/courses", nil)
	evilReq.Header.Set("Origin", "http://untrusted-origin.example.com")
	evilResp, err := app.Test(evilReq, 5000)
	if err != nil {
		t.Fatalf("disallowed origin request failed: %v", err)
	}
	defer evilResp.Body.Close()
	if got := evilResp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected empty Access-Control-Allow-Origin for untrusted origin, got %q", got)
	}
}

func TestPhase4FullEndToEndWorkflowAndDBConsistency(t *testing.T) {
	app, db := setupPhase1TestApp(t)

	// =========================================================================
	// Step 1: สร้างรายวิชาใหม่ (พร้อมทดสอบกรณีผิดพลาด)
	// =========================================================================
	status, badCourseRes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "CS101-INVALID!",
		"courseName": "Invalid Course Code",
	})
	if status != http.StatusBadRequest || badCourseRes["error"].(map[string]interface{})["code"] != "VALIDATION_ERROR" {
		t.Fatalf("Step 1 error check: expected 400 VALIDATION_ERROR, got %d: %v", status, badCourseRes)
	}

	status, courseRes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204499",
		"courseName": "Full-Stack Software Engineering",
		"section":    "001",
		"semester":   "2/2569",
	})
	if status != http.StatusCreated {
		t.Fatalf("Step 1: expected 201 creating course, got %d: %v", status, courseRes)
	}
	courseID := uint64(courseRes["data"].(map[string]interface{})["id"].(float64))

	// =========================================================================
	// Step 2: เพิ่มนักศึกษาและนำเข้ารายชื่อหลายคน (Single + Bulk Paste + XLSX File)
	// =========================================================================
	// 2a. Create 1 student directly into course
	status, s1Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660590001",
		"fullName":    "กิตติภพ หนึ่ง",
		"courseId":    courseID,
	})
	if status != http.StatusCreated {
		t.Fatalf("Step 2a: expected 201 creating student 1, got %d: %v", status, s1Res)
	}

	// 2b. Bulk Paste 2 students
	status, bulkRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/students/bulk", courseID), map[string]interface{}{
		"rawText": "660590002, ขวัญชนก สอง\n660590003\tคณิศร สาม",
	})
	if status != http.StatusOK {
		t.Fatalf("Step 2b: expected 200 on bulk paste, got %d: %v", status, bulkRes)
	}

	// 2c. Import .xlsx file with 1 more student (660590004)
	xlsxBytes, err := importer.BuildSampleXLSX([][]string{
		{"student_code", "full_name"},
		{"660590004", "ฆฤณ สี่"},
	})
	if err != nil {
		t.Fatalf("Step 2c: failed building sample xlsx: %v", err)
	}
	status, importRes := doMultipartFileRequest(t, app, fmt.Sprintf("/api/courses/%d/students/import-file", courseID), "file", "roster.xlsx", xlsxBytes)
	if status != http.StatusOK {
		t.Fatalf("Step 2c: expected 200 on xlsx import, got %d: %v", status, importRes)
	}

	// =========================================================================
	// Step 3: ตรวจสอบการลงทะเบียน
	// =========================================================================
	status, enrListRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/students", courseID), nil)
	if status != http.StatusOK {
		t.Fatalf("Step 3: expected 200 listing course students, got %d: %v", status, enrListRes)
	}
	enrolledItems := enrListRes["data"].([]interface{})
	if len(enrolledItems) != 4 {
		t.Fatalf("Step 3: expected 4 enrolled students, got %d", len(enrolledItems))
	}
	var studentIDs []uint64
	for _, raw := range enrolledItems {
		m := raw.(map[string]interface{})
		studentIDs = append(studentIDs, uint64(m["studentId"].(float64)))
	}

	// =========================================================================
	// Step 4: เริ่มคาบเรียนที่ 1
	// =========================================================================
	status, sess1Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Lecture 1: System Architecture",
		"initialMode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("Step 4: expected 201 starting session 1, got %d: %v", status, sess1Res)
	}
	sess1ID := uint64(sess1Res["data"].(map[string]interface{})["id"].(float64))

	// =========================================================================
	// Step 5: เช็กชื่อ present, absent, late และ unconfirmed
	// =========================================================================
	// Student 1 -> present, Student 2 -> late, Student 3 -> absent, Student 4 -> stays unconfirmed
	status, attUpdateRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sess1ID), map[string]interface{}{
		"items": []map[string]interface{}{
			{"studentId": studentIDs[0], "status": "present"},
			{"studentId": studentIDs[1], "status": "late", "note": "รถติด"},
			{"studentId": studentIDs[2], "status": "absent"},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("Step 5: expected 200 updating attendance, got %d: %v", status, attUpdateRes)
	}
	attSum := attUpdateRes["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(attSum["present"].(float64)) != 1 ||
		int(attSum["late"].(float64)) != 1 ||
		int(attSum["absent"].(float64)) != 1 ||
		int(attSum["unconfirmed"].(float64)) != 1 ||
		int(attSum["eligibleForPick"].(float64)) != 2 {
		t.Fatalf("Step 5: unexpected attendance summary: %v", attSum)
	}

	// =========================================================================
	// Step 6: สุ่มแบบ Weighted Random หลายครั้ง (เฉพาะ Student 1 และ 2 เท่านั้นที่มีสิทธิ์)
	// =========================================================================
	for i := 0; i < 4; i++ {
		st, pRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sess1ID), map[string]interface{}{
			"mode": "weighted",
		})
		if st != http.StatusCreated {
			t.Fatalf("Step 6: expected 201 on weighted pick %d, got %d: %v", i+1, st, pRes)
		}
		pickedStudent := pRes["data"].(map[string]interface{})["pick"].(map[string]interface{})["student"].(map[string]interface{})
		pID := uint64(pickedStudent["id"].(float64))
		if pID != studentIDs[0] && pID != studentIDs[1] {
			t.Fatalf("Step 6: ineligible student %d was picked!", pID)
		}
	}

	// =========================================================================
	// Step 7: เปลี่ยนโหมดเป็น No Repeat
	// =========================================================================
	status, modeRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/mode", sess1ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusOK {
		t.Fatalf("Step 7: expected 200 switching mode to no_repeat, got %d: %v", status, modeRes)
	}

	// =========================================================================
	// Step 8: สุ่มจนหมดรอบและเริ่มรอบใหม่
	// =========================================================================
	// Eligible = 2 students -> 2 picks complete Round 1
	for i := 0; i < 2; i++ {
		st, pRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sess1ID), map[string]interface{}{
			"mode": "no_repeat",
		})
		if st != http.StatusCreated {
			t.Fatalf("Step 8: expected 201 on no_repeat pick %d, got %d: %v", i+1, st, pRes)
		}
	}
	// 3rd pick in Round 1 must fail with 422 ROUND_COMPLETED
	status, roundDoneRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sess1ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusUnprocessableEntity || roundDoneRes["error"].(map[string]interface{})["code"] != "ROUND_COMPLETED" {
		t.Fatalf("Step 8: expected 422 ROUND_COMPLETED, got %d: %v", status, roundDoneRes)
	}

	// Start Round 2
	status, nrRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks/new-round", sess1ID), nil)
	if status != http.StatusOK || int(nrRes["data"].(map[string]interface{})["currentRound"].(float64)) != 2 {
		t.Fatalf("Step 8: expected 200 starting round 2, got %d: %v", status, nrRes)
	}

	// Pick 1 student in Round 2
	status, r2PickRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/sessions/%d/picks", sess1ID), map[string]interface{}{
		"mode": "no_repeat",
	})
	if status != http.StatusCreated {
		t.Fatalf("Step 8: expected 201 picking in round 2, got %d: %v", status, r2PickRes)
	}
	r2PickID := uint64(r2PickRes["data"].(map[string]interface{})["pick"].(map[string]interface{})["id"].(float64))

	// =========================================================================
	// Step 9: ยกเลิกผลการสุ่มในรอบที่ 2 และตรวจสอบผล
	// =========================================================================
	status, cancelRes := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/picks/%d/cancel", r2PickID), map[string]interface{}{
		"cancelReason": "ขออนุญาตเข้าห้องน้ำ",
	})
	if status != http.StatusOK {
		t.Fatalf("Step 9: expected 200 cancelling pick, got %d: %v", status, cancelRes)
	}

	// =========================================================================
	// Step 10: จบคาบเรียนที่ 1 (คงสถานะ unconfirmed ของ Student 4 ไว้ตามจริง)
	// =========================================================================
	status, end1Res := doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sess1ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})
	if status != http.StatusOK {
		t.Fatalf("Step 10: expected 200 ending session 1, got %d: %v", status, end1Res)
	}
	end1Data := end1Res["data"].(map[string]interface{})
	if end1Data["hasUnconfirmedWarning"] != true || int(end1Data["unconfirmedRemainingCount"].(float64)) != 1 {
		t.Fatalf("Step 10: expected unconfirmed warning with 1 remaining, got %v", end1Data)
	}

	// =========================================================================
	// Step 11: เริ่มคาบเรียนใหม่ (คาบที่ 2) และเช็กชื่อทุกคนมาเรียน
	// =========================================================================
	status, sess2Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/sessions", courseID), map[string]interface{}{
		"topic":       "Lecture 2: Database & API Integration",
		"initialMode": "weighted",
	})
	if status != http.StatusCreated {
		t.Fatalf("Step 11: expected 201 starting session 2, got %d: %v", status, sess2Res)
	}
	sess2ID := uint64(sess2Res["data"].(map[string]interface{})["id"].(float64))

	doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/attendance", sess2ID), map[string]interface{}{
		"markAllStatus": "present",
	})

	// =========================================================================
	// Step 12: ตรวจสอบประวัติย้อนหลังและการคำนวณน้ำหนักในคาบที่ 2
	// =========================================================================
	// In Session 1: total valid picks = 4 (weighted) + 2 (no_repeat R1) = 6 valid picks across Student 1 & Student 2.
	// Student 3 & Student 4 had 0 picks in Session 1 (H=0 -> W=1.0).
	status, pool2Res := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/sessions/%d/picks/pool", sess2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("Step 12: expected 200 getting session 2 pick pool, got %d: %v", status, pool2Res)
	}
	pool2List := pool2Res["data"].(map[string]interface{})["pool"].([]interface{})
	if len(pool2List) != 4 {
		t.Fatalf("Step 12: expected 4 eligible students in session 2, got %d", len(pool2List))
	}
	var sumH int
	for _, rawCand := range pool2List {
		cMap := rawCand.(map[string]interface{})
		h := int(cMap["recent4SessionsPickCount"].(float64))
		sumH += h
		expectedW := math.Round((1.0/float64(1+h))*10000) / 10000
		if cMap["weightScore"].(float64) != expectedW {
			t.Fatalf("Step 12: expected weightScore=%v for H=%d, got %v", expectedW, h, cMap["weightScore"])
		}
	}
	if sumH != 6 {
		t.Fatalf("Step 12: expected sum of H across candidates to equal 6 valid picks from Session 1, got %d", sumH)
	}

	// End Session 2
	status, _ = doJSONRequest(t, app, http.MethodPatch, fmt.Sprintf("/api/sessions/%d/end", sess2ID), map[string]interface{}{
		"markUnconfirmedAsAbsent": false,
	})
	if status != http.StatusOK {
		t.Fatalf("Step 12: expected 200 ending session 2, got %d", status)
	}

	// =========================================================================
	// Step 13: ตรวจสอบสถิติการเข้าเรียน
	// =========================================================================
	// Session 1: 1 present, 1 late, 1 absent, 1 unconfirmed (confirmed=3, attended=2)
	// Session 2: 4 present (confirmed=4, attended=4)
	// Overall across ended sessions: attended=6 / confirmed=7 -> 85.71%
	status, ovRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/stats/overview", courseID), nil)
	if status != http.StatusOK {
		t.Fatalf("Step 13: expected 200 getting course stats overview, got %d: %v", status, ovRes)
	}
	overall := ovRes["data"].(map[string]interface{})["overall"].(map[string]interface{})
	if overall["averageAttendanceRatePercent"].(float64) != 85.71 ||
		int(overall["totalValidPicks"].(float64)) != 6 ||
		int(overall["totalCancelledPicks"].(float64)) != 1 {
		t.Fatalf("Step 13: unexpected course overview stats: %v", overall)
	}

	// =========================================================================
	// Step 14: ตรวจสอบข้อมูลที่แสดงผ่าน API ว่าสอดคล้องกับ PostgreSQL จริง
	// =========================================================================
	var dbCourseCount, dbStudentCount, dbEnrollmentCount, dbSessionCount, dbAttendanceCount, dbValidPickCount, dbCancelledPickCount int64
	db.Model(&models.Course{}).Where("id = ?", courseID).Count(&dbCourseCount)
	db.Model(&models.Student{}).Count(&dbStudentCount)
	db.Model(&models.Enrollment{}).Where("course_id = ? AND status = ?", courseID, models.EnrollmentStatusActive).Count(&dbEnrollmentCount)
	db.Model(&models.ClassSession{}).Where("course_id = ?", courseID).Count(&dbSessionCount)
	db.Model(&models.Attendance{}).Where("course_id = ?", courseID).Count(&dbAttendanceCount)
	db.Model(&models.PickHistory{}).Where("course_id = ? AND status = ?", courseID, models.PickStatusValid).Count(&dbValidPickCount)
	db.Model(&models.PickHistory{}).Where("course_id = ? AND status = ?", courseID, models.PickStatusCancelled).Count(&dbCancelledPickCount)

	if dbCourseCount != 1 || dbStudentCount != 4 || dbEnrollmentCount != 4 || dbSessionCount != 2 ||
		dbAttendanceCount != 8 || dbValidPickCount != 6 || dbCancelledPickCount != 1 {
		t.Fatalf("Step 14 PostgreSQL mismatch: courses=%d students=%d enrollments=%d sessions=%d attendance=%d validPicks=%d cancelledPicks=%d",
			dbCourseCount, dbStudentCount, dbEnrollmentCount, dbSessionCount, dbAttendanceCount, dbValidPickCount, dbCancelledPickCount)
	}
}

