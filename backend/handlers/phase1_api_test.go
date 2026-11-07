package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tewan0/FairPick/database"
	"github.com/Tewan0/FairPick/models"
	"github.com/Tewan0/FairPick/pkg/importer"
	"github.com/Tewan0/FairPick/routes"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupPhase1TestApp(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	db, cfg, err := database.ConnectTestDB()
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(func() {
		database.CloseTestDB(db)
	})
	if err := database.ResetTestData(db); err != nil {
		t.Fatalf("failed to reset test database: %v", err)
	}

	app := fiber.New()
	routes.SetupWithDB(app, cfg, db)
	return app, db
}

func doJSONRequest(t *testing.T, app *fiber.App, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body for %s %s: %v", method, path, err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBytes, &parsed); err != nil {
		t.Fatalf("invalid JSON response for %s %s (status=%d): %v | raw=%s", method, path, resp.StatusCode, err, string(rawBytes))
	}

	return resp.StatusCode, parsed
}

func doMultipartFileRequest(t *testing.T, app *fiber.App, path, fieldName, fileName string, fileContent []byte) (int, map[string]interface{}) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, fileName)
		if err != nil {
			t.Fatalf("failed creating form file: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("failed writing file content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("multipart request POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBytes, &parsed); err != nil {
		t.Fatalf("invalid JSON response (status=%d): %v | raw=%s", resp.StatusCode, err, string(rawBytes))
	}

	return resp.StatusCode, parsed
}

func TestCoursesCRUDAndValidations(t *testing.T) {
	app, db := setupPhase1TestApp(t)

	// 1. Create valid course with leading zero in courseCode and whitespace to trim
	status, res := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": " 001101 ",
		"courseName": " Fundamental English 1 ",
		"section":    " 001 ",
		"semester":   " 1/2569 ",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %v", status, res)
	}
	data := res["data"].(map[string]interface{})
	course1ID := uint64(data["id"].(float64))
	if data["courseCode"] != "001101" {
		t.Fatalf("expected courseCode '001101', got %v", data["courseCode"])
	}

	// 2. Create course with only courseName (courseCode, section, semester as empty strings -> normalized to null)
	status, res = doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "   ",
		"courseName": "Special Seminar in CS",
		"section":    "",
		"semester":   "",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 Created for name-only course, got %d: %v", status, res)
	}
	data2 := res["data"].(map[string]interface{})
	course2ID := uint64(data2["id"].(float64))
	if data2["courseCode"] != nil {
		t.Fatalf("expected nil courseCode for blank input, got %v", data2["courseCode"])
	}

	// 3. Validation error: both courseCode and courseName missing
	status, res = doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "",
		"courseName": "   ",
		"section":    "001",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when both code and name are blank, got %d: %v", status, res)
	}
	errObj := res["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %v", errObj["code"])
	}

	// 4. Validation error: invalid courseCode (not 6 digits)
	status, res = doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "12345",
		"courseName": "Bad Code",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for 5-digit courseCode, got %d: %v", status, res)
	}

	// 5. Duplicate course: same (courseCode, section, semester) even with different courseName -> 409 DUPLICATE_RESOURCE
	status, res = doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "001101",
		"courseName": "Different Title Same Code Section Semester",
		"section":    "001",
		"semester":   "1/2569",
	})
	if status != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate course, got %d: %v", status, res)
	}
	if res["error"].(map[string]interface{})["code"] != "DUPLICATE_RESOURCE" {
		t.Fatalf("expected DUPLICATE_RESOURCE, got %v", res["error"])
	}

	// 6. Get Course detail & List Courses with search/pagination
	status, res = doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d", course1ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/courses/%d, got %d", course1ID, status)
	}

	status, res = doJSONRequest(t, app, http.MethodGet, "/api/courses?search=001101&page=1&limit=10", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/courses, got %d", status)
	}
	list := res["data"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 course matching search=001101, got %d", len(list))
	}

	// 7. Non-existent course -> 404 RESOURCE_NOT_FOUND
	status, res = doJSONRequest(t, app, http.MethodGet, "/api/courses/999999", nil)
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d: %v", status, res)
	}

	// 8. Update course
	status, res = doJSONRequest(t, app, http.MethodPut, fmt.Sprintf("/api/courses/%d", course1ID), map[string]interface{}{
		"courseCode": "001101",
		"courseName": "Fundamental English 1 (Updated)",
		"section":    "002",
		"semester":   "1/2569",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT /api/courses/%d, got %d: %v", course1ID, status, res)
	}

	// 9. Delete course WITHOUT session history (course2ID) -> 200 OK
	status, res = doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d", course2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK deleting history-free course, got %d: %v", status, res)
	}

	// 10. Delete course WITH session history (course1ID) -> 409 COURSE_HAS_HISTORY
	session := models.ClassSession{
		CourseID:      course1ID,
		SessionNumber: 1,
		CurrentMode:   models.PickModeWeighted,
		CurrentRound:  1,
		Status:        models.SessionStatusActive,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed creating test class session: %v", err)
	}

	status, res = doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d", course1ID), nil)
	if status != http.StatusConflict {
		t.Fatalf("expected 409 Conflict deleting course with session history, got %d: %v", status, res)
	}
	if res["error"].(map[string]interface{})["code"] != "COURSE_HAS_HISTORY" {
		t.Fatalf("expected error code COURSE_HAS_HISTORY, got %v", res["error"])
	}
}

func TestStudentsAndEnrollmentsLifecycle(t *testing.T) {
	app, db := setupPhase1TestApp(t)

	// Create a course first
	_, courseRes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "204111",
		"courseName": "Fundamentals of Programming",
		"section":    "001",
		"semester":   "1/2569",
	})
	courseID := uint64(courseRes["data"].(map[string]interface{})["id"].(float64))

	// Start an active session in this course to test automatic attendance creation upon enrollment
	activeSession := models.ClassSession{
		CourseID:      courseID,
		SessionNumber: 1,
		CurrentMode:   models.PickModeWeighted,
		CurrentRound:  1,
		Status:        models.SessionStatusActive,
	}
	if err := db.Create(&activeSession).Error; err != nil {
		t.Fatalf("failed to create active session: %v", err)
	}

	// 1. Create student1 and enroll in courseID directly
	status, s1Res := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660510001",
		"fullName":    "กิตติพงศ์ ใจดี",
		"courseId":    courseID,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 Created for student1, got %d: %v", status, s1Res)
	}
	s1Data := s1Res["data"].(map[string]interface{})
	student1ID := uint64(s1Data["id"].(float64))
	if uint64(s1Data["addedToActiveSessionId"].(float64)) != activeSession.ID {
		t.Fatalf("expected student1 to be added to active session %d, got %v", activeSession.ID, s1Data["addedToActiveSessionId"])
	}

	// 2. Duplicate studentCode -> 409 DUPLICATE_RESOURCE
	status, dupRes := doJSONRequest(t, app, http.MethodPost, "/api/students", map[string]interface{}{
		"studentCode": "660510001",
		"fullName":    "ซ้ำ รหัสเดิม",
	})
	if status != http.StatusConflict || dupRes["error"].(map[string]interface{})["code"] != "DUPLICATE_RESOURCE" {
		t.Fatalf("expected 409 DUPLICATE_RESOURCE for duplicate studentCode, got %d: %v", status, dupRes)
	}

	// 3. Enroll student2 via POST /api/courses/:courseId/enrollments
	status, e2Res := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseID), map[string]interface{}{
		"studentCode": "660510002",
		"fullName":    "ชนิกานต์ รักเรียน",
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 Created enrolling student2, got %d: %v", status, e2Res)
	}
	student2ID := uint64(e2Res["data"].(map[string]interface{})["studentId"].(float64))

	// 4. Duplicate enrollment in same course -> 409 DUPLICATE_RESOURCE
	status, dupEnrollRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseID), map[string]interface{}{
		"studentId": student2ID,
	})
	if status != http.StatusConflict || dupEnrollRes["error"].(map[string]interface{})["code"] != "DUPLICATE_RESOURCE" {
		t.Fatalf("expected 409 DUPLICATE_RESOURCE for duplicate enrollment, got %d: %v", status, dupEnrollRes)
	}

	// 5. Try deleting student2 while still enrolled -> 409 STUDENT_HAS_HISTORY
	status, delS2Fail := doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/students/%d", student2ID), nil)
	if status != http.StatusConflict || delS2Fail["error"].(map[string]interface{})["code"] != "STUDENT_HAS_HISTORY" {
		t.Fatalf("expected 409 STUDENT_HAS_HISTORY deleting enrolled student, got %d: %v", status, delS2Fail)
	}

	// 6. Withdraw student2 (who has NO confirmed attendance and NO pick_history) -> deletes enrollment row
	status, w2Res := doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d/enrollments/%d", courseID, student2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK withdrawing history-free student2, got %d: %v", status, w2Res)
	}
	w2Data := w2Res["data"].(map[string]interface{})
	if w2Data["action"] != "deleted" || w2Data["historyPreserved"] != false {
		t.Fatalf("expected action=deleted and historyPreserved=false for history-free student2, got %v", w2Data)
	}

	// Now deleting student2 from global system succeeds!
	status, delS2OK := doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/students/%d", student2ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK deleting student2 after removing history-free enrollment, got %d: %v", status, delS2OK)
	}

	// 7. Give student1 confirmed attendance ('present') and a valid pick, then withdraw student1 -> Soft Withdraw ('withdrawn') preserving history!
	if err := db.Model(&models.Attendance{}).
		Where("session_id = ? AND student_id = ?", activeSession.ID, student1ID).
		Update("status", models.AttendanceStatusPresent).Error; err != nil {
		t.Fatalf("failed updating student1 attendance to present: %v", err)
	}
	pick := models.PickHistory{
		CourseID:    courseID,
		SessionID:   activeSession.ID,
		StudentID:   student1ID,
		Mode:        models.PickModeWeighted,
		RoundNumber: 1,
		Status:      models.PickStatusValid,
	}
	if err := db.Create(&pick).Error; err != nil {
		t.Fatalf("failed creating pick history for student1: %v", err)
	}

	status, w1Res := doJSONRequest(t, app, http.MethodDelete, fmt.Sprintf("/api/courses/%d/enrollments/%d", courseID, student1ID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK withdrawing student1 with history, got %d: %v", status, w1Res)
	}
	w1Data := w1Res["data"].(map[string]interface{})
	if w1Data["action"] != "withdrawn" || w1Data["status"] != "withdrawn" || w1Data["historyPreserved"] != true {
		t.Fatalf("expected soft withdraw with historyPreserved=true for student1, got %v", w1Data)
	}

	// Verify student1's attendance and pick_history still exist in DB
	var remainingPicks int64
	db.Model(&models.PickHistory{}).Where("course_id = ? AND student_id = ?", courseID, student1ID).Count(&remainingPicks)
	if remainingPicks != 1 {
		t.Fatalf("expected student1 pick_history to be preserved, found %d", remainingPicks)
	}

	// Re-enrolling withdrawn student1 reactivates enrollment
	status, reEnrollRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/enrollments", courseID), map[string]interface{}{
		"studentId": student1ID,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 Created reactivating withdrawn student1, got %d: %v", status, reEnrollRes)
	}
}

func TestBulkImportCSVXLSXAndPaste(t *testing.T) {
	app, _ := setupPhase1TestApp(t)

	// Create a course
	_, courseRes := doJSONRequest(t, app, http.MethodPost, "/api/courses", map[string]interface{}{
		"courseCode": "001102",
		"courseName": "Fundamental English 2",
		"section":    "001",
		"semester":   "2/2569",
	})
	courseID := uint64(courseRes["data"].(map[string]interface{})["id"].(float64))

	// 1. Test CSV Import (with header, 2 valid new rows, 1 duplicate row in file, 1 invalid row missing code)
	csvContent := "student_code,full_name\n660510011,สมชาย ใจสว่าง\n660510012,สมหญิง รักดี\n660510011,สมชาย ซ้ำในไฟล์\n,ไม่มีรหัส นักศึกษา\n"
	status, csvRes := doMultipartFileRequest(
		t,
		app,
		fmt.Sprintf("/api/courses/%d/students/import-file", courseID),
		"file",
		"students.csv",
		[]byte(csvContent),
	)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for CSV import, got %d: %v", status, csvRes)
	}
	csvSummary := csvRes["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(csvSummary["totalRowsRead"].(float64)) != 4 ||
		int(csvSummary["newlyCreatedAndEnrolled"].(float64)) != 2 ||
		int(csvSummary["alreadyEnrolledSkipped"].(float64)) != 1 ||
		int(csvSummary["invalidRowsSkipped"].(float64)) != 1 {
		t.Fatalf("unexpected CSV import summary: %v", csvSummary)
	}

	// 2. Test Excel (.xlsx) Import (1 existing enrolled student, 2 new students)
	xlsxBytes, err := importer.BuildSampleXLSX([][]string{
		{"รหัสนักศึกษา", "ชื่อ-นามสกุล"},
		{"660510011", "สมชาย ใจสว่าง"},
		{"660510013", "อนันต์ ยอดเยี่ยม"},
		{"660510014", "กมลวรรณ แจ่มใส"},
	})
	if err != nil {
		t.Fatalf("failed building sample xlsx: %v", err)
	}

	status, xlsxRes := doMultipartFileRequest(
		t,
		app,
		fmt.Sprintf("/api/courses/%d/students/import-file", courseID),
		"file",
		"roster.xlsx",
		xlsxBytes,
	)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for XLSX import, got %d: %v", status, xlsxRes)
	}
	xlsxSummary := xlsxRes["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(xlsxSummary["newlyCreatedAndEnrolled"].(float64)) != 2 ||
		int(xlsxSummary["alreadyEnrolledSkipped"].(float64)) != 1 {
		t.Fatalf("unexpected XLSX import summary: %v", xlsxSummary)
	}

	// 3. Test invalid file extension (.txt) -> 400 VALIDATION_ERROR
	status, badFileRes := doMultipartFileRequest(
		t,
		app,
		fmt.Sprintf("/api/courses/%d/students/import-file", courseID),
		"file",
		"notes.txt",
		[]byte("660510099,Test"),
	)
	if status != http.StatusBadRequest || badFileRes["error"].(map[string]interface{})["code"] != "VALIDATION_ERROR" {
		t.Fatalf("expected 400 VALIDATION_ERROR for .txt upload, got %d: %v", status, badFileRes)
	}

	// 4. Test Bulk Paste (POST /api/courses/:courseId/students/bulk)
	status, bulkRes := doJSONRequest(t, app, http.MethodPost, fmt.Sprintf("/api/courses/%d/students/bulk", courseID), map[string]interface{}{
		"students": []map[string]string{
			{"studentCode": "660510015", "fullName": "ธีรภัทร มั่นคง"},
		},
		"rawText": "660510016\tพิมพ์ชนก สดใส\n660510011\tสมชาย ใจสว่าง\nรหัสอย่างเดียวไม่มีชื่อ",
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK for bulk paste, got %d: %v", status, bulkRes)
	}
	bulkSummary := bulkRes["data"].(map[string]interface{})["summary"].(map[string]interface{})
	if int(bulkSummary["newlyCreatedAndEnrolled"].(float64)) != 2 ||
		int(bulkSummary["alreadyEnrolledSkipped"].(float64)) != 1 ||
		int(bulkSummary["invalidRowsSkipped"].(float64)) != 1 {
		t.Fatalf("unexpected bulk paste summary: %v", bulkSummary)
	}

	// 5. Verify total enrolled active students in course is now 6 (11, 12, 13, 14, 15, 16)
	status, listRes := doJSONRequest(t, app, http.MethodGet, fmt.Sprintf("/api/courses/%d/students", courseID), nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 OK listing course students, got %d", status)
	}
	enrolledList := listRes["data"].([]interface{})
	if len(enrolledList) != 6 {
		t.Fatalf("expected 6 active enrolled students in course, got %d", len(enrolledList))
	}
}
