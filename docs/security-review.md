# รายงานการวิเคราะห์ความปลอดภัยและการปรับปรุงระบบ (Security Review & Enhancements)

เอกสารนี้สรุปผลการตรวจสอบช่องโหว่และความปลอดภัยของโปรเจกต์ **FairPick** (ทั้งส่วน Backend Go/Fiber, PostgreSQL Database และ Frontend Next.js/React) สำหรับการส่งงานอาจารย์ รวมถึงรายการที่ได้ดำเนินการแก้ไขเรียบร้อยแล้ว

---

## 1. สรุปภาพรวมความปลอดภัยเดิมของโปรเจกต์ (Security Strengths)

โครงสร้างพื้นฐานเดิมของโปรเจกต์มีการออกแบบด้านความปลอดภัยที่ดีในหลายส่วนอยู่แล้ว ได้แก่:
- **การป้องกัน SQL Injection (GO-INJECT-001):** ใช้ GORM แบบ Parameterized Queries (`?`) ทุกจุด ไม่มีการนำ String จากผู้ใช้มาต่อเข้ากับคำสั่ง SQL โดยตรง
- **การใช้ Cryptographic Randomness (`crypto/rand`) (GO-CRYPTO-001):** ระบบสุ่มรายชื่อใน `backend/handlers/pick_handler.go` ใช้ `crypto/rand` แทน `math/rand` ทำให้ผลการสุ่มคาดเดาไม่ได้และยุติธรรมจริง
- **การป้องกัน Race Condition ด้วย Database Locking (`FOR UPDATE`):** ในจังหวะเปิด-ปิดคาบเรียน และการกดสุ่มพร้อมกัน มีการใช้ Transaction ร่วมกับ `clause.Locking{Strength: "UPDATE"}` และ Partial Unique Index ใน PostgreSQL
- **การตั้งค่า CORS อย่างจำกัดสิทธิ์ (GO-HTTP-007):** อนุญาตเฉพาะ Origin ที่กำหนดใน `CORS_ORIGINS` (`http://localhost:3000`) ไม่ได้เปิด `*` แบบสาธารณะ
- **การป้องกัน XSS ในฝั่ง Frontend (REACT-XSS-001/002):** ใช้การแสดงผลผ่าน JSX ของ React ที่ทำ Auto-escaping ทุกจุด ไม่มีการใช้ `dangerouslySetInnerHTML`

---

## 2. ช่องโหว่และจุดเสี่ยงที่ตรวจพบ พร้อมการแก้ไข (Vulnerabilities Fixed)

### [ID-01] ระดับความรุนแรง: สูง (High) — การอัปโหลดไฟล์แบบไม่จำกัดขนาด และช่องโหว่ XLSX Zip Bomb (DoS / Memory Exhaustion)
- **ตำแหน่งที่พบ:**
  - `backend/main.go` (บรรทัด 42–44)
  - `backend/handlers/enrollment_handler.go` (`ImportStudentsFile`, บรรทัด 427–445)
  - `backend/pkg/importer/importer.go` (`readSharedStrings`, `readWorksheetRows`, บรรทัด 176–215)
- **สาเหตุและผลกระทบ:**
  1. เซิร์ฟเวอร์ Fiber ไม่ได้กำหนด `ReadTimeout`, `WriteTimeout`, `IdleTimeout` และ `BodyLimit`
  2. ฟังก์ชัน `ImportStudentsFile` อ่านไฟล์ทั้งหมดเข้าสู่หน่วยความจำด้วย `io.ReadAll(f)` โดยไม่จำกัดขนาดไฟล์
  3. ฟังก์ชันอ่านไฟล์ `.xlsx` แตกไฟล์ ZIP และอ่าน XML (`xl/sharedStrings.xml`, `xl/worksheets/sheet*.xml`) เข้าสู่ `xml.NewDecoder(rc)` โดยไม่จำกัดขนาดหลังคลายการบีบอัด (Unbounded Decompression) หากอัปโหลดไฟล์ `.xlsx` ที่มีขนาดเล็กแต่บีบอัด XML ขนาดใหญ่ไว้ (Zip Bomb) จะทำให้ RAM ของเซิร์ฟเวอร์เต็มและล่มได้ทันที
- **การแก้ไข:**
  - กำหนด `ReadTimeout` (15s), `WriteTimeout` (15s), `IdleTimeout` (60s) และ `BodyLimit` (10 MB) ใน `backend/main.go`
  - จำกัดขนาดไฟล์อัปโหลดใน `enrollment_handler.go` ไม่เกิน **5 MB** พร้อมครอบด้วย `io.LimitReader`
  - ครอบการอ่าน XML ภายในไฟล์ `.xlsx` ด้วย `io.LimitReader(rc, 15*1024*1024)` ใน `backend/pkg/importer/importer.go`

---

### [ID-02] ระดับความรุนแรง: ปานกลาง (Medium) — ช่องโหว่ CSV / Formula Injection จากการนำเข้ารายชื่อนักศึกษา
- **ตำแหน่งที่พบ:** `backend/pkg/importer/importer.go` (`validateStudentPair`, บรรทัด 418–435)
- **สาเหตุและผลกระทบ:**
  - การนำเข้าข้อมูลจากไฟล์ `.csv` / `.xlsx` หรือการวางข้อความ (Bulk Paste) ไม่ได้ตรวจสอบอักขระพิเศษที่ขึ้นต้นด้วย `=`, `+`, `@` ซึ่งอาจถูกใช้ทำ Formula Injection เมื่อนำข้อมูลออกไปยังโปรแกรม Spreadsheet
- **การแก้ไข:**
  - เพิ่มการตรวจสอบใน `validateStudentPair` เพื่อปฏิเสธรหัสนักศึกษาหรือชื่อ-นามสกุลที่ขึ้นต้นด้วยอักขระสูตร (`=`, `+`, `@`)

---

### [ID-03] ระดับความรุนแรง: ปานกลาง (Medium) — SQL `ILIKE` Wildcard Injection ในช่องค้นหา (Search)
- **ตำแหน่งที่พบ:**
  - `backend/handlers/course_handler.go`, `student_handler.go`, `enrollment_handler.go`, `attendance_handler.go`, `pick_handler.go`, `stats_handler.go`
- **สาเหตุและผลกระทบ:**
  - การนำค่าพารามิเตอร์ `search` มาต่อด้วย `"%" + search + "%"` โดยไม่ทำการ Escape อักขระ `%`, `_`, และ `\` ทำให้ผู้ใช้สามารถพิมพ์ `%` หรือ `_` เพื่อบังคับเงื่อนไข Wildcard ในฐานข้อมูลได้ และไม่มีการจำกัดความยาวของคำค้นหา
- **การแก้ไข:**
  - สร้างฟังก์ชัน `validator.FormatSearchLike(search)` ใน `backend/pkg/validator/normalize.go` เพื่อจำกัดความยาวคำค้นหา (ไม่เกิน 100 ตัวอักษร) และ Escape อักขระ `\`, `%`, `_` ทุกจุดก่อนส่งเข้าคำสั่ง `ILIKE`

---

### [ID-04] ระดับความรุนแรง: ปานกลาง (Medium) — การตรวจสอบความยาวข้อมูลรายวิชาและปรับปรุงรูปแบบรหัสวิชา (Input Validation)
- **ตำแหน่งที่พบ:**
  - `backend/pkg/validator/normalize.go` (`ValidateCourseInput`)
  - `backend/models/course.go` และ `backend/database/migrate.go`
- **สาเหตุและผลกระทบ:**
  - เดิม `ValidateCourseInput` ตรวจสอบเฉพาะ `courseCode` ว่าเป็นตัวเลข 6 หลัก (`^[0-9]{6}$`) และไม่ได้ตรวจสอบความยาวสูงสุดของ `courseName` (255), `section` (50), `semester` (50) ก่อนส่งเข้าฐานข้อมูล ซึ่งหากส่งข้อความยาวเกินขนาดคอลัมน์จะทำให้เกิด HTTP 500 แทนที่จะเป็น HTTP 400
- **การแก้ไข:**
  - ปรับเกณฑ์รหัสวิชา (`courseCode`) ให้รองรับ **ตัวอักษรภาษาอังกฤษและตัวเลข (`A-Z`, `a-z`, `0-9`) ความยาวไม่เกิน 10 ตัวอักษร** (`^[A-Za-z0-9]{1,10}$`) ทั้งในระดับ Go Validator, GORM Model (`varchar(10)`), PostgreSQL Constraint (`chk_courses_code_format`) และฟอร์มฝั่ง Frontend
  - เพิ่ม `ValidateCourseFields` เพื่อตรวจสอบความยาวของ `courseName`, `section`, และ `semester` อย่างครบถ้วน

---

### [ID-05] ระดับความรุนแรง: ต่ำ-ปานกลาง (Low/Medium) — การเปิดเผยข้อมูลโครงสร้างฐานข้อมูลใน Health Check และขาด Security Headers
- **ตำแหน่งที่พบ:**
  - `backend/handlers/health.go` (`GET /api/health/db`)
  - `backend/routes/routes.go` และ `frontend/next.config.ts`
- **สาเหตุและผลกระทบ:**
  - Endpoint `/api/health/db` เดิมส่งค่า `host`, `port`, และ `db_name` ของฐานข้อมูลกลับไปใน JSON Response โดยไม่จำเป็น
  - ทั้ง Backend และ Frontend ยังไม่ได้ตั้งค่า HTTP Security Headers พื้นฐาน (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`)
- **การแก้ไข:**
  - ตัด `host`, `port`, `db_name` ออกจาก Response ของ `/api/health/db` คงไว้เฉพาะสถานะการเชื่อมต่อ (`status: "healthy", database: "connected"`)
  - เพิ่ม Security Headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`) ทั้งใน `backend/routes/routes.go` และ `frontend/next.config.ts`

---

### [ID-06] ข้อสังเกตเชิงสถาปัตยกรรม (สำหรับตอบคำถามอาจารย์)
- **ระบบยืนยันตัวตน (Authentication / Authorization):** ปัจจุบันโปรเจกต์ถูกออกแบบสำหรับให้อาจารย์ผู้สอนใช้งานภายในเครื่อง (Localhost Classroom Tool) จึงยังไม่มีระบบ Login / Session / Role-Based Access Control (RBAC) ซึ่งเหมาะสมกับขอบเขตงานส่งอาจารย์ที่ไม่ต้องการความซับซ้อนในการล็อกอินก่อนสาธิตระบบ แต่หากในอนาคตนำไป Deploy จริงบนอินเทอร์เน็ต ควรเพิ่มระบบยืนยันตัวตน (เช่น JWT หรือ HttpOnly Session Cookie พร้อม CSRF Protection) และ Rate Limiting
