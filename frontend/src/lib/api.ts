// frontend/src/lib/api.ts
// Centralized REST API Client for FairPick (All 33 Endpoints)

import {
  ApiErrorDetail,
  ApiErrorResponse,
  ApiSuccessResponse,
  BulkEnrollPayload,
  BulkImportResult,
  BulkUpdateAttendancePayload,
  BulkUpdateAttendanceResult,
  CancelPickPayload,
  CancelPickResult,
  ClassSession,
  Course,
  CourseDetail,
  CourseStatsOverview,
  CourseUpsertPayload,
  CreateStudentPayload,
  DeleteCourseResult,
  DeleteStudentResult,
  EndSessionPayload,
  EndSessionResult,
  EnrolledStudent,
  EnrollStudentPayload,
  EnrollStudentResult,
  ExecutePickPayload,
  ExecutePickResult,
  FairPickErrorCode,
  GetSessionAttendanceParams,
  HealthCheckDBResponse,
  ListCoursePicksParams,
  ListCourseSessionsParams,
  ListCoursesParams,
  ListCourseStudentsParams,
  ListCourseStudentStatsParams,
  ListSessionPicksParams,
  ListStudentsParams,
  PickHistoryItem,
  PickPoolData,
  SessionAttendanceData,
  SessionDetail,
  SessionPickHistoryData,
  SingleAttendanceUpdateResult,
  StartNewRoundResult,
  StartSessionPayload,
  Student,
  StudentCourseDetailStats,
  StudentCourseStatItem,
  StudentCreatedResult,
  StudentDetail,
  SwitchModePayload,
  SwitchModeResult,
  UpdateAttendancePayload,
  UpdateStudentPayload,
  WithdrawStudentResult,
} from "@/types/fairpick";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") ||
  "http://localhost:8080";

export class ApiClientError extends Error {
  readonly status: number;
  readonly code: FairPickErrorCode | string;
  readonly details?: Record<string, string>;

  constructor(status: number, errorPayload: ApiErrorDetail) {
    super(errorPayload.message || "เกิดข้อผิดพลาดในการเชื่อมต่อกับเซิร์ฟเวอร์");
    this.name = "ApiClientError";
    this.status = status;
    this.code = errorPayload.code || "UNKNOWN_ERROR";
    this.details = errorPayload.details;
  }

  isValidationError(): boolean {
    return this.status === 400 && this.code === "VALIDATION_ERROR";
  }

  isNotFound(): boolean {
    return this.status === 404 || this.code === "RESOURCE_NOT_FOUND";
  }

  isConflict(): boolean {
    return this.status === 409;
  }

  isRoundCompleted(): boolean {
    return this.status === 422 && this.code === "ROUND_COMPLETED";
  }

  isNoEligibleStudents(): boolean {
    return this.status === 422 && this.code === "NO_ELIGIBLE_STUDENTS";
  }

  getFieldError(fieldName: string): string | undefined {
    return this.details?.[fieldName];
  }
}

export function isApiClientError(err: unknown): err is ApiClientError {
  return err instanceof ApiClientError;
}

function buildQueryString(params?: object): string {
  if (!params) return "";
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params as Record<string, unknown>)) {
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      searchParams.set(key, String(value));
    }
  }
  const qs = searchParams.toString();
  return qs ? `?${qs}` : "";
}

export interface ApiRequestOptions extends Omit<RequestInit, "body"> {
  params?: object;
  body?: unknown;
}

export async function apiFetch<T>(
  path: string,
  options: ApiRequestOptions = {}
): Promise<ApiSuccessResponse<T>> {
  const { params, body, headers: customHeaders, ...restOptions } = options;
  const url = `${API_BASE_URL}${path}${buildQueryString(params)}`;

  const isFormData =
    typeof FormData !== "undefined" && body instanceof FormData;

  const headers: HeadersInit = {
    Accept: "application/json",
    ...(isFormData || body === undefined
      ? {}
      : { "Content-Type": "application/json" }),
    ...(customHeaders || {}),
  };

  const serializedBody: BodyInit | undefined = isFormData
    ? (body as FormData)
    : body !== undefined
      ? JSON.stringify(body)
      : undefined;

  const response = await fetch(url, {
    ...restOptions,
    headers,
    body: serializedBody,
    cache: "no-store",
  });

  let payload: unknown = null;
  try {
    payload = await response.json();
  } catch {
    throw new ApiClientError(response.status, {
      code: "UNKNOWN_ERROR",
      message: `เซิร์ฟเวอร์ตอบกลับด้วยรูปแบบที่ไม่ใช่ JSON (HTTP ${response.status})`,
    });
  }

  const envelope = payload as ApiSuccessResponse<T> | ApiErrorResponse;
  if (!response.ok || envelope.success === false) {
    const errEnvelope = envelope as ApiErrorResponse;
    throw new ApiClientError(
      response.status,
      errEnvelope.error || {
        code: "UNKNOWN_ERROR",
        message: "เกิดข้อผิดพลาดในการเชื่อมต่อกับเซิร์ฟเวอร์",
      }
    );
  }

  return envelope as ApiSuccessResponse<T>;
}

// ============================================================================
// หมวดที่ 0: Health Check (1 Endpoint)
// ============================================================================

/** 1. GET /api/health/db — ตรวจสอบสถานะการเชื่อมต่อฐานข้อมูล PostgreSQL */
export async function checkDatabaseHealth(): Promise<HealthCheckDBResponse> {
  const response = await fetch(`${API_BASE_URL}/api/health/db`, {
    method: "GET",
    headers: { Accept: "application/json" },
    cache: "no-store",
  });
  const data = (await response.json()) as HealthCheckDBResponse;
  if (!response.ok) {
    throw new ApiClientError(response.status, {
      code: "DATABASE_UNAVAILABLE",
      message: data.message || "ไม่สามารถเชื่อมต่อฐานข้อมูล PostgreSQL ได้",
    });
  }
  return data;
}

// ============================================================================
// หมวดที่ 1: จัดการรายวิชา — Courses CRUD (5 Endpoints)
// ============================================================================

/** 2. GET /api/courses — ดึงรายการรายวิชาทั้งหมด */
export async function listCourses(params?: ListCoursesParams) {
  return apiFetch<Course[]>("/api/courses", {
    method: "GET",
    params,
  });
}

/** 3. POST /api/courses — สร้างรายวิชาใหม่ */
export async function createCourse(payload: CourseUpsertPayload) {
  return apiFetch<Course>("/api/courses", {
    method: "POST",
    body: payload,
  });
}

/** 4. GET /api/courses/:courseId — ดูรายละเอียดรายวิชาและสถานะคาบเรียนที่ active */
export async function getCourse(courseId: number) {
  return apiFetch<CourseDetail>(`/api/courses/${courseId}`, {
    method: "GET",
  });
}

/** 5. PUT /api/courses/:courseId — แก้ไขข้อมูลรายวิชา */
export async function updateCourse(
  courseId: number,
  payload: CourseUpsertPayload
) {
  return apiFetch<Course>(`/api/courses/${courseId}`, {
    method: "PUT",
    body: payload,
  });
}

/** 6. DELETE /api/courses/:courseId — ลบรายวิชา (เฉพาะรายวิชาที่ยังไม่มีประวัติคาบเรียน) */
export async function deleteCourse(courseId: number) {
  return apiFetch<DeleteCourseResult>(`/api/courses/${courseId}`, {
    method: "DELETE",
  });
}

// ============================================================================
// หมวดที่ 2: จัดการนักศึกษาในระบบกลาง — Students CRUD (5 Endpoints)
// ============================================================================

/** 7. GET /api/students — ดึงรายชื่อนักศึกษาทั้งหมดในระบบกลาง */
export async function listStudents(params?: ListStudentsParams) {
  return apiFetch<Student[]>("/api/students", {
    method: "GET",
    params,
  });
}

/** 8. POST /api/students — สร้างข้อมูลนักศึกษาใหม่ (พร้อมตัวเลือกลงทะเบียนเข้าวิชาทันที) */
export async function createStudent(payload: CreateStudentPayload) {
  return apiFetch<StudentCreatedResult>("/api/students", {
    method: "POST",
    body: payload,
  });
}

/** 9. GET /api/students/:studentId — ดูข้อมูลนักศึกษารายบุคคลและรายวิชาที่ลงทะเบียน */
export async function getStudent(studentId: number) {
  return apiFetch<StudentDetail>(`/api/students/${studentId}`, {
    method: "GET",
  });
}

/** 10. PUT /api/students/:studentId — แก้ไขรหัสนักศึกษาหรือชื่อ-นามสกุล */
export async function updateStudent(
  studentId: number,
  payload: UpdateStudentPayload
) {
  return apiFetch<Student>(`/api/students/${studentId}`, {
    method: "PUT",
    body: payload,
  });
}

/** 11. DELETE /api/students/:studentId — ลบข้อมูลนักศึกษา (เฉพาะกรณีที่ยังไม่มีประวัติ) */
export async function deleteStudent(studentId: number) {
  return apiFetch<DeleteStudentResult>(`/api/students/${studentId}`, {
    method: "DELETE",
  });
}

// ============================================================================
// หมวดที่ 3: นำเข้ารายชื่อนักศึกษาแบบกลุ่ม — Bulk Import (2 Endpoints)
// ============================================================================

/** 12. POST /api/courses/:courseId/students/import-file — อัปโหลดไฟล์ .xlsx หรือ .csv */
export async function importCourseStudentsFile(courseId: number, file: File) {
  const formData = new FormData();
  formData.append("file", file);

  return apiFetch<BulkImportResult>(
    `/api/courses/${courseId}/students/import-file`,
    {
      method: "POST",
      body: formData,
    }
  );
}

/** 13. POST /api/courses/:courseId/students/bulk — นำเข้ารายชื่อนักศึกษาจากข้อความ (Paste) หรือ JSON Array */
export async function bulkEnrollCourseStudents(
  courseId: number,
  payload: BulkEnrollPayload
) {
  return apiFetch<BulkImportResult>(`/api/courses/${courseId}/students/bulk`, {
    method: "POST",
    body: payload,
  });
}

// ============================================================================
// หมวดที่ 4: จัดการการลงทะเบียนในรายวิชา — Enrollments (3 Endpoints)
// ============================================================================

/** 14. GET /api/courses/:courseId/students — ดึงรายชื่อนักศึกษาที่ลงทะเบียนในรายวิชา */
export async function listCourseStudents(
  courseId: number,
  params?: ListCourseStudentsParams
) {
  return apiFetch<EnrolledStudent[]>(`/api/courses/${courseId}/students`, {
    method: "GET",
    params,
  });
}

/** 15. POST /api/courses/:courseId/enrollments — เพิ่มนักศึกษาเข้าสู่รายวิชา */
export async function enrollStudentInCourse(
  courseId: number,
  payload: EnrollStudentPayload
) {
  return apiFetch<EnrollStudentResult>(`/api/courses/${courseId}/enrollments`, {
    method: "POST",
    body: payload,
  });
}

/** 16. DELETE /api/courses/:courseId/enrollments/:studentId — ถอนนักศึกษาออกจากรายวิชา */
export async function withdrawStudentFromCourse(
  courseId: number,
  studentId: number
) {
  return apiFetch<WithdrawStudentResult>(
    `/api/courses/${courseId}/enrollments/${studentId}`,
    {
      method: "DELETE",
    }
  );
}

// ============================================================================
// หมวดที่ 5: จัดการคาบเรียน — Class Sessions (5 Endpoints)
// ============================================================================

/** 17. GET /api/courses/:courseId/sessions — ดูประวัติคาบเรียนทั้งหมดของรายวิชา */
export async function listCourseSessions(
  courseId: number,
  params?: ListCourseSessionsParams
) {
  return apiFetch<ClassSession[]>(`/api/courses/${courseId}/sessions`, {
    method: "GET",
    params,
  });
}

/** 18. POST /api/courses/:courseId/sessions — เริ่มคาบเรียนใหม่ */
export async function startClassSession(
  courseId: number,
  payload?: StartSessionPayload
) {
  return apiFetch<ClassSession>(`/api/courses/${courseId}/sessions`, {
    method: "POST",
    body: payload || {},
  });
}

/** 19. GET /api/sessions/:sessionId — ดูรายละเอียดคาบเรียน สรุปเช็กชื่อ และสถานะรอบสุ่ม */
export async function getClassSession(sessionId: number) {
  return apiFetch<SessionDetail>(`/api/sessions/${sessionId}`, {
    method: "GET",
  });
}

/** 20. PATCH /api/sessions/:sessionId/end — จบคาบเรียน (ค่าเริ่มต้น markUnconfirmedAsAbsent = false) */
export async function endClassSession(
  sessionId: number,
  payload: EndSessionPayload = { markUnconfirmedAsAbsent: false }
) {
  return apiFetch<EndSessionResult>(`/api/sessions/${sessionId}/end`, {
    method: "PATCH",
    body: payload,
  });
}

/** 21. PATCH /api/sessions/:sessionId/mode — สลับโหมดการสุ่มประจำคาบ (weighted | no_repeat) */
export async function switchSessionPickMode(
  sessionId: number,
  payload: SwitchModePayload
) {
  return apiFetch<SwitchModeResult>(`/api/sessions/${sessionId}/mode`, {
    method: "PATCH",
    body: payload,
  });
}

// ============================================================================
// หมวดที่ 6: จัดการสถานะการเข้าเรียน — Attendance (3 Endpoints)
// ============================================================================

/** 22. GET /api/sessions/:sessionId/attendance — ดึงใบเช็กชื่อประจำคาบเรียน */
export async function getSessionAttendance(
  sessionId: number,
  params?: GetSessionAttendanceParams
) {
  return apiFetch<SessionAttendanceData>(
    `/api/sessions/${sessionId}/attendance`,
    {
      method: "GET",
      params,
    }
  );
}

/** 23. PATCH /api/sessions/:sessionId/attendance/:studentId — อัปเดตสถานะการเข้าเรียนรายบุคคล */
export async function updateStudentAttendance(
  sessionId: number,
  studentId: number,
  payload: UpdateAttendancePayload
) {
  return apiFetch<SingleAttendanceUpdateResult>(
    `/api/sessions/${sessionId}/attendance/${studentId}`,
    {
      method: "PATCH",
      body: payload,
    }
  );
}

/** 24. PATCH /api/sessions/:sessionId/attendance — อัปเดตสถานะการเข้าเรียนแบบกลุ่ม (Bulk Check-in) */
export async function bulkUpdateSessionAttendance(
  sessionId: number,
  payload: BulkUpdateAttendancePayload
) {
  return apiFetch<BulkUpdateAttendanceResult>(
    `/api/sessions/${sessionId}/attendance`,
    {
      method: "PATCH",
      body: payload,
    }
  );
}

// ============================================================================
// หมวดที่ 7: สุ่มรายชื่อนักศึกษา — Random Pick (3 Endpoints)
// ============================================================================

/** 25. GET /api/sessions/:sessionId/picks/pool — ดึงข้อมูล Pool ผู้มีสิทธิ์ถูกสุ่มและค่าน้ำหนักปัจจุบัน */
export async function getSessionPickPool(sessionId: number) {
  return apiFetch<PickPoolData>(`/api/sessions/${sessionId}/picks/pool`, {
    method: "GET",
  });
}

/** 26. POST /api/sessions/:sessionId/picks — กดสุ่มนักศึกษา 1 คนจาก Backend */
export async function executeRandomPick(
  sessionId: number,
  payload: ExecutePickPayload = { autoAdvanceRound: false }
) {
  return apiFetch<ExecutePickResult>(`/api/sessions/${sessionId}/picks`, {
    method: "POST",
    body: {
      autoAdvanceRound: false,
      ...payload,
    },
  });
}

/** 27. POST /api/sessions/:sessionId/picks/new-round — เริ่มรอบการสุ่มใหม่ในโหมด No Repeat */
export async function startNewPickRound(sessionId: number) {
  return apiFetch<StartNewRoundResult>(
    `/api/sessions/${sessionId}/picks/new-round`,
    {
      method: "POST",
    }
  );
}

// ============================================================================
// หมวดที่ 8: ยกเลิกผลการสุ่ม — Cancel Pick (1 Endpoint)
// ============================================================================

/** 28. PATCH /api/picks/:pickId/cancel — ยกเลิกผลการสุ่ม */
export async function cancelPick(pickId: number, payload?: CancelPickPayload) {
  return apiFetch<CancelPickResult>(`/api/picks/${pickId}/cancel`, {
    method: "PATCH",
    body: payload || {},
  });
}

// ============================================================================
// หมวดที่ 9: ประวัติการสุ่ม — Pick History (2 Endpoints)
// ============================================================================

/** 29. GET /api/sessions/:sessionId/picks — ดูประวัติการสุ่มภายในคาบเรียน */
export async function listSessionPicks(
  sessionId: number,
  params?: ListSessionPicksParams
) {
  return apiFetch<SessionPickHistoryData>(`/api/sessions/${sessionId}/picks`, {
    method: "GET",
    params,
  });
}

/** 30. GET /api/courses/:courseId/picks — ดูประวัติการสุ่มย้อนหลังทั้งรายวิชา */
export async function listCoursePicks(
  courseId: number,
  params?: ListCoursePicksParams
) {
  return apiFetch<PickHistoryItem[]>(`/api/courses/${courseId}/picks`, {
    method: "GET",
    params,
  });
}

// ============================================================================
// หมวดที่ 10: สถิติการเข้าเรียนและสถิติรายบุคคล — Statistics (3 Endpoints)
// ============================================================================

/** 31. GET /api/courses/:courseId/stats/overview — ดึงสถิติภาพรวมของรายวิชาและสรุปรายคาบ */
export async function getCourseStatsOverview(courseId: number) {
  return apiFetch<CourseStatsOverview>(
    `/api/courses/${courseId}/stats/overview`,
    {
      method: "GET",
    }
  );
}

/** 32. GET /api/courses/:courseId/stats/students — ดึงตารางสถิติรายบุคคลของนักศึกษาทุกคนในวิชา */
export async function listCourseStudentStats(
  courseId: number,
  params?: ListCourseStudentStatsParams
) {
  return apiFetch<StudentCourseStatItem[]>(
    `/api/courses/${courseId}/stats/students`,
    {
      method: "GET",
      params,
    }
  );
}

/** 33. GET /api/courses/:courseId/stats/students/:studentId — ดึงสถิติเชิงลึกและไทม์ไลน์รายบุคคลในรายวิชา */
export async function getStudentCourseStats(
  courseId: number,
  studentId: number
) {
  return apiFetch<StudentCourseDetailStats>(
    `/api/courses/${courseId}/stats/students/${studentId}`,
    {
      method: "GET",
    }
  );
}

// Grouped API client object for convenient namespace import
export const fairpickApi = {
  health: {
    checkDB: checkDatabaseHealth,
  },
  courses: {
    list: listCourses,
    create: createCourse,
    get: getCourse,
    update: updateCourse,
    delete: deleteCourse,
  },
  students: {
    list: listStudents,
    create: createStudent,
    get: getStudent,
    update: updateStudent,
    delete: deleteStudent,
  },
  enrollments: {
    importFile: importCourseStudentsFile,
    bulkEnroll: bulkEnrollCourseStudents,
    listByCourse: listCourseStudents,
    enroll: enrollStudentInCourse,
    withdraw: withdrawStudentFromCourse,
  },
  sessions: {
    listByCourse: listCourseSessions,
    start: startClassSession,
    get: getClassSession,
    end: endClassSession,
    switchMode: switchSessionPickMode,
  },
  attendance: {
    getSheet: getSessionAttendance,
    updateStudent: updateStudentAttendance,
    bulkUpdate: bulkUpdateSessionAttendance,
  },
  picks: {
    getPool: getSessionPickPool,
    execute: executeRandomPick,
    startNewRound: startNewPickRound,
    cancel: cancelPick,
    listBySession: listSessionPicks,
    listByCourse: listCoursePicks,
  },
  stats: {
    getOverview: getCourseStatsOverview,
    listStudents: listCourseStudentStats,
    getStudentDetail: getStudentCourseStats,
  },
} as const;

