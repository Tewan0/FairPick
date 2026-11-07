// frontend/src/types/fairpick.ts
// TypeScript Type Definitions for FairPick REST API (All 33 Endpoints)

export type EnrollmentStatus = "active" | "withdrawn";
export type EnrollmentFilterStatus = "active" | "withdrawn" | "all";
export type SessionStatus = "active" | "ended";
export type PickMode = "weighted" | "no_repeat";
export type AttendanceStatus = "present" | "absent" | "late" | "unconfirmed";
export type PickStatus = "valid" | "cancelled";

export type FairPickErrorCode =
  | "INVALID_REQUEST"
  | "VALIDATION_ERROR"
  | "RESOURCE_NOT_FOUND"
  | "DUPLICATE_RESOURCE"
  | "COURSE_HAS_HISTORY"
  | "STUDENT_HAS_HISTORY"
  | "ACTIVE_SESSION_EXISTS"
  | "SESSION_ALREADY_ENDED"
  | "PICK_ALREADY_CANCELLED"
  | "STUDENT_NOT_ENROLLED"
  | "NO_ELIGIBLE_STUDENTS"
  | "ROUND_COMPLETED"
  | "INTERNAL_SERVER_ERROR"
  | "DATABASE_UNAVAILABLE"
  | "UNKNOWN_ERROR";

// ==================== Standard Envelopes ====================

export interface PaginationMeta {
  page: number;
  limit: number;
  totalItems: number;
  totalPages: number;
}

export interface ApiSuccessResponse<T> {
  success: true;
  message: string;
  data: T;
  meta?: PaginationMeta;
}

export interface ApiErrorDetail {
  code: FairPickErrorCode | string;
  message: string;
  details?: Record<string, string>;
}

export interface ApiErrorResponse {
  success: false;
  error: ApiErrorDetail;
}

// ==================== 0. Health Check ====================

export interface HealthCheckDBResponse {
  status: "healthy" | "unhealthy";
  database: "connected" | "disconnected";
  host?: string;
  port?: string;
  db_name?: string;
  message?: string;
}

// ==================== 1. Courses (5 Endpoints) ====================

export interface Course {
  id: number;
  courseCode: string | null; // 6-digit string e.g. "001101" or null
  courseName: string | null;
  section: string | null;
  semester: string | null;
  enrolledCount: number;
  totalSessions: number;
  activeSessionId: number | null;
  createdAt: string;
  updatedAt: string;
}

export interface ActiveSessionBrief {
  id: number;
  sessionNumber: number;
  topic: string | null;
  currentMode: PickMode;
  currentRound: number;
  status: SessionStatus;
  startedAt: string;
}

export interface CourseDetail {
  id: number;
  courseCode: string | null;
  courseName: string | null;
  section: string | null;
  semester: string | null;
  enrolledCount: number;
  totalSessions: number;
  activeSession: ActiveSessionBrief | null;
  createdAt: string;
  updatedAt: string;
}

export interface CourseUpsertPayload {
  courseCode?: string | null;
  courseName?: string | null;
  section?: string | null;
  semester?: string | null;
}

export interface ListCoursesParams {
  search?: string;
  semester?: string;
  page?: number;
  limit?: number;
}

export interface DeleteCourseResult {
  deletedCourseId: number;
}

// ==================== 2. Students (5 Endpoints) ====================

export interface Student {
  id: number;
  studentCode: string; // Always string to preserve leading zeros
  fullName: string;
  activeEnrollmentsCount?: number;
  createdAt: string;
  updatedAt: string;
}

export interface StudentCreatedResult extends Student {
  enrolledInCourseId: number | null;
  addedToActiveSessionId: number | null;
}

export interface StudentCourseMembership {
  courseId: number;
  courseCode: string | null;
  courseName: string | null;
  section: string | null;
  semester: string | null;
  enrollmentStatus: EnrollmentStatus;
  enrolledAt: string;
  withdrawnAt: string | null;
}

export interface StudentDetail {
  id: number;
  studentCode: string;
  fullName: string;
  courses: StudentCourseMembership[];
  createdAt: string;
  updatedAt: string;
}

export interface CreateStudentPayload {
  studentCode: string;
  fullName: string;
  courseId?: number | null;
}

export interface UpdateStudentPayload {
  studentCode: string;
  fullName: string;
}

export interface ListStudentsParams {
  search?: string;
  page?: number;
  limit?: number;
}

export interface DeleteStudentResult {
  deletedStudentId: number;
}

// ==================== 3 & 4. Bulk Import & Enrollments (5 Endpoints) ====================

export interface EnrolledStudent {
  enrollmentId: number;
  courseId: number;
  studentId: number;
  studentCode: string;
  fullName: string;
  status: EnrollmentStatus;
  enrolledAt: string;
  withdrawnAt: string | null;
}

export interface EnrollStudentResult extends EnrolledStudent {
  addedToActiveSessionId: number | null;
}

export interface EnrollStudentPayload {
  studentId?: number;
  studentCode?: string;
  fullName?: string;
}

export interface ListCourseStudentsParams {
  status?: EnrollmentFilterStatus;
  search?: string;
  page?: number;
  limit?: number;
}

export interface WithdrawStudentResult {
  courseId: number;
  studentId: number;
  action: "withdrawn" | "deleted";
  status: EnrollmentStatus | "deleted";
  historyPreserved: boolean;
  withdrawnAt: string | null;
}

export interface BulkStudentInputItem {
  studentCode: string;
  fullName: string;
}

export interface BulkEnrollPayload {
  students?: BulkStudentInputItem[];
  rawText?: string;
}

export interface BulkImportInvalidRow {
  rowNumber: number;
  rawContent: string;
  reason: string;
}

export interface BulkImportSummary {
  totalRowsRead: number;
  newlyCreatedAndEnrolled: number;
  existingStudentEnrolled: number;
  reactivatedEnrollment: number;
  alreadyEnrolledSkipped: number;
  invalidRowsSkipped: number;
}

export interface BulkImportResult {
  courseId: number;
  activeSessionIdUpdated: number | null;
  summary: BulkImportSummary;
  invalidRows: BulkImportInvalidRow[];
}

// ==================== 5. Class Sessions (5 Endpoints) ====================

export interface AttendanceSummary {
  totalEnrolled: number;
  present: number;
  late: number;
  absent: number;
  unconfirmed: number;
  confirmedCount: number; // present + late + absent
  eligibleForPick: number; // present + late (active enrollments)
}

export interface SessionPickSummary {
  totalValidPicks: number;
  totalCancelledPicks: number;
  remainingInCurrentRound?: number;
}

export interface ClassSession {
  id: number;
  courseId: number;
  sessionNumber: number;
  topic: string | null;
  currentMode: PickMode;
  currentRound: number;
  status: SessionStatus;
  startedAt: string;
  endedAt: string | null;
  attendanceSummary: AttendanceSummary;
  pickSummary?: SessionPickSummary;
}

export interface SessionDetail extends ClassSession {
  courseCode: string | null;
  courseName: string | null;
  section: string | null;
  semester: string | null;
  pickSummary: SessionPickSummary;
}

export interface ListCourseSessionsParams {
  status?: SessionStatus;
  page?: number;
  limit?: number;
}

export interface StartSessionPayload {
  topic?: string | null;
  initialMode?: PickMode;
}

export interface EndSessionPayload {
  markUnconfirmedAsAbsent?: boolean;
}

export interface EndSessionResult {
  id: number;
  courseId: number;
  sessionNumber: number;
  status: "ended";
  startedAt: string;
  endedAt: string;
  markUnconfirmedAsAbsentUsed: boolean;
  unconfirmedRemainingCount: number;
  hasUnconfirmedWarning: boolean;
  warningMessage: string | null;
  attendanceSummary: AttendanceSummary;
}

export interface SwitchModePayload {
  mode: PickMode;
}

export interface SwitchModeResult {
  sessionId: number;
  currentMode: PickMode;
  currentRound: number;
  eligibleCount: number;
  remainingInRound: number;
  isRoundCompleted: boolean;
}

// ==================== 6. Attendance (3 Endpoints) ====================

export interface AttendanceRecord {
  attendanceId: number;
  studentId: number;
  studentCode: string;
  fullName: string;
  status: AttendanceStatus;
  isEligibleForPick: boolean;
  checkedAt: string | null;
  note: string | null;
}

export interface SingleAttendanceUpdateResult extends AttendanceRecord {
  sessionId: number;
  courseId: number;
}

export interface SessionAttendanceData {
  sessionId: number;
  courseId: number;
  sessionStatus: SessionStatus;
  hasUnconfirmedWarning: boolean;
  summary: AttendanceSummary;
  records: AttendanceRecord[];
}

export interface GetSessionAttendanceParams {
  status?: AttendanceStatus;
  search?: string;
}

export interface UpdateAttendancePayload {
  status: AttendanceStatus;
  note?: string | null;
}

export interface BulkAttendanceItemPayload {
  studentId: number;
  status: AttendanceStatus;
  note?: string | null;
}

export interface BulkUpdateAttendancePayload {
  markAllStatus?: AttendanceStatus;
  items?: BulkAttendanceItemPayload[];
}

export interface BulkUpdateAttendanceResult {
  sessionId: number;
  updatedCount: number;
  summary: AttendanceSummary;
}

// ==================== 7. Random Pick (3 Endpoints) ====================

export interface PickPoolCandidate {
  studentId: number;
  studentCode: string;
  fullName: string;
  attendanceStatus: "present" | "late";
  recent4SessionsPickCount: number; // H
  currentSessionPickCount: number; // C
  weightScore: number; // W = 1 / (1 + H + 2C)
  probabilityPercent: number;
  pickedInCurrentNoRepeatRound: boolean;
}

export interface PickPoolData {
  sessionId: number;
  courseId: number;
  sessionNumber: number;
  currentMode: PickMode;
  currentRound: number;
  weightFormula: string;
  previousSessionsUsed: number[];
  totalEligible: number;
  remainingInNoRepeatRound: number;
  isNoRepeatRoundCompleted: boolean;
  pool: PickPoolCandidate[];
}

export interface ExecutePickPayload {
  mode?: PickMode;
  autoAdvanceRound?: boolean; // default false
}

export interface ExecutePickResult {
  pick: {
    id: number;
    courseId: number;
    sessionId: number;
    sessionNumber: number;
    mode: PickMode;
    roundNumber: number;
    status: PickStatus;
    weightScore: number | null;
    pickedAt: string;
    student: {
      id: number;
      studentCode: string;
      fullName: string;
      attendanceStatus: "present" | "late";
    };
  };
  poolState: {
    currentMode: PickMode;
    currentRound: number;
    roundReset: boolean;
    isRoundCompleted: boolean;
    totalEligible: number;
    remainingInRound: number;
  };
}

export interface StartNewRoundResult {
  sessionId: number;
  previousRound: number;
  currentRound: number;
  isRoundCompleted: boolean;
  totalEligible: number;
  remainingInRound: number;
}

// ==================== 8. Cancel Pick (1 Endpoint) ====================

export interface CancelPickPayload {
  cancelReason?: string | null;
  markStudentAbsent?: boolean;
}

export interface CancelPickResult {
  id: number;
  courseId: number;
  sessionId: number;
  studentId: number;
  studentCode: string;
  fullName: string;
  mode: PickMode;
  roundNumber: number;
  status: "cancelled";
  pickedAt: string;
  cancelledAt: string;
  cancelReason: string | null;
}

// ==================== 9. Pick History (2 Endpoints) ====================

export interface PickHistoryItem {
  id: number;
  courseId: number;
  sessionId: number;
  sessionNumber: number;
  sessionTopic?: string | null;
  studentId: number;
  studentCode: string;
  fullName: string;
  mode: PickMode;
  roundNumber: number;
  status: PickStatus;
  weightScore: number | null;
  pickedAt: string;
  cancelledAt: string | null;
  cancelReason: string | null;
}

export interface SessionPickHistoryData {
  sessionId: number;
  summary: {
    totalPicks: number;
    validPicks: number;
    cancelledPicks: number;
  };
  items: PickHistoryItem[];
}

export interface ListSessionPicksParams {
  status?: PickStatus;
  mode?: PickMode;
  roundNumber?: number;
}

export interface ListCoursePicksParams {
  sessionId?: number;
  studentId?: number;
  mode?: PickMode;
  status?: PickStatus;
  search?: string;
  page?: number;
  limit?: number;
}

// ==================== 10. Statistics (3 Endpoints) ====================

export interface StatsCourseBrief {
  id: number;
  courseCode: string | null;
  courseName: string | null;
  section: string | null;
  semester: string | null;
}

export interface SessionStatBreakdown {
  sessionId: number;
  sessionNumber: number;
  topic: string | null;
  startedAt: string;
  status: SessionStatus;
  presentCount: number;
  lateCount: number;
  absentCount: number;
  unconfirmedCount: number;
  confirmedCount: number;
  attendanceRatePercent: number;
  validPicksCount: number;
}

export interface CourseStatsOverview {
  course: StatsCourseBrief;
  overall: {
    totalActiveStudents: number;
    totalWithdrawnStudents: number;
    totalSessions: number;
    averageAttendanceRatePercent: number;
    totalValidPicks: number;
    totalCancelledPicks: number;
    weightedValidPicks: number;
    noRepeatValidPicks: number;
    studentsNeverPickedCount: number;
  };
  sessionBreakdown: SessionStatBreakdown[];
}

export interface StudentAttendanceStatSummary {
  totalSessions: number;
  confirmedSessionsCount: number;
  presentCount: number;
  lateCount: number;
  absentCount: number;
  unconfirmedCount: number;
  attendedCount: number;
  attendanceRatePercent: number;
}

export interface StudentCourseStatItem {
  studentId: number;
  studentCode: string;
  fullName: string;
  enrollmentStatus: EnrollmentStatus;
  attendance: StudentAttendanceStatSummary;
  picks: {
    totalValidPicks: number;
    weightedValidPicks: number;
    noRepeatValidPicks: number;
    cancelledPicks: number;
    recent4SessionsPickCount: number;
    lastPickedAt: string | null;
  };
}

export interface ListCourseStudentStatsParams {
  enrollmentStatus?: EnrollmentFilterStatus;
  search?: string;
  sortBy?: "studentCode" | "attendanceRate" | "validPickCount";
  sortOrder?: "asc" | "desc";
}

export interface StudentAttendanceHistoryItem {
  sessionId: number;
  sessionNumber: number;
  topic: string | null;
  startedAt: string;
  status: AttendanceStatus;
  checkedAt: string | null;
  note: string | null;
}

export interface StudentPickHistoryItem {
  pickId: number;
  sessionId: number;
  sessionNumber: number;
  mode: PickMode;
  roundNumber: number;
  status: PickStatus;
  weightScore: number | null;
  pickedAt: string;
  cancelledAt: string | null;
  cancelReason: string | null;
}

export interface StudentCourseDetailStats {
  course: StatsCourseBrief;
  student: {
    id: number;
    studentCode: string;
    fullName: string;
    enrollmentStatus: EnrollmentStatus;
    enrolledAt: string;
    withdrawnAt: string | null;
  };
  attendanceSummary: StudentAttendanceStatSummary;
  pickSummary: {
    totalValidPicks: number;
    weightedValidPicks: number;
    noRepeatValidPicks: number;
    cancelledPicks: number;
  };
  attendanceHistory: StudentAttendanceHistoryItem[];
  pickHistory: StudentPickHistoryItem[];
}

