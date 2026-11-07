# คู่มือการเชื่อมต่อ API สำหรับทีม Frontend (Frontend Integration Guide)

เอกสารนี้จัดทำขึ้นสำหรับสมาชิกทีม Frontend ทั้ง 2 คน เพื่อให้สามารถพัฒนาหน้าจอ **Next.js + TypeScript + Tailwind CSS + shadcn/ui** โดยใช้ TypeScript Types และ API Client ที่เตรียมไว้ให้แล้วในโปรเจกต์ (`frontend/src/types/fairpick.ts` และ `frontend/src/lib/api.ts`) ซึ่งตรงกับ REST API ทั้ง **33 Endpoints** ของ Backend ทุกประการ

---

## 1. การแบ่งหน้าที่ในทีมและลำดับหน้าจอ (Team Roles & Recommended Routes)

| ผู้รับผิดชอบ | ขอบเขตหน้าที่ | หน้าจอ / ส่วนงานหลัก | หมวด API ที่เกี่ยวข้อง |
|---|---|---|---|
| **คนที่ 1** | Backend, PostgreSQL, GORM, REST API, API Contract และ **API Integration ทั้งหมด** | - พัฒนาและตรวจสอบ Backend (`backend/`) ทั้ง 33 Endpoints<br>- เตรียมไฟล์ [fairpick.ts](file:///c:/FairPick/frontend/src/types/fairpick.ts) และ [api.ts](file:///c:/FairPick/frontend/src/lib/api.ts)<br>- เชื่อมต่อ API จริงเข้ากับหน้าจอเมื่อ UI พร้อม | ทุกหมวด (หมวดที่ 0 - 10) |
| **คนที่ 2** | **UX/UI และ Frontend** ส่วนรายวิชา นักศึกษา และเช็กชื่อ | - `/courses` (หน้ารวมรายวิชา + สร้าง/แก้ไข/ลบวิชา)<br>- `/courses/[courseId]/students` (จัดการนักศึกษาในวิชา + Import Excel/CSV + Paste รายชื่อ)<br>- `/courses/[courseId]/attendance` (เริ่มคาบเรียน / ใบเช็กชื่อประจำคาบ / แจ้งเตือน `unconfirmed` ก่อนจบคาบเรียน) | หมวดที่ 1, 2, 3, 4, 5, 6 |
| **คนที่ 3** | **UX/UI และ Frontend** ส่วนระบบสุ่ม ประวัติ และสถิติ | - `/courses/[courseId]/pick` (หน้าจอสุ่มรายชื่อประจำคาบ + สลับโหมด + ปุ่มเริ่มรอบใหม่ + ปุ่มยกเลิกผลสุ่ม)<br>- `/courses/[courseId]/history` (ประวัติการสุ่มย้อนหลัง)<br>- `/courses/[courseId]/stats` (แดชบอร์ดสถิติภาพรวมและสถิตินักศึกษารายบุคคล) | หมวดที่ 5 (ดูคาบ/สลับโหมด), 7, 8, 9, 10 |

---

## 2. วิธีรันระบบ Backend และ Frontend (Local Development Setup)

### 2.1 การตั้งค่าและรัน Backend (`http://localhost:8080`)
1. คัดลอกไฟล์ `backend/.env.example` เป็น `backend/.env` แล้วระบุรหัสผ่าน PostgreSQL 18 ของเครื่องตนเอง:
   ```env
   PORT=8080
   CORS_ORIGINS=http://localhost:3000
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=YOUR_POSTGRES_PASSWORD
   DB_NAME=fairpick
   DB_SSLMODE=disable
   ```
2. รัน Migration เพื่อสร้างตารางและ Constraints ทั้งหมดบนฐานข้อมูล `fairpick`:
   ```powershell
   cd c:\FairPick\backend
   go run ./cmd/migrate
   ```
3. รันเซิร์ฟเวอร์ Backend:
   ```powershell
   cd c:\FairPick\backend
   go run .
   ```
   - ทดสอบสถานะฐานข้อมูลได้ที่: `http://localhost:8080/api/health/db`
   - ระบบตั้งค่า CORS รองรับ `http://localhost:3000` พร้อม HTTP Methods (`GET, POST, PUT, PATCH, DELETE, OPTIONS`) เรียบร้อยแล้ว

### 2.2 การตั้งค่าและรัน Frontend (`http://localhost:3000`)
1. คัดลอกไฟล์ `frontend/.env.example` ไปเป็น `frontend/.env.local`:
   ```env
   NEXT_PUBLIC_API_URL=http://localhost:8080
   ```
   *(หากไม่กำหนดค่า ระบบจะใช้ค่าเริ่มต้น `http://localhost:8080` โดยอัตโนมัติ ห้ามใส่รหัสผ่านฐานข้อมูลหรือความลับใด ๆ ไว้ใน `frontend/.env.local`)*
2. รันเซิร์ฟเวอร์ Next.js Development:
   ```powershell
   cd c:\FairPick\frontend
   npm run dev
   ```

---

## 3. โครงสร้างไฟล์ TypeScript ที่เตรียมไว้ให้แล้ว

ในโฟลเดอร์ `frontend/` มีไฟล์พร้อมใช้งานทันทีโดยไม่ต้องสร้างใหม่:

1. **[frontend/src/types/fairpick.ts](file:///c:/FairPick/frontend/src/types/fairpick.ts)**
   - รวบรวม TypeScript Interfaces และ Types ทั้งหมดของระบบ (Envelopes, Pagination, Error Codes, Courses, Students, Enrollments, Bulk Import, Class Sessions, Attendance, Pick Pool, Execute Pick, Pick History และ Statistics)
2. **[frontend/src/lib/api.ts](file:///c:/FairPick/frontend/src/lib/api.ts)**
   - รวบรวมฟังก์ชันเรียก REST API ครบทั้ง **33 Endpoints** พร้อมคลาส `ApiClientError` และออบเจกต์รวม `fairpickApi`
   - รองรับทั้งการส่ง JSON ปกติ, Query Parameters อัตโนมัติ และการอัปโหลดไฟล์ Excel/CSV ด้วย `FormData` (`multipart/form-data`)

---

## 4. ตารางสรุปฟังก์ชันใน `frontend/src/lib/api.ts` ทั้ง 33 Endpoints

สามารถ Import แยกฟังก์ชัน หรือ Import ผ่าน `fairpickApi` จาก `@/lib/api` ได้ทันที:

| ลำดับ | Method & Path | ฟังก์ชันใน `@/lib/api` | Namespace (`fairpickApi.*`) |
|---|---|---|---|
| 1 | `GET /api/health/db` | `checkDatabaseHealth()` | `fairpickApi.health.checkDB()` |
| 2 | `GET /api/courses` | `listCourses(params?)` | `fairpickApi.courses.list(params?)` |
| 3 | `POST /api/courses` | `createCourse(payload)` | `fairpickApi.courses.create(payload)` |
| 4 | `GET /api/courses/:courseId` | `getCourse(courseId)` | `fairpickApi.courses.get(courseId)` |
| 5 | `PUT /api/courses/:courseId` | `updateCourse(courseId, payload)` | `fairpickApi.courses.update(courseId, payload)` |
| 6 | `DELETE /api/courses/:courseId` | `deleteCourse(courseId)` | `fairpickApi.courses.delete(courseId)` |
| 7 | `GET /api/students` | `listStudents(params?)` | `fairpickApi.students.list(params?)` |
| 8 | `POST /api/students` | `createStudent(payload)` | `fairpickApi.students.create(payload)` |
| 9 | `GET /api/students/:studentId` | `getStudent(studentId)` | `fairpickApi.students.get(studentId)` |
| 10 | `PUT /api/students/:studentId` | `updateStudent(studentId, payload)` | `fairpickApi.students.update(studentId, payload)` |
| 11 | `DELETE /api/students/:studentId` | `deleteStudent(studentId)` | `fairpickApi.students.delete(studentId)` |
| 12 | `POST /api/courses/:courseId/students/import-file` | `importCourseStudentsFile(courseId, file)` | `fairpickApi.enrollments.importFile(courseId, file)` |
| 13 | `POST /api/courses/:courseId/students/bulk` | `bulkEnrollCourseStudents(courseId, payload)` | `fairpickApi.enrollments.bulkEnroll(courseId, payload)` |
| 14 | `GET /api/courses/:courseId/students` | `listCourseStudents(courseId, params?)` | `fairpickApi.enrollments.listByCourse(courseId, params?)` |
| 15 | `POST /api/courses/:courseId/enrollments` | `enrollStudentInCourse(courseId, payload)` | `fairpickApi.enrollments.enroll(courseId, payload)` |
| 16 | `DELETE /api/courses/:courseId/enrollments/:studentId` | `withdrawStudentFromCourse(courseId, studentId)` | `fairpickApi.enrollments.withdraw(courseId, studentId)` |
| 17 | `GET /api/courses/:courseId/sessions` | `listCourseSessions(courseId, params?)` | `fairpickApi.sessions.listByCourse(courseId, params?)` |
| 18 | `POST /api/courses/:courseId/sessions` | `startClassSession(courseId, payload?)` | `fairpickApi.sessions.start(courseId, payload?)` |
| 19 | `GET /api/sessions/:sessionId` | `getClassSession(sessionId)` | `fairpickApi.sessions.get(sessionId)` |
| 20 | `PATCH /api/sessions/:sessionId/end` | `endClassSession(sessionId, payload?)` | `fairpickApi.sessions.end(sessionId, payload?)` |
| 21 | `PATCH /api/sessions/:sessionId/mode` | `switchSessionPickMode(sessionId, payload)` | `fairpickApi.sessions.switchMode(sessionId, payload)` |
| 22 | `GET /api/sessions/:sessionId/attendance` | `getSessionAttendance(sessionId, params?)` | `fairpickApi.attendance.getSheet(sessionId, params?)` |
| 23 | `PATCH /api/sessions/:sessionId/attendance/:studentId` | `updateStudentAttendance(sessionId, studentId, payload)` | `fairpickApi.attendance.updateStudent(sessionId, studentId, payload)` |
| 24 | `PATCH /api/sessions/:sessionId/attendance` | `bulkUpdateSessionAttendance(sessionId, payload)` | `fairpickApi.attendance.bulkUpdate(sessionId, payload)` |
| 25 | `GET /api/sessions/:sessionId/picks/pool` | `getSessionPickPool(sessionId)` | `fairpickApi.picks.getPool(sessionId)` |
| 26 | `POST /api/sessions/:sessionId/picks` | `executeRandomPick(sessionId, payload?)` | `fairpickApi.picks.execute(sessionId, payload?)` |
| 27 | `POST /api/sessions/:sessionId/picks/new-round` | `startNewPickRound(sessionId)` | `fairpickApi.picks.startNewRound(sessionId)` |
| 28 | `PATCH /api/picks/:pickId/cancel` | `cancelPick(pickId, payload?)` | `fairpickApi.picks.cancel(pickId, payload?)` |
| 29 | `GET /api/sessions/:sessionId/picks` | `listSessionPicks(sessionId, params?)` | `fairpickApi.picks.listBySession(sessionId, params?)` |
| 30 | `GET /api/courses/:courseId/picks` | `listCoursePicks(courseId, params?)` | `fairpickApi.picks.listByCourse(courseId, params?)` |
| 31 | `GET /api/courses/:courseId/stats/overview` | `getCourseStatsOverview(courseId)` | `fairpickApi.stats.getOverview(courseId)` |
| 32 | `GET /api/courses/:courseId/stats/students` | `listCourseStudentStats(courseId, params?)` | `fairpickApi.stats.listStudents(courseId, params?)` |
| 33 | `GET /api/courses/:courseId/stats/students/:studentId` | `getStudentCourseStats(courseId, studentId)` | `fairpickApi.stats.getStudentDetail(courseId, studentId)` |

---

## 5. วิธีจัดการ Loading / Error / Empty State และ Validation Errors

เมื่อเกิด Error จาก Backend ฟังก์ชันใน `@/lib/api` จะโยน `ApiClientError` ซึ่งมี Helper Methods พร้อมใช้งาน:

```typescript
import { createCourse, isApiClientError } from "@/lib/api";

async function handleCreateCourse(formValues: {
  courseCode: string;
  courseName: string;
  section: string;
  semester: string;
}) {
  setIsSubmitting(true);
  setFieldErrors({});
  setGeneralError(null);

  try {
    const res = await createCourse(formValues);
    // res.data คือ Course ที่สร้างเสร็จ, res.message คือข้อความภาษาไทยจาก Backend
    onSuccess(res.data);
  } catch (err) {
    if (isApiClientError(err)) {
      // 1. กรณี Validation Error (400 VALIDATION_ERROR) — ดึง Error แยกรายฟิลด์มาแสดงใต้ Input ได้เลย
      if (err.isValidationError() && err.details) {
        setFieldErrors(err.details); // เช่น { courseCode: "รหัสวิชาต้องเป็นตัวเลข 6 หลักเท่านั้น (เช่น 001101)" }
        return;
      }

      // 2. กรณีข้อมูลซ้ำหรือมีประวัติอยู่แล้ว (409 Conflict เช่น DUPLICATE_RESOURCE, COURSE_HAS_HISTORY)
      if (err.isConflict()) {
        setGeneralError(err.message);
        return;
      }

      // 3. กรณีอื่น ๆ แสดงข้อความภาษาไทยจาก err.message ได้โดยตรง
      setGeneralError(err.message);
    } else {
      setGeneralError("ไม่สามารถเชื่อมต่อกับเซิร์ฟเวอร์ได้ กรุณาตรวจสอบว่ารัน Backend อยู่");
    }
  } finally {
    setIsSubmitting(false);
  }
}
```

### แนวทางจัดการสถานะหน้าจอ (UI States)
1. **Loading State:** ขณะที่ `isLoading === true` ให้แสดง Skeleton หรือ Spinner และปิดปุ่มกดซ้ำ (`disabled={isSubmitting}`) เพื่อป้องกันการยิง API ซ้ำซ้อน
2. **Empty State:**
   - หาก `courses.length === 0` แสดงข้อความแนะนำให้กดปุ่ม *"สร้างรายวิชาแรก"*
   - หากในคาบเรียนยังไม่มีนักศึกษาที่ได้สถานะ `present` หรือ `late` (`poolData.totalEligible === 0`) ให้แสดงข้อความแนะนำอาจารย์ไปเช็กชื่อนักศึกษาก่อนเริ่มสุ่ม
3. **Error State:** แสดงข้อความภาษาไทยจาก `err.message` ใน Alert / Toast และหากเป็น `err.details` ให้แสดงข้อความสีแดงใต้ช่องกรอกข้อมูลที่ตรงกับชื่อฟิลด์ (`courseCode`, `courseName`, `studentCode`, `fullName`, `status`, `mode`)

---

## 6. ตัวอย่างการใช้งานจริงตาม Workflow ของ FairPick

### 6.1 สำหรับคนที่ 2: นำเข้าไฟล์ Excel/CSV และจัดการใบเช็กชื่อประจำคาบ

```typescript
import {
  importCourseStudentsFile,
  startClassSession,
  getSessionAttendance,
  updateStudentAttendance,
  bulkUpdateSessionAttendance,
  endClassSession,
} from "@/lib/api";

// 1. อัปโหลดไฟล์รายชื่อนักศึกษา (.xlsx หรือ .csv)
async function onUploadRosterFile(courseId: number, file: File) {
  const res = await importCourseStudentsFile(courseId, file);
  console.log("เพิ่มใหม่:", res.data.summary.newlyCreatedAndEnrolled);
  console.log("แถวที่ข้ามเพราะข้อมูลไม่ครบ:", res.data.invalidRows);
}

// 2. เริ่มคาบเรียนใหม่ (ระบบสร้างใบเช็กชื่อสถานะ unconfirmed ให้นักศึกษาที่ active อัตโนมัติ)
async function onStartSession(courseId: number) {
  const res = await startClassSession(courseId, {
    topic: "Unit 1: Introduction",
    initialMode: "weighted",
  });
  return res.data; // ClassSession
}

// 3. เช็กชื่อทั้งหมดเป็นมาเรียน (Mark All Present) หรือเช็กชื่อรายบุคคล
async function onMarkAllPresent(sessionId: number) {
  const res = await bulkUpdateSessionAttendance(sessionId, {
    markAllStatus: "present",
  });
  return res.data.summary;
}

async function onMarkSingleStudentLate(sessionId: number, studentId: number) {
  const res = await updateStudentAttendance(sessionId, studentId, {
    status: "late",
    note: "มาสาย 10 นาที",
  });
  return res.data;
}

// 4. จบคาบเรียน (ตรวจสอบ summary.unconfirmed ก่อนจบคาบเสมอ)
async function onEndSession(sessionId: number, markUnconfirmedAsAbsent: boolean) {
  const res = await endClassSession(sessionId, { markUnconfirmedAsAbsent });
  // หากเลือก markUnconfirmedAsAbsent = false และยังมีคนเป็น unconfirmed:
  // res.data.hasUnconfirmedWarning === true และมีข้อความใน res.data.warningMessage
  return res.data;
}
```

### 6.2 สำหรับคนที่ 3: สุ่มรายชื่อ (`weighted` & `no_repeat`), เริ่มรอบใหม่, ยกเลิกผล และดูสถิติ

```typescript
import {
  getSessionPickPool,
  executeRandomPick,
  startNewPickRound,
  cancelPick,
  switchSessionPickMode,
  getCourseStatsOverview,
  listCourseStudentStats,
  getStudentCourseStats,
  isApiClientError,
} from "@/lib/api";

// 1. ดึงข้อมูล Pool ผู้มีสิทธิ์ถูกสุ่มและค่าน้ำหนัก W = 1 / (1 + H + 2C)
async function loadPickPool(sessionId: number) {
  const res = await getSessionPickPool(sessionId);
  // res.data.pool มี weightScore, probabilityPercent, recent4SessionsPickCount (H), currentSessionPickCount (C)
  // res.data.isNoRepeatRoundCompleted บอกว่าในโหมด no_repeat สุ่มครบทุกคนในรอบปัจจุบันหรือยัง
  return res.data;
}

// 2. กดสุ่มนักศึกษา 1 คน (ประมวลผลการสุ่มจริงจาก Go Backend เสมอ)
async function handleRandomPick(sessionId: number, mode: "weighted" | "no_repeat") {
  try {
    const res = await executeRandomPick(sessionId, {
      mode,
      autoAdvanceRound: false,
    });
    // res.data.pick.student คือผู้ถูกสุ่ม
    // หาก res.data.poolState.isRoundCompleted === true ให้แสดงปุ่ม "เริ่มรอบใหม่"
    return res.data;
  } catch (err) {
    if (isApiClientError(err)) {
      if (err.isRoundCompleted()) {
        // แสดงปุ่ม "เริ่มรอบใหม่" (POST /api/sessions/:sessionId/picks/new-round)
      } else if (err.isNoEligibleStudents()) {
        // แจ้งเตือนว่ายังไม่มีนักศึกษาที่เช็กชื่อเป็น present หรือ late ในคาบนี้
      }
    }
    throw err;
  }
}

// 3. กดปุ่ม "เริ่มรอบใหม่" ในโหมด no_repeat
async function handleStartNewRound(sessionId: number) {
  const res = await startNewPickRound(sessionId);
  return res.data; // { currentRound: 2, remainingInRound: 39, isRoundCompleted: false, ... }
}

// 4. ยกเลิกผลการสุ่ม (กรณีนักศึกษาไม่อยู่ที่โต๊ะ และสามารถเลือกปรับเช็กชื่อเป็น absent พร้อมกันได้)
async function handleCancelPick(pickId: number, markStudentAbsent = false) {
  const res = await cancelPick(pickId, {
    cancelReason: "นักศึกษาไม่ได้อยู่ในห้องเรียนขณะเรียกชื่อ",
    markStudentAbsent,
  });
  return res.data;
}

// 5. ดึงสถิติภาพรวมรายวิชาและตารางสถิติรายบุคคล
async function loadCourseStatistics(courseId: number) {
  const [overviewRes, studentsRes] = await Promise.all([
    getCourseStatsOverview(courseId),
    listCourseStudentStats(courseId, {
      enrollmentStatus: "active",
      sortBy: "studentCode",
      sortOrder: "asc",
    }),
  ]);
  return {
    overview: overviewRes.data,
    students: studentsRes.data,
  };
}
```

---

## 7. ข้อควรระวังสำคัญสำหรับทีม Frontend (Important Guidelines)

1. **การสุ่มโหมด `no_repeat` เมื่อสุ่มครบทุกคนในรอบปัจจุบัน (สำหรับคนที่ 3):**
   - ระบบตั้งค่าเริ่มต้น `autoAdvanceRound: false`
   - เมื่อ `poolState.isRoundCompleted === true` (หรือ `remainingInNoRepeatRound === 0`) หรือได้รับ Error `err.isRoundCompleted()` (`code === "ROUND_COMPLETED"`) ให้ซ่อน/ปิดปุ่ม "สุ่มรายชื่อ" ชั่วคราว แล้วแสดงปุ่ม **"เริ่มรอบใหม่ (Start New Round)"** เพื่อเรียก `startNewPickRound(sessionId)`
2. **การแสดงคำเตือน `unconfirmed` ก่อนจบคาบเรียน (สำหรับคนที่ 2):**
   - ก่อนเรียก `endClassSession(sessionId, ...)` ให้ตรวจสอบ `summary.unconfirmed`
   - หาก `summary.unconfirmed > 0` ให้แสดง Modal แจ้งเตือนว่า *"ยังมีนักศึกษาที่ไม่ได้ยืนยันสถานะการเข้าเรียนจำนวน X คน"* พร้อมให้เลือกว่าจะ:
     - **คงสถานะ `unconfirmed` ตามจริง** (`markUnconfirmedAsAbsent: false` — ค่าเริ่มต้น ซึ่งระบบจะไม่นับเป็นขาดเรียนในการคิดเปอร์เซ็นต์เข้าเรียน)
     - หรือ **เปลี่ยนคนที่ยังไม่ยืนยันเป็นขาดเรียน (`absent`) ทั้งหมด** (`markUnconfirmedAsAbsent: true`)
3. **การแสดงผลสถิติอัตราการเข้าเรียน (สำหรับคนที่ 3):**
   - อัตราการเข้าเรียน (`attendanceRatePercent`) คำนวณจาก `(presentCount + lateCount) / (presentCount + lateCount + absentCount) * 100` เฉพาะคาบเรียนที่จบแล้ว (`ended`) โดยไม่ถือว่า `unconfirmed` เป็น `absent`
   - แนะนำให้แสดงจำนวน `unconfirmedCount` กำกับไว้ในตารางสถิติด้วย (เช่น *"เข้าเรียน 4/4 คาบที่เช็กชื่อ (100%) • ยังไม่ยืนยัน 1 คาบ"*)
4. **ห้ามแปลง `courseCode` และ `studentCode` เป็น `number`:**
   - ใช้ `string` เสมอเพื่อรักษาเลข `0` นำหน้า เช่น `"001101"`
5. **ห้ามสุ่มรายชื่อในฝั่ง Frontend:**
   - การสุ่มต้องเรียก `executeRandomPick(sessionId, ...)` จาก Backend เสมอ แล้วนำผลลัพธ์ที่ได้ไปแสดง Animation บนหน้าจอ
