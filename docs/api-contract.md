# เอกสารข้อตกลงการเชื่อมต่อ API (FairPick REST API Contract)

เอกสารนี้กำหนดมาตรฐาน REST API ทั้งหมด **33 Endpoints** ของระบบ **FairPick** ที่สอดคล้องกับโครงสร้างฐานข้อมูล PostgreSQL 18 และ GORM Models ใน [docs/database-design.md](file:///c:/FairPick/docs/database-design.md) ทุกประการ

---

## 1. มาตรฐานกลางของ API (API Conventions)

### 1.1 Base URL
- **Local Development:** `http://localhost:8080/api`
- **Content-Type:** `application/json; charset=utf-8` (ยกเว้น Endpoint อัปโหลดไฟล์ที่ใช้ `multipart/form-data`)

### 1.2 รูปแบบการตั้งชื่อฟิลด์ ค่าว่าง และวันเวลา
- **Naming Convention:** ใช้ **`camelCase`** ทั้งหมดสำหรับ JSON Request, JSON Response และ Query Parameters
- **NULL & Empty String Normalization:**
  - ฟิลด์ข้อความที่ไม่บังคับกรอก (`courseCode`, `courseName`, `section`, `semester`, `topic`, `note`, `cancelReason`) หากส่ง `""` หรือช่องว่างล้วนเข้ามา Backend จะตัดช่องว่าง (`TrimSpace`) และแปลงเป็น `null` ให้โดยอัตโนมัติ
  - ใน JSON Response ฟิลด์ที่ไม่มีค่าจะส่งกลับเป็น `null` เสมอ (ไม่มีการส่ง `""` ปะปนกับ `null`)
- **Date/Time Format:** ใช้มาตรฐาน **ISO 8601 / RFC 3339** พร้อม Timezone ของไทย (`Asia/Bangkok`, `+07:00`) เช่น `"2026-11-07T09:00:00+07:00"`

### 1.3 โครงสร้าง Response มาตรฐาน (Standard Response Envelope)

#### กรณีสำเร็จ (Success Response — `200 OK`, `201 Created`)
```json
{
  "success": true,
  "message": "ดำเนินการสำเร็จ",
  "data": { ... },
  "meta": {
    "page": 1,
    "limit": 20,
    "totalItems": 45,
    "totalPages": 3
  }
}
```

#### กรณีเกิดข้อผิดพลาด (Error Response — `400`, `404`, `409`, `422`, `500`, `503`)
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "กรุณาระบุรหัสวิชาหรือชื่อวิชาอย่างน้อย 1 อย่าง",
    "details": {
      "courseCode": "รหัสวิชาต้องเป็นตัวเลข 6 หลักเท่านั้น"
    }
  }
}
```

### 1.4 รหัสข้อผิดพลาดมาตรฐาน (Standard Error Codes)

| HTTP Status | Error Code | ความหมายและกรณีที่เกิด |
|---|---|---|
| `400` | `INVALID_REQUEST` | รูปแบบ JSON ไม่ถูกต้อง หรือพารามิเตอร์ใน URL/Query ผิดประเภท |
| `400` | `VALIDATION_ERROR` | ข้อมูลไม่ผ่านเงื่อนไข (เช่น รหัสวิชาไม่ใช่ตัวเลข 6 หลัก, ไม่ระบุทั้งรหัสและชื่อวิชา) |
| `404` | `RESOURCE_NOT_FOUND` | ไม่พบรายวิชา, นักศึกษา, คาบเรียน หรือประวัติการสุ่มตาม ID ที่ระบุ |
| `409` | `DUPLICATE_RESOURCE` | ข้อมูลซ้ำ (เช่น รหัสวิชา+กลุ่ม+เทอมซ้ำกัน, รหัสนักศึกษาซ้ำ, ลงทะเบียนซ้ำในวิชาเดิม) |
| `409` | `COURSE_HAS_HISTORY` | ไม่สามารถลบรายวิชาได้เนื่องจากมีประวัติคาบเรียน (`class_sessions`) เกิดขึ้นแล้ว |
| `409` | `STUDENT_HAS_HISTORY` | ไม่สามารถลบนักศึกษาได้เนื่องจากมีประวัติการลงทะเบียน/เช็กชื่อ/การสุ่มอยู่ในระบบ |
| `409` | `ACTIVE_SESSION_EXISTS` | รายวิชานี้มีคาบเรียนที่กำลังดำเนินการ (`active`) อยู่แล้ว |
| `409` | `SESSION_ALREADY_ENDED` | พยายามเช็กชื่อ, สลับโหมด หรือสุ่มรายชื่อในคาบเรียนที่จบไปแล้ว (`ended`) |
| `409` | `PICK_ALREADY_CANCELLED` | พยายามยกเลิกผลการสุ่มที่ถูกยกเลิกไปแล้ว |
| `422` | `STUDENT_NOT_ENROLLED` | นักศึกษาไม่ได้ลงทะเบียนอยู่ในรายวิชานี้ (หรือถูกถอนออกไปแล้ว `withdrawn`) |
| `422` | `NO_ELIGIBLE_STUDENTS` | ไม่มีนักศึกษาที่มีสถานะ `present` หรือ `late` ให้สุ่มในคาบนี้ |
| `422` | `ROUND_COMPLETED` | สุ่มครบทุกคนในรอบปัจจุบันของโหมด `no_repeat` แล้ว (กรณีกดสุ่มโดย `autoAdvanceRound: false`) |
| `500` | `INTERNAL_SERVER_ERROR` | ข้อผิดพลาดภายในเซิร์ฟเวอร์ |
| `503` | `DATABASE_UNAVAILABLE` | ไม่สามารถเชื่อมต่อฐานข้อมูล PostgreSQL ได้ |

### 1.5 มาตรฐาน Query Parameters สำหรับค้นหา กรอง และแบ่งหน้า

| Query Param | ชนิดข้อมูล | ค่าเริ่มต้น | คำอธิบาย |
|---|---|---|---|
| `page` | `number` | `1` | หน้าที่ต้องการดึงข้อมูล (เริ่มจาก 1) |
| `limit` | `number` | `20` | จำนวนรายการต่อหน้า (สูงสุด `1000` สำหรับดึงทั้งห้องเรียน) |
| `search` | `string` | `""` | คำค้นหา (รหัสวิชา/ชื่อวิชา หรือ รหัสนักศึกษา/ชื่อ-นามสกุล) |

---

## 2. สรุปรายการ API Endpoints ทั้งหมด (33 Endpoints)

| ลำดับ | หมวดหมู่ | Method | URL Path | คำอธิบาย | ผู้ใช้งานหลัก |
|---|---|---|---|---|---|
| 1 | **0. Health Check** | `GET` | `/api/health/db` | ตรวจสอบการเชื่อมต่อฐานข้อมูล PostgreSQL จริง | ทุกคน |
| 2 | **1. Courses** | `GET` | `/api/courses` | ดึงรายการรายวิชาทั้งหมด (ค้นหา/กรองภาคการศึกษา/แบ่งหน้า) | คนที่ 2, 3 |
| 3 | | `POST` | `/api/courses` | สร้างรายวิชาใหม่ | คนที่ 2 |
| 4 | | `GET` | `/api/courses/:courseId` | ดูรายละเอียดรายวิชาและสถานะคาบเรียนปัจจุบัน | คนที่ 2, 3 |
| 5 | | `PUT` | `/api/courses/:courseId` | แก้ไขข้อมูลรายวิชา | คนที่ 2 |
| 6 | | `DELETE` | `/api/courses/:courseId` | ลบรายวิชา (เฉพาะรายวิชาที่ยังไม่มีประวัติคาบเรียน) | คนที่ 2 |
| 7 | **2. Students** | `GET` | `/api/students` | ดึงรายชื่อนักศึกษาทั้งหมดในระบบกลาง | คนที่ 2 |
| 8 | | `POST` | `/api/students` | สร้างข้อมูลนักศึกษาใหม่ (พร้อมตัวเลือกลงทะเบียนเข้าวิชา) | คนที่ 2 |
| 9 | | `GET` | `/api/students/:studentId` | ดูข้อมูลนักศึกษารายบุคคลและวิชาที่ลงทะเบียน | คนที่ 2 |
| 10 | | `PUT` | `/api/students/:studentId` | แก้ไขรหัสนักศึกษาหรือชื่อ-นามสกุล | คนที่ 2 |
| 11 | | `DELETE` | `/api/students/:studentId` | ลบข้อมูลนักศึกษา (เฉพาะกรณีที่ยังไม่มีประวัติ) | คนที่ 2 |
| 12 | **3. Bulk Import** | `POST` | `/api/courses/:courseId/students/import-file` | นำเข้าไฟล์ Excel (`.xlsx`) หรือ `.csv` เข้าสู่รายวิชา | คนที่ 2 |
| 13 | | `POST` | `/api/courses/:courseId/students/bulk` | วางรายชื่อนักศึกษาหลายคน (Paste Text / JSON Array) เข้าวิชา | คนที่ 2 |
| 14 | **4. Enrollments** | `GET` | `/api/courses/:courseId/students` | ดึงรายชื่อนักศึกษาที่ลงทะเบียนในรายวิชานี้ | คนที่ 2, 3 |
| 15 | | `POST` | `/api/courses/:courseId/enrollments` | เพิ่มนักศึกษาเข้าสู่รายวิชา (หรือดึงคนที่เคยถอนกลับมา `active`) | คนที่ 2 |
| 16 | | `DELETE` | `/api/courses/:courseId/enrollments/:studentId` | ถอนนักศึกษาออกจากรายวิชา (คงประวัติเดิมไว้ด้วย `withdrawn`) | คนที่ 2 |
| 17 | **5. Sessions** | `GET` | `/api/courses/:courseId/sessions` | ดูประวัติคาบเรียนทั้งหมดของรายวิชา | คนที่ 2, 3 |
| 18 | | `POST` | `/api/courses/:courseId/sessions` | เริ่มคาบเรียนใหม่ (สร้างใบเช็กชื่อ `unconfirmed` อัตโนมัติ) | คนที่ 2 |
| 19 | | `GET` | `/api/sessions/:sessionId` | ดูข้อมูลคาบเรียน สรุปเช็กชื่อ และสถานะรอบสุ่มปัจจุบัน | คนที่ 2, 3 |
| 20 | | `PATCH` | `/api/sessions/:sessionId/end` | จบคาบเรียน (คง `unconfirmed` หรือเลือกเปลี่ยนเป็น `absent`) | คนที่ 2 |
| 21 | | `PATCH` | `/api/sessions/:sessionId/mode` | เปลี่ยนโหมดการสุ่มประจำคาบ (`weighted` / `no_repeat`) | คนที่ 3 |
| 22 | **6. Attendance** | `GET` | `/api/sessions/:sessionId/attendance` | ดูรายการเช็กชื่อทั้งหมดในคาบเรียน | คนที่ 2, 3 |
| 23 | | `PATCH` | `/api/sessions/:sessionId/attendance/:studentId` | อัปเดตสถานะการเข้าเรียนรายบุคคล | คนที่ 2 |
| 24 | | `PATCH` | `/api/sessions/:sessionId/attendance` | อัปเดตสถานะการเข้าเรียนหลายคนพร้อมกัน (Bulk Check-in) | คนที่ 2 |
| 25 | **7. Random Pick** | `GET` | `/api/sessions/:sessionId/picks/pool` | ดึงรายชื่อผู้มีสิทธิ์ถูกสุ่มพร้อมค่าน้ำหนัก $W = \frac{1}{1+H+2C}$ และสถานะรอบ | คนที่ 3 |
| 26 | | `POST` | `/api/sessions/:sessionId/picks` | สุ่มนักศึกษา 1 คน (`weighted` หรือ `no_repeat`) | คนที่ 3 |
| 27 | | `POST` | `/api/sessions/:sessionId/picks/new-round` | เริ่มรอบการสุ่มใหม่ในโหมด `no_repeat` | คนที่ 3 |
| 28 | **8. Cancel Pick** | `PATCH` | `/api/picks/:pickId/cancel` | ยกเลิกผลการสุ่ม (ไม่นับเป็นจำนวนครั้งที่ถูกสุ่มจริง) | คนที่ 3 |
| 29 | **9. Pick History** | `GET` | `/api/sessions/:sessionId/picks` | ดูประวัติการสุ่มภายในคาบเรียน | คนที่ 3 |
| 30 | | `GET` | `/api/courses/:courseId/picks` | ดูประวัติการสุ่มย้อนหลังของรายวิชา | คนที่ 3 |
| 31 | **10. Statistics** | `GET` | `/api/courses/:courseId/stats/overview` | ดูสถิติภาพรวมของรายวิชาและสรุปรายคาบ | คนที่ 3 |
| 32 | | `GET` | `/api/courses/:courseId/stats/students` | ดูตารางสถิติการเข้าเรียนและการถูกสุ่มของนักศึกษาทุกคนในวิชา | คนที่ 3 |
| 33 | | `GET` | `/api/courses/:courseId/stats/students/:studentId` | ดูสถิติเชิงลึกและไทม์ไลน์รายบุคคลในรายวิชา | คนที่ 3 |

---

## 3. รายละเอียด API แยกตามหมวดหมู่

---

### หมวดที่ 0: ตรวจสอบระบบ (Health Check)

#### 0.1 ตรวจสอบการเชื่อมต่อฐานข้อมูล PostgreSQL
- **Method & Path:** `GET /api/health/db`
- **Response (`200 OK`):**
```json
{
  "database": "connected",
  "db_name": "fairpick",
  "host": "localhost",
  "port": "5432",
  "status": "healthy"
}
```
- **Error Response (`503 Service Unavailable`):**
```json
{
  "database": "disconnected",
  "message": "Unable to establish connection to PostgreSQL database",
  "status": "unhealthy"
}
```

---

### หมวดที่ 1: จัดการรายวิชา (Courses CRUD)

#### 1.1 ดึงรายการรายวิชาทั้งหมด
- **Method & Path:** `GET /api/courses`
- **Query Parameters:** `search` (optional), `semester` (optional), `page` (default `1`), `limit` (default `20`)
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงข้อมูลรายวิชาสำเร็จ",
  "data": [
    {
      "id": 1,
      "courseCode": "001101",
      "courseName": "Fundamental English 1",
      "section": "001",
      "semester": "1/2569",
      "enrolledCount": 42,
      "totalSessions": 5,
      "activeSessionId": 12,
      "createdAt": "2026-11-01T08:30:00+07:00",
      "updatedAt": "2026-11-07T09:00:00+07:00"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "totalItems": 1,
    "totalPages": 1
  }
}
```

#### 1.2 สร้างรายวิชาใหม่
- **Method & Path:** `POST /api/courses`
- **เงื่อนไข Validation & Duplication Check:**
  - ทุกฟิลด์สามารถเป็น `null` หรือ `""` ได้ (Backend จะ Trim และแปลง `""` เป็น `null`)
  - ต้องระบุ `courseCode` หรือ `courseName` อย่างน้อย 1 อย่าง
  - หากระบุ `courseCode` ต้องเป็นตัวเลข 6 หลักเท่านั้น (`^[0-9]{6}$` เช่น `"001101"`)
  - ห้ามสร้างรายวิชาที่มี `(courseCode, section, semester)` ซ้ำกับวิชาที่มีอยู่แล้ว แม้จะตั้ง `courseName` ต่างกันก็ตาม (ตอบ `409 DUPLICATE_RESOURCE`)
- **Request Body:**
```json
{
  "courseCode": "001101",
  "courseName": "Fundamental English 1",
  "section": "001",
  "semester": "1/2569"
}
```
- **Response (`201 Created`):**
```json
{
  "success": true,
  "message": "สร้างรายวิชาสำเร็จ",
  "data": {
    "id": 1,
    "courseCode": "001101",
    "courseName": "Fundamental English 1",
    "section": "001",
    "semester": "1/2569",
    "enrolledCount": 0,
    "totalSessions": 0,
    "activeSessionId": null,
    "createdAt": "2026-11-07T09:00:00+07:00",
    "updatedAt": "2026-11-07T09:00:00+07:00"
  }
}
```
- **Error Response กรณีรหัสวิชา+กลุ่ม+เทอมซ้ำ (`409 Conflict`):**
```json
{
  "success": false,
  "error": {
    "code": "DUPLICATE_RESOURCE",
    "message": "มีรายวิชารหัส 001101 กลุ่ม 001 ในภาคการศึกษา 1/2569 อยู่ในระบบแล้ว"
  }
}
```

#### 1.3 ดูรายละเอียดรายวิชา
- **Method & Path:** `GET /api/courses/:courseId`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงรายละเอียดรายวิชาสำเร็จ",
  "data": {
    "id": 1,
    "courseCode": "001101",
    "courseName": "Fundamental English 1",
    "section": "001",
    "semester": "1/2569",
    "enrolledCount": 42,
    "totalSessions": 5,
    "activeSession": {
      "id": 12,
      "sessionNumber": 5,
      "topic": "Unit 5: Speaking Practice",
      "currentMode": "weighted",
      "currentRound": 1,
      "status": "active",
      "startedAt": "2026-11-07T09:00:00+07:00"
    },
    "createdAt": "2026-11-01T08:30:00+07:00",
    "updatedAt": "2026-11-07T09:00:00+07:00"
  }
}
```

#### 1.4 แก้ไขข้อมูลรายวิชา
- **Method & Path:** `PUT /api/courses/:courseId`
- **Request Body:**
```json
{
  "courseCode": "001101",
  "courseName": "Fundamental English 1",
  "section": "002",
  "semester": "1/2569"
}
```
- **Response (`200 OK`):** โครงสร้างเดียวกับ `POST /api/courses`

#### 1.5 ลบรายวิชา
- **Method & Path:** `DELETE /api/courses/:courseId`
- **เงื่อนไขการลบรายวิชา:**
  - รายวิชาที่ **ไม่มีประวัติคาบเรียน (`class_sessions`)** สามารถลบได้ (รายการ `enrollments` ในวิชานั้นจะถูกลบออกตามไปด้วย)
  - รายวิชาที่ **มีประวัติคาบเรียนแล้ว** ห้ามลบเด็ดขาด เพื่อป้องกันการลบข้อมูล `attendance` และ `pick_history` โดยไม่ตั้งใจ โดยระบบจะตอบกลับด้วย HTTP `409 Conflict` พร้อมรหัส `COURSE_HAS_HISTORY`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ลบรายวิชาสำเร็จ",
  "data": {
    "deletedCourseId": 1
  }
}
```
- **Error Response กรณีรายวิชามีประวัติคาบเรียนแล้ว (`409 Conflict`):**
```json
{
  "success": false,
  "error": {
    "code": "COURSE_HAS_HISTORY",
    "message": "ไม่สามารถลบรายวิชานี้ได้ เนื่องจากมีประวัติคาบเรียน ข้อมูลการเข้าเรียน หรือประวัติการสุ่มอยู่ในระบบแล้ว"
  }
}
```

---

### หมวดที่ 2: จัดการนักศึกษา (Students CRUD)

#### 2.1 ดึงรายชื่อนักศึกษาในระบบกลาง
- **Method & Path:** `GET /api/students`
- **Query Parameters:** `search`, `page`, `limit`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงรายชื่อนักศึกษาสำเร็จ",
  "data": [
    {
      "id": 101,
      "studentCode": "660510001",
      "fullName": "กิตติพงศ์ ใจดี",
      "activeEnrollmentsCount": 2,
      "createdAt": "2026-11-01T08:35:00+07:00",
      "updatedAt": "2026-11-01T08:35:00+07:00"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "totalItems": 1,
    "totalPages": 1
  }
}
```

#### 2.2 สร้างข้อมูลนักศึกษาใหม่
- **Method & Path:** `POST /api/students`
- **คำอธิบาย:** สร้างนักศึกษาใหม่ในระบบ และหากระบุ `courseId` มาด้วยจะลงทะเบียนเข้าวิชานั้นทันที (หากวิชานั้นมีคาบเรียนที่ `active` อยู่ จะสร้างรายการเช็กชื่อ `unconfirmed` ในคาบนั้นให้อัตโนมัติ)
- **Request Body:**
```json
{
  "studentCode": "660510001",
  "fullName": "กิตติพงศ์ ใจดี",
  "courseId": 1
}
```
- **Response (`201 Created`):**
```json
{
  "success": true,
  "message": "เพิ่มข้อมูลนักศึกษาสำเร็จ",
  "data": {
    "id": 101,
    "studentCode": "660510001",
    "fullName": "กิตติพงศ์ ใจดี",
    "enrolledInCourseId": 1,
    "addedToActiveSessionId": 12,
    "createdAt": "2026-11-07T09:05:00+07:00",
    "updatedAt": "2026-11-07T09:05:00+07:00"
  }
}
```

#### 2.3 ดูข้อมูลนักศึกษารายบุคคล
- **Method & Path:** `GET /api/students/:studentId`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงข้อมูลนักศึกษาสำเร็จ",
  "data": {
    "id": 101,
    "studentCode": "660510001",
    "fullName": "กิตติพงศ์ ใจดี",
    "courses": [
      {
        "courseId": 1,
        "courseCode": "001101",
        "courseName": "Fundamental English 1",
        "section": "001",
        "semester": "1/2569",
        "enrollmentStatus": "active",
        "enrolledAt": "2026-11-01T08:35:00+07:00",
        "withdrawnAt": null
      }
    ],
    "createdAt": "2026-11-01T08:35:00+07:00",
    "updatedAt": "2026-11-01T08:35:00+07:00"
  }
}
```

#### 2.4 แก้ไขข้อมูลนักศึกษา
- **Method & Path:** `PUT /api/students/:studentId`
- **Request Body:**
```json
{
  "studentCode": "660510001",
  "fullName": "นายกิตติพงศ์ ใจดี"
}
```
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "แก้ไขข้อมูลนักศึกษาสำเร็จ",
  "data": {
    "id": 101,
    "studentCode": "660510001",
    "fullName": "นายกิตติพงศ์ ใจดี",
    "createdAt": "2026-11-01T08:35:00+07:00",
    "updatedAt": "2026-11-07T09:10:00+07:00"
  }
}
```

#### 2.5 ลบข้อมูลนักศึกษา
- **Method & Path:** `DELETE /api/students/:studentId`
- **เงื่อนไขการคงข้อมูลย้อนหลัง:** หากนักศึกษาคนนี้มีการลงทะเบียนหรือมีประวัติการเข้าเรียน/การสุ่มอยู่ในระบบ ระบบจะไม่อนุญาตให้ลบออกจากฐานข้อมูลและจะตอบ `409 STUDENT_HAS_HISTORY`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ลบข้อมูลนักศึกษาสำเร็จ",
  "data": {
    "deletedStudentId": 101
  }
}
```
- **Error Response กรณีนักศึกษามีประวัติ (`409 Conflict`):**
```json
{
  "success": false,
  "error": {
    "code": "STUDENT_HAS_HISTORY",
    "message": "ไม่สามารถลบข้อมูลนักศึกษาได้ เนื่องจากมีประวัติการลงทะเบียนหรือประวัติการเรียนในระบบ (กรุณาใช้การถอนรายวิชาแทน)"
  }
}
```

---

### หมวดที่ 3: นำเข้าไฟล์ Excel / CSV และวางรายชื่อหลายคน (Import & Bulk Paste)

#### 3.1 นำเข้าด้วยไฟล์ Excel (`.xlsx`) หรือ CSV (`.csv`)
- **Method & Path:** `POST /api/courses/:courseId/students/import-file`
- **Content-Type:** `multipart/form-data`
- **Form Fields:** `file` (ไฟล์ `.xlsx` หรือ `.csv`)
- **พฤติกรรมเมื่อมีคาบเรียน `active` อยู่:** นักศึกษาทุกคนที่ถูกเพิ่มเข้าวิชาใหม่ (หรือถูกเปลี่ยนจาก `withdrawn` กลับมาเป็น `active`) จะถูกสร้างรายการ `attendance` (`status = 'unconfirmed'`) ในคาบที่กำลังเรียนอยู่โดยอัตโนมัติ
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "นำเข้ารายชื่อนักศึกษาสำเร็จ",
  "data": {
    "courseId": 1,
    "activeSessionIdUpdated": 12,
    "summary": {
      "totalRowsRead": 45,
      "newlyCreatedAndEnrolled": 40,
      "existingStudentEnrolled": 2,
      "reactivatedEnrollment": 1,
      "alreadyEnrolledSkipped": 1,
      "invalidRowsSkipped": 1
    },
    "invalidRows": [
      {
        "rowNumber": 18,
        "rawContent": ",สมชาย ไม่มีรหัส",
        "reason": "ไม่พบรหัสนักศึกษา"
      }
    ]
  }
}
```

#### 3.2 วางรายชื่อหลายคนพร้อมกัน (Bulk Paste / JSON Array)
- **Method & Path:** `POST /api/courses/:courseId/students/bulk`
- **Request Body:**
```json
{
  "students": [
    {
      "studentCode": "660510001",
      "fullName": "กิตติพงศ์ ใจดี"
    },
    {
      "studentCode": "660510002",
      "fullName": "ชนิกานต์ รักเรียน"
    }
  ],
  "rawText": "660510003\tณัฐวุฒิ สุขสันต์\n660510004\tปภาวรินทร์ ทองดี"
}
```
- **Response (`200 OK`):** โครงสร้างเดียวกับ `POST /api/courses/:courseId/students/import-file`

---

### หมวดที่ 4: จัดการการลงทะเบียนรายวิชา (Enrollments)

#### 4.1 ดึงรายชื่อนักศึกษาที่ลงทะเบียนในรายวิชา
- **Method & Path:** `GET /api/courses/:courseId/students`
- **Query Parameters:**
  - `status` (optional): `"active"` (default — แสดงเฉพาะคนที่กำลังเรียนในวิชา) | `"withdrawn"` | `"all"`
  - `search` (optional): ค้นหารหัสนักศึกษาหรือชื่อ
  - `page` (default `1`), `limit` (default `100`)
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงรายชื่อนักศึกษาในรายวิชาสำเร็จ",
  "data": [
    {
      "enrollmentId": 1,
      "courseId": 1,
      "studentId": 101,
      "studentCode": "660510001",
      "fullName": "กิตติพงศ์ ใจดี",
      "status": "active",
      "enrolledAt": "2026-11-01T08:35:00+07:00",
      "withdrawnAt": null
    }
  ],
  "meta": {
    "page": 1,
    "limit": 100,
    "totalItems": 1,
    "totalPages": 1
  }
}
```

#### 4.2 ลงทะเบียนนักศึกษาเข้าสู่รายวิชา
- **Method & Path:** `POST /api/courses/:courseId/enrollments`
- **คำอธิบาย:**
  - รองรับทั้งส่ง `studentId` หรือส่ง `studentCode` + `fullName`
  - หากนักศึกษาเคยอยู่ในวิชานี้และมีสถานะ `withdrawn` ระบบจะเปลี่ยนสถานะกลับมาเป็น `active` (`withdrawnAt = null`) โดยคงประวัติเดิมไว้ทั้งหมด
  - หากวิชานี้มีคาบเรียนที่ `status = 'active'` อยู่ ระบบจะเพิ่มรายการเช็กชื่อ `unconfirmed` เข้าสู่คาบปัจจุบันทันที
- **Request Body:**
```json
{
  "studentCode": "660510005",
  "fullName": "วรเมธ ตั้งใจ"
}
```
- **Response (`201 Created`):**
```json
{
  "success": true,
  "message": "ลงทะเบียนนักศึกษาเข้าสู่รายวิชาสำเร็จ",
  "data": {
    "enrollmentId": 5,
    "courseId": 1,
    "studentId": 105,
    "studentCode": "660510005",
    "fullName": "วรเมธ ตั้งใจ",
    "status": "active",
    "addedToActiveSessionId": 12,
    "enrolledAt": "2026-11-07T09:15:00+07:00",
    "withdrawnAt": null
  }
}
```

#### 4.3 ถอนนักศึกษาออกจากรายวิชา
- **Method & Path:** `DELETE /api/courses/:courseId/enrollments/:studentId`
- **พฤติกรรมการคงประวัติย้อนหลัง:**
  - หากนักศึกษามีประวัติใน `attendance` (ที่เช็กชื่อแล้ว) หรือมีประวัติใน `pick_history` ของวิชานี้ ระบบจะทำ **Soft Withdraw** (`status = "withdrawn"`, `withdrawnAt = NOW()`) เพื่อคงข้อมูลย้อนหลังไว้ทั้งหมด และตัดออกจากรายชื่อผู้มีสิทธิ์สุ่มทันที
  - หากยังไม่มีประวัติใด ๆ ในวิชานี้เลย ระบบจะลบแถวการลงทะเบียน (และลบแถว `attendance` ที่ยังเป็น `unconfirmed` ในคาบปัจจุบันถ้ามี)
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ถอนนักศึกษาออกจากรายวิชาสำเร็จ (คงประวัติการเรียนย้อนหลังไว้)",
  "data": {
    "courseId": 1,
    "studentId": 105,
    "action": "withdrawn",
    "status": "withdrawn",
    "historyPreserved": true,
    "withdrawnAt": "2026-11-07T09:40:00+07:00"
  }
}
```

---

### หมวดที่ 5: เริ่มคาบเรียนและจบคาบเรียน (Class Sessions)

#### 5.1 ดูรายการคาบเรียนทั้งหมดของรายวิชา
- **Method & Path:** `GET /api/courses/:courseId/sessions`
- **Query Parameters:** `status` (optional: `active` | `ended`), `page`, `limit`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงรายการคาบเรียนสำเร็จ",
  "data": [
    {
      "id": 12,
      "courseId": 1,
      "sessionNumber": 5,
      "topic": "Unit 5: Speaking Practice",
      "currentMode": "weighted",
      "currentRound": 1,
      "status": "active",
      "startedAt": "2026-11-07T09:00:00+07:00",
      "endedAt": null,
      "attendanceSummary": {
        "totalEnrolled": 42,
        "present": 36,
        "late": 3,
        "absent": 2,
        "unconfirmed": 1,
        "confirmedCount": 41,
        "eligibleForPick": 39
      },
      "pickSummary": {
        "totalValidPicks": 4,
        "totalCancelledPicks": 1
      }
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "totalItems": 5,
    "totalPages": 1
  }
}
```

#### 5.2 เริ่มคาบเรียนใหม่ (Start Session)
- **Method & Path:** `POST /api/courses/:courseId/sessions`
- **พฤติกรรมของระบบ:**
  - สร้างแถวใน `class_sessions` (`status = 'active'`)
  - สร้างแถวใน `attendance` (`status = 'unconfirmed'`) ให้กับนักศึกษาทุกคนที่มี `enrollments.status = 'active'` ในวิชานั้นโดยอัตโนมัติ
- **Request Body:**
```json
{
  "topic": "Unit 5: Speaking Practice",
  "initialMode": "weighted"
}
```
- **Response (`201 Created`):**
```json
{
  "success": true,
  "message": "เริ่มคาบเรียนที่ 5 สำเร็จ และสร้างใบเช็กชื่อเริ่มต้นเรียบร้อยแล้ว",
  "data": {
    "id": 12,
    "courseId": 1,
    "sessionNumber": 5,
    "topic": "Unit 5: Speaking Practice",
    "currentMode": "weighted",
    "currentRound": 1,
    "status": "active",
    "startedAt": "2026-11-07T09:00:00+07:00",
    "endedAt": null,
    "attendanceSummary": {
      "totalEnrolled": 42,
      "present": 0,
      "late": 0,
      "absent": 0,
      "unconfirmed": 42,
      "confirmedCount": 0,
      "eligibleForPick": 0
    }
  }
}
```

#### 5.3 ดูรายละเอียดคาบเรียน
- **Method & Path:** `GET /api/sessions/:sessionId`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงข้อมูลคาบเรียนสำเร็จ",
  "data": {
    "id": 12,
    "courseId": 1,
    "courseCode": "001101",
    "courseName": "Fundamental English 1",
    "section": "001",
    "semester": "1/2569",
    "sessionNumber": 5,
    "topic": "Unit 5: Speaking Practice",
    "currentMode": "weighted",
    "currentRound": 1,
    "status": "active",
    "startedAt": "2026-11-07T09:00:00+07:00",
    "endedAt": null,
    "attendanceSummary": {
      "totalEnrolled": 42,
      "present": 36,
      "late": 3,
      "absent": 2,
      "unconfirmed": 1,
      "confirmedCount": 41,
      "eligibleForPick": 39
    },
    "pickSummary": {
      "totalValidPicks": 4,
      "totalCancelledPicks": 1,
      "remainingInCurrentRound": 35
    }
  }
}
```

#### 5.4 จบคาบเรียน (End Session)
- **Method & Path:** `PATCH /api/sessions/:sessionId/end`
- **พฤติกรรมการจัดการสถานะ `unconfirmed`:**
  - ค่าเริ่มต้น `markUnconfirmedAsAbsent: false` — ระบบจะ **คงสถานะ `unconfirmed` ไว้ตามจริง** และไม่ถือเป็น `absent` ในการคำนวณสถิติ
  - หากยังมีนักศึกษาที่เป็น `unconfirmed` เหลืออยู่ Response จะแนบ `hasUnconfirmedWarning: true`, `unconfirmedRemainingCount` และ `warningMessage` เพื่อให้ Frontend แสดงแจ้งเตือนได้อย่างชัดเจน
  - หากส่ง `"markUnconfirmedAsAbsent": true` ระบบจะเปลี่ยนคนที่ยังเป็น `unconfirmed` ในคาบนี้ให้เป็น `absent` ทั้งหมดก่อนปิดคาบ
- **Request Body (Optional):**
```json
{
  "markUnconfirmedAsAbsent": false
}
```
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "จบคาบเรียนสำเร็จ (มีนักศึกษาที่ยังไม่ได้ยืนยันสถานะ 1 คน คงสถานะเป็น unconfirmed ตามจริง)",
  "data": {
    "id": 12,
    "courseId": 1,
    "sessionNumber": 5,
    "status": "ended",
    "startedAt": "2026-11-07T09:00:00+07:00",
    "endedAt": "2026-11-07T10:30:00+07:00",
    "markUnconfirmedAsAbsentUsed": false,
    "unconfirmedRemainingCount": 1,
    "hasUnconfirmedWarning": true,
    "warningMessage": "ยังมีนักศึกษาที่ไม่ได้ยืนยันสถานะการเข้าเรียน (unconfirmed) จำนวน 1 คน โดยระบบคงสถานะไว้ตามจริงและไม่นำมาคิดเป็นขาดเรียน (absent)",
    "attendanceSummary": {
      "totalEnrolled": 42,
      "present": 36,
      "late": 3,
      "absent": 2,
      "unconfirmed": 1,
      "confirmedCount": 41,
      "eligibleForPick": 39
    }
  }
}
```

#### 5.5 เปลี่ยนโหมดการสุ่มประจำคาบ (Switch Mode)
- **Method & Path:** `PATCH /api/sessions/:sessionId/mode`
- **คำอธิบาย:** สลับโหมดระหว่าง `"weighted"` และ `"no_repeat"` โดยไม่ทำให้ประวัติการสุ่มหรือเลขรอบ `currentRound` สูญหาย
- **Request Body:**
```json
{
  "mode": "no_repeat"
}
```
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "เปลี่ยนโหมดการสุ่มเป็น no_repeat สำเร็จ",
  "data": {
    "sessionId": 12,
    "currentMode": "no_repeat",
    "currentRound": 1,
    "eligibleCount": 39,
    "remainingInRound": 35,
    "isRoundCompleted": false
  }
}
```

---

### หมวดที่ 6: จัดการสถานะการเข้าเรียน (Attendance)

#### 6.1 ดึงใบเช็กชื่อประจำคาบเรียน
- **Method & Path:** `GET /api/sessions/:sessionId/attendance`
- **Query Parameters:** `status` (optional), `search` (optional)
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงข้อมูลการเข้าเรียนสำเร็จ",
  "data": {
    "sessionId": 12,
    "courseId": 1,
    "sessionStatus": "active",
    "hasUnconfirmedWarning": true,
    "summary": {
      "totalEnrolled": 3,
      "present": 1,
      "late": 1,
      "absent": 0,
      "unconfirmed": 1,
      "confirmedCount": 2,
      "eligibleForPick": 2
    },
    "records": [
      {
        "attendanceId": 501,
        "studentId": 101,
        "studentCode": "660510001",
        "fullName": "กิตติพงศ์ ใจดี",
        "status": "present",
        "isEligibleForPick": true,
        "checkedAt": "2026-11-07T09:05:12+07:00",
        "note": null
      },
      {
        "attendanceId": 502,
        "studentId": 102,
        "studentCode": "660510002",
        "fullName": "ชนิกานต์ รักเรียน",
        "status": "late",
        "isEligibleForPick": true,
        "checkedAt": "2026-11-07T09:20:00+07:00",
        "note": "รถติด"
      },
      {
        "attendanceId": 503,
        "studentId": 103,
        "studentCode": "660510003",
        "fullName": "ณัฐวุฒิ สุขสันต์",
        "status": "unconfirmed",
        "isEligibleForPick": false,
        "checkedAt": null,
        "note": null
      }
    ]
  }
}
```

#### 6.2 อัปเดตสถานะการเข้าเรียนรายบุคคล
- **Method & Path:** `PATCH /api/sessions/:sessionId/attendance/:studentId`
- **Request Body:**
```json
{
  "status": "late",
  "note": "มาสาย 15 นาที"
}
```
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "อัปเดตสถานะการเข้าเรียนสำเร็จ",
  "data": {
    "attendanceId": 502,
    "sessionId": 12,
    "courseId": 1,
    "studentId": 102,
    "studentCode": "660510002",
    "fullName": "ชนิกานต์ รักเรียน",
    "status": "late",
    "isEligibleForPick": true,
    "checkedAt": "2026-11-07T09:20:00+07:00",
    "note": "มาสาย 15 นาที"
  }
}
```

#### 6.3 อัปเดตสถานะการเข้าเรียนหลายคนพร้อมกัน (Bulk Update)
- **Method & Path:** `PATCH /api/sessions/:sessionId/attendance`
- **Request Body:**
```json
{
  "markAllStatus": "present"
}
```
*(หรือส่ง `"items": [{ "studentId": 101, "status": "present" }, ...]`) *
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "อัปเดตสถานะการเข้าเรียนแบบกลุ่มสำเร็จ",
  "data": {
    "sessionId": 12,
    "updatedCount": 3,
    "summary": {
      "totalEnrolled": 3,
      "present": 3,
      "late": 0,
      "absent": 0,
      "unconfirmed": 0,
      "confirmedCount": 3,
      "eligibleForPick": 3
    }
  }
}
```

---

### หมวดที่ 7: สุ่มนักศึกษา 2 รูปแบบ (`weighted` และ `no_repeat`)

#### 7.1 ดึงข้อมูล Pool ผู้มีสิทธิ์ถูกสุ่มและค่าน้ำหนักปัจจุบัน
- **Method & Path:** `GET /api/sessions/:sessionId/picks/pool`
- **คำอธิบาย:** คำนวณค่าน้ำหนักจากสูตร $W_i = \frac{1}{1 + H_i + 2C_i}$ และความน่าจะเป็น $P_i$ ของนักศึกษาทุกคนที่มีสิทธิ์ถูกสุ่ม (`present` หรือ `late`) ณ ขณะนั้น พร้อมแจ้งสถานะว่าในโหมด `no_repeat` สุ่มครบทุกคนในรอบปัจจุบันหรือยัง (`isRoundCompleted`)
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงข้อมูล Pool การสุ่มสำเร็จ",
  "data": {
    "sessionId": 12,
    "courseId": 1,
    "sessionNumber": 5,
    "currentMode": "weighted",
    "currentRound": 1,
    "weightFormula": "W = 1 / (1 + H + 2C)",
    "previousSessionsUsed": [11, 10, 9, 8],
    "totalEligible": 2,
    "remainingInNoRepeatRound": 1,
    "isNoRepeatRoundCompleted": false,
    "pool": [
      {
        "studentId": 101,
        "studentCode": "660510001",
        "fullName": "กิตติพงศ์ ใจดี",
        "attendanceStatus": "present",
        "recent4SessionsPickCount": 0,
        "currentSessionPickCount": 0,
        "weightScore": 1.0,
        "probabilityPercent": 75.0,
        "pickedInCurrentNoRepeatRound": false
      },
      {
        "studentId": 102,
        "studentCode": "660510002",
        "fullName": "ชนิกานต์ รักเรียน",
        "attendanceStatus": "late",
        "recent4SessionsPickCount": 0,
        "currentSessionPickCount": 1,
        "weightScore": 0.3333,
        "probabilityPercent": 25.0,
        "pickedInCurrentNoRepeatRound": true
      }
    ]
  }
}
```

#### 7.2 กดสุ่มนักศึกษา 1 คน (Execute Random Pick)
- **Method & Path:** `POST /api/sessions/:sessionId/picks`
- **Request Body:**
```json
{
  "mode": "no_repeat",
  "autoAdvanceRound": false
}
```
  - `mode` (optional, string): `"weighted"` หรือ `"no_repeat"`
  - `autoAdvanceRound` (optional, boolean, **ค่าเริ่มต้นคือ `false`**):
    - เมื่อ `autoAdvanceRound: false` (ค่าเริ่มต้น) และสุ่มครบทุกคนในรอบปัจจุบันของโหมด `no_repeat` แล้ว การกดสุ่มอีกครั้งจะหยุดและได้รับ HTTP `422 ROUND_COMPLETED` เพื่อให้ผู้ใช้กดปุ่ม **"เริ่มรอบใหม่"** (`POST /api/sessions/:sessionId/picks/new-round`)
- **Response (`201 Created`):**
```json
{
  "success": true,
  "message": "สุ่มรายชื่อนักศึกษาสำเร็จ",
  "data": {
    "pick": {
      "id": 901,
      "courseId": 1,
      "sessionId": 12,
      "sessionNumber": 5,
      "mode": "no_repeat",
      "roundNumber": 1,
      "status": "valid",
      "weightScore": null,
      "pickedAt": "2026-11-07T09:25:30+07:00",
      "student": {
        "id": 101,
        "studentCode": "660510001",
        "fullName": "กิตติพงศ์ ใจดี",
        "attendanceStatus": "present"
      }
    },
    "poolState": {
      "currentMode": "no_repeat",
      "currentRound": 1,
      "roundReset": false,
      "isRoundCompleted": true,
      "totalEligible": 2,
      "remainingInRound": 0
    }
  }
}
```
*(สังเกตว่าเมื่อสุ่มคนสุดท้ายของรอบจน `remainingInRound` เป็น `0` ฟิลด์ `isRoundCompleted` จะเป็น `true` ทันที เพื่อให้ Frontend แสดงปุ่ม **"เริ่มรอบใหม่"** ได้เลย)*

- **Error Response กรณีสุ่มครบทุกคนในรอบ `no_repeat` แล้ว (`422 Unprocessable Entity`):**
```json
{
  "success": false,
  "error": {
    "code": "ROUND_COMPLETED",
    "message": "สุ่มครบทุกคนที่มีสิทธิ์ในรอบที่ 1 แล้ว กรุณากดปุ่ม 'เริ่มรอบใหม่' เพื่อสุ่มรอบถัดไป"
  }
}
```

- **Error Response กรณีไม่มีนักศึกษาที่ `present` หรือ `late` (`422 Unprocessable Entity`):**
```json
{
  "success": false,
  "error": {
    "code": "NO_ELIGIBLE_STUDENTS",
    "message": "ไม่สามารถสุ่มได้ เนื่องจากยังไม่มีนักศึกษาที่มีสถานะมาเรียน (present) หรือมาสาย (late) ในคาบนี้"
  }
}
```

#### 7.3 เริ่มรอบใหม่ในโหมด No Repeat (Start New Round)
- **Method & Path:** `POST /api/sessions/:sessionId/picks/new-round`
- **คำอธิบาย:** เพิ่ม `class_sessions.current_round` ขึ้น 1 รอบ (เช่น จากรอบที่ 1 เป็นรอบที่ 2) และรีเซ็ตจำนวนคนที่เหลือในรอบใหม่ให้เท่ากับจำนวนผู้มีสิทธิ์ (`present` + `late`) ทั้งหมดในคาบนั้น โดยไม่ลบประวัติของรอบก่อนหน้า
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "เริ่มรอบการสุ่มใหม่ (รอบที่ 2) สำเร็จ",
  "data": {
    "sessionId": 12,
    "previousRound": 1,
    "currentRound": 2,
    "isRoundCompleted": false,
    "totalEligible": 39,
    "remainingInRound": 39
  }
}
```

---

### หมวดที่ 8: ยกเลิกผลการสุ่ม (Cancel Pick)

#### 8.1 ยกเลิกผลการสุ่ม
- **Method & Path:** `PATCH /api/picks/:pickId/cancel`
- **Request Body (Optional):**
```json
{
  "cancelReason": "นักศึกษาขออนุญาตเข้าห้องน้ำชั่วคราว"
}
```
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ยกเลิกผลการสุ่มสำเร็จ (ไม่นับเป็นจำนวนครั้งที่ถูกสุ่มจริง)",
  "data": {
    "id": 901,
    "courseId": 1,
    "sessionId": 12,
    "studentId": 101,
    "studentCode": "660510001",
    "fullName": "กิตติพงศ์ ใจดี",
    "mode": "no_repeat",
    "roundNumber": 1,
    "status": "cancelled",
    "pickedAt": "2026-11-07T09:25:30+07:00",
    "cancelledAt": "2026-11-07T09:26:10+07:00",
    "cancelReason": "นักศึกษาขออนุญาตเข้าห้องน้ำชั่วคราว"
  }
}
```

---

### หมวดที่ 9: ดูประวัติการสุ่ม (Pick History)

#### 9.1 ดูประวัติการสุ่มภายในคาบเรียน
- **Method & Path:** `GET /api/sessions/:sessionId/picks`
- **Query Parameters:** `status` (`valid` | `cancelled`), `mode` (`weighted` | `no_repeat`), `roundNumber`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงประวัติการสุ่มประจำคาบสำเร็จ",
  "data": {
    "sessionId": 12,
    "summary": {
      "totalPicks": 2,
      "validPicks": 1,
      "cancelledPicks": 1
    },
    "items": [
      {
        "id": 902,
        "courseId": 1,
        "sessionId": 12,
        "sessionNumber": 5,
        "studentId": 102,
        "studentCode": "660510002",
        "fullName": "ชนิกานต์ รักเรียน",
        "mode": "weighted",
        "roundNumber": 1,
        "status": "valid",
        "weightScore": 1.0,
        "pickedAt": "2026-11-07T09:27:00+07:00",
        "cancelledAt": null,
        "cancelReason": null
      }
    ]
  }
}
```

#### 9.2 ดูประวัติการสุ่มย้อนหลังทั้งรายวิชา
- **Method & Path:** `GET /api/courses/:courseId/picks`
- **Query Parameters:** `sessionId`, `studentId`, `mode`, `status`, `search`, `page`, `limit`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงประวัติการสุ่มของรายวิชาสำเร็จ",
  "data": [
    {
      "id": 902,
      "courseId": 1,
      "sessionId": 12,
      "sessionNumber": 5,
      "sessionTopic": "Unit 5: Speaking Practice",
      "studentId": 102,
      "studentCode": "660510002",
      "fullName": "ชนิกานต์ รักเรียน",
      "mode": "weighted",
      "roundNumber": 1,
      "status": "valid",
      "weightScore": 1.0,
      "pickedAt": "2026-11-07T09:27:00+07:00",
      "cancelledAt": null,
      "cancelReason": null
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "totalItems": 1,
    "totalPages": 1
  }
}
```

---

### หมวดที่ 10: สถิติการเข้าเรียนและสถิติรายบุคคล (Statistics)

> **กฎการคำนวณอัตราการเข้าเรียน (`attendanceRatePercent`):**
> - ไม่ถือว่า `unconfirmed` เป็น `absent`
> - นับเฉพาะคาบที่ยืนยันสถานะแล้ว: `confirmedSessionsCount = presentCount + lateCount + absentCount`
> - จำนวนครั้งที่มาเรียน: `attendedCount = presentCount + lateCount`
> - `attendanceRatePercent = (attendedCount / confirmedSessionsCount) * 100` (หาก `confirmedSessionsCount == 0` จะได้ `0.00`)

#### 10.1 สถิติภาพรวมของรายวิชา (Course Statistics Overview)
- **Method & Path:** `GET /api/courses/:courseId/stats/overview`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงสถิติภาพรวมของรายวิชาสำเร็จ",
  "data": {
    "course": {
      "id": 1,
      "courseCode": "001101",
      "courseName": "Fundamental English 1",
      "section": "001",
      "semester": "1/2569"
    },
    "overall": {
      "totalActiveStudents": 42,
      "totalWithdrawnStudents": 1,
      "totalSessions": 5,
      "averageAttendanceRatePercent": 95.12,
      "totalValidPicks": 28,
      "totalCancelledPicks": 3,
      "weightedValidPicks": 18,
      "noRepeatValidPicks": 10,
      "studentsNeverPickedCount": 14
    },
    "sessionBreakdown": [
      {
        "sessionId": 12,
        "sessionNumber": 5,
        "topic": "Unit 5: Speaking Practice",
        "startedAt": "2026-11-07T09:00:00+07:00",
        "status": "active",
        "presentCount": 36,
        "lateCount": 3,
        "absentCount": 2,
        "unconfirmedCount": 1,
        "confirmedCount": 41,
        "attendanceRatePercent": 95.12,
        "validPicksCount": 4
      }
    ]
  }
}
```

#### 10.2 ตารางสถิติรายบุคคลของนักศึกษาทุกคนในรายวิชา
- **Method & Path:** `GET /api/courses/:courseId/stats/students`
- **Query Parameters:**
  - `enrollmentStatus` (optional): `"active"` (default) | `"withdrawn"` | `"all"`
  - `search` (optional): ค้นหารหัสนักศึกษาหรือชื่อ-นามสกุล
  - `sortBy` (optional): `"studentCode"` (default) | `"attendanceRate"` | `"validPickCount"`
  - `sortOrder` (optional): `"asc"` (default) | `"desc"`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงสถิตินักศึกษาในรายวิชาสำเร็จ",
  "data": [
    {
      "studentId": 101,
      "studentCode": "660510001",
      "fullName": "กิตติพงศ์ ใจดี",
      "enrollmentStatus": "active",
      "attendance": {
        "totalSessions": 5,
        "confirmedSessionsCount": 4,
        "presentCount": 3,
        "lateCount": 1,
        "absentCount": 0,
        "unconfirmedCount": 1,
        "attendedCount": 4,
        "attendanceRatePercent": 100.0
      },
      "picks": {
        "totalValidPicks": 2,
        "weightedValidPicks": 1,
        "noRepeatValidPicks": 1,
        "cancelledPicks": 1,
        "recent4SessionsPickCount": 2,
        "lastPickedAt": "2026-11-03T09:40:00+07:00"
      }
    }
  ]
}
```

#### 10.3 สถิติเชิงลึกและประวัติรายบุคคลในรายวิชา
- **Method & Path:** `GET /api/courses/:courseId/stats/students/:studentId`
- **Response (`200 OK`):**
```json
{
  "success": true,
  "message": "ดึงสถิติรายบุคคลสำเร็จ",
  "data": {
    "course": {
      "id": 1,
      "courseCode": "001101",
      "courseName": "Fundamental English 1",
      "section": "001",
      "semester": "1/2569"
    },
    "student": {
      "id": 101,
      "studentCode": "660510001",
      "fullName": "กิตติพงศ์ ใจดี",
      "enrollmentStatus": "active",
      "enrolledAt": "2026-11-01T08:35:00+07:00",
      "withdrawnAt": null
    },
    "attendanceSummary": {
      "totalSessions": 5,
      "confirmedSessionsCount": 4,
      "presentCount": 3,
      "lateCount": 1,
      "absentCount": 0,
      "unconfirmedCount": 1,
      "attendedCount": 4,
      "attendanceRatePercent": 100.0
    },
    "pickSummary": {
      "totalValidPicks": 2,
      "weightedValidPicks": 1,
      "noRepeatValidPicks": 1,
      "cancelledPicks": 1
    },
    "attendanceHistory": [
      {
        "sessionId": 12,
        "sessionNumber": 5,
        "topic": "Unit 5: Speaking Practice",
        "startedAt": "2026-11-07T09:00:00+07:00",
        "status": "present",
        "checkedAt": "2026-11-07T09:05:12+07:00",
        "note": null
      }
    ],
    "pickHistory": [
      {
        "pickId": 901,
        "sessionId": 12,
        "sessionNumber": 5,
        "mode": "no_repeat",
        "roundNumber": 1,
        "status": "cancelled",
        "weightScore": null,
        "pickedAt": "2026-11-07T09:25:30+07:00",
        "cancelledAt": "2026-11-07T09:26:10+07:00",
        "cancelReason": "นักศึกษาขออนุญาตเข้าห้องน้ำชั่วคราว"
      }
    ]
  }
}
```
