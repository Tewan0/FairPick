"use client";

import { useEffect, useState, use } from "react";
import {
  getCourse,
  listCourseSessions,
  startClassSession,
  endClassSession,
  getSessionAttendance,
  updateStudentAttendance,
} from "@/lib/api";
import { CourseDetail, ClassSession, SessionAttendanceData, AttendanceStatus } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Play, Square, Clock, AlertCircle } from "lucide-react";

export default function CourseAttendancePage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [sessions, setSessions] = useState<ClassSession[]>([]);
  const [activeSession, setActiveSession] = useState<ClassSession | null>(null);
  const [attendanceSheet, setAttendanceSheet] = useState<SessionAttendanceData | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchSessionData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [cRes, sRes] = await Promise.all([
        getCourse(courseId),
        listCourseSessions(courseId),
      ]);
      setCourse(cRes.data);
      const list = sRes.data || [];
      setSessions(list);

      // Find currently active session if any
      const active = list.find((s) => s.status === "active");
      if (active) {
        setActiveSession(active);
        const attRes = await getSessionAttendance(active.id);
        setAttendanceSheet(attRes.data);
      } else {
        setActiveSession(null);
        setAttendanceSheet(null);
      }
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถโหลดข้อมูลคาบเรียนและใบเช็กชื่อได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSessionData();
  }, [courseId]);

  const handleStartSession = async () => {
    try {
      setActionLoading(true);
      await startClassSession(courseId);
      await fetchSessionData();
    } catch (err: any) {
      alert("เริ่มคาบเรียนไม่สำเร็จ: " + (err?.message || ""));
    } finally {
      setActionLoading(false);
    }
  };

  const handleEndSession = async () => {
    if (!activeSession) return;
    if (!confirm("คุณต้องการปิดคาบเรียนนี้ใช่หรือไม่? เมื่อปิดแล้วระบบจะสรุปผลและหยุดสุ่มประจำคาบ")) return;
    try {
      setActionLoading(true);
      await endClassSession(activeSession.id, { markUnconfirmedAsAbsent: true });
      await fetchSessionData();
    } catch (err: any) {
      alert("ปิดคาบเรียนไม่สำเร็จ: " + (err?.message || ""));
    } finally {
      setActionLoading(false);
    }
  };

  const handleStatusChange = async (studentId: number, status: AttendanceStatus) => {
    if (!activeSession) return;
    try {
      await updateStudentAttendance(activeSession.id, studentId, { status });
      const attRes = await getSessionAttendance(activeSession.id);
      setAttendanceSheet(attRes.data);
    } catch (err: any) {
      alert("บันทึกการเช็กชื่อไม่สำเร็จ: " + (err?.message || ""));
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">ใบเช็กชื่อประจำคาบเรียน</h1>
          <p className="text-muted-foreground text-sm">
            จัดการสถานะการเข้าเรียน (มา / สาย / ลา / ขาด) เพื่อคัดกรองผู้มีสิทธิ์ถูกสุ่มในคาบนี้
          </p>
        </div>

        <div>
          {activeSession ? (
            <Button
              variant="destructive"
              size="sm"
              className="gap-1.5"
              disabled={actionLoading}
              onClick={handleEndSession}
            >
              <Square className="size-4 fill-current" /> ปิดคาบเรียน (จบคาบ #{activeSession.sessionNumber})
            </Button>
          ) : (
            <Button
              size="sm"
              className="gap-1.5 bg-emerald-600 hover:bg-emerald-700"
              disabled={actionLoading}
              onClick={handleStartSession}
            >
              <Play className="size-4 fill-current" /> เริ่มคาบเรียนใหม่
            </Button>
          )}
        </div>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>เกิดข้อผิดพลาด</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-12 text-muted-foreground">กำลังโหลดข้อมูลการเช็กชื่อ...</div>
      ) : !activeSession ? (
        <Card className="text-center py-12 border-dashed">
          <CardContent className="space-y-4">
            <Clock className="size-12 mx-auto text-muted-foreground opacity-50" />
            <div>
              <h3 className="text-lg font-semibold">ยังไม่มีคาบเรียนที่กำลังเปิดอยู่</h3>
              <p className="text-sm text-muted-foreground max-w-md mx-auto">
                คลิกปุ่ม "เริ่มคาบเรียนใหม่" ด้านบนเพื่อสร้างคาบเรียนและเปิดใบเช็กชื่อสำหรับนักศึกษา
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <Card className="bg-muted/40 border">
            <CardHeader className="py-4">
              <div className="flex flex-wrap justify-between items-center gap-3">
                <div className="flex items-center gap-2">
                  <Badge className="bg-emerald-600">กำลังเรียนอยู่</Badge>
                  <CardTitle className="text-lg">คาบที่ #{activeSession.sessionNumber}</CardTitle>
                  <span className="text-xs text-muted-foreground">
                    เริ่มเมื่อ: {new Date(activeSession.startedAt || "").toLocaleTimeString("th-TH")}
                  </span>
                </div>

                {attendanceSheet?.summary && (
                  <div className="flex items-center gap-3 text-xs font-medium">
                    <span className="text-emerald-600">มา: {attendanceSheet.summary.present}</span>
                    <span className="text-amber-600">สาย: {attendanceSheet.summary.late}</span>
                    <span className="text-blue-600">ลา: {attendanceSheet.summary.absent}</span>
                    <span className="text-muted-foreground">ยังไม่ระบุ: {attendanceSheet.summary.unconfirmed}</span>
                    <Badge variant="secondary">มีสิทธิ์สุ่ม: {attendanceSheet.summary.eligibleForPick} คน</Badge>
                  </div>
                )}
              </div>
            </CardHeader>
          </Card>

          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[80px]">ลำดับ</TableHead>
                    <TableHead className="w-[160px]">รหัสนักศึกษา</TableHead>
                    <TableHead>ชื่อ - นามสกุล</TableHead>
                    <TableHead className="w-[120px] text-center">สถานะปัจจุบัน</TableHead>
                    <TableHead className="text-right">เปลี่ยนสถานะ</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {attendanceSheet?.records?.map((record, idx) => (
                    <TableRow key={record.studentId}>
                      <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                      <TableCell className="font-mono font-medium">{record.studentCode}</TableCell>
                      <TableCell>{record.fullName}</TableCell>
                      <TableCell className="text-center">
                        <Badge
                          variant="outline"
                          className={
                            record.status === "present"
                              ? "bg-emerald-50 text-emerald-700 border-emerald-300"
                              : record.status === "late"
                              ? "bg-amber-50 text-amber-700 border-amber-300"
                              : record.status === "absent"
                              ? "bg-rose-50 text-rose-700 border-rose-300"
                              : "bg-gray-100 text-gray-700 border-gray-300"
                          }
                        >
                          {record.status === "present" && "มา"}
                          {record.status === "late" && "สาย"}
                          {record.status === "absent" && "ขาด"}
                          {record.status === "unconfirmed" && "ยังไม่เช็ก"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            size="sm"
                            variant={record.status === "present" ? "default" : "outline"}
                            className="h-7 px-2 text-xs"
                            onClick={() => handleStatusChange(record.studentId, "present")}
                          >
                            มา
                          </Button>
                          <Button
                            size="sm"
                            variant={record.status === "late" ? "default" : "outline"}
                            className="h-7 px-2 text-xs"
                            onClick={() => handleStatusChange(record.studentId, "late")}
                          >
                            สาย
                          </Button>
                          <Button
                            size="sm"
                            variant={record.status === "absent" ? "destructive" : "outline"}
                            className="h-7 px-2 text-xs"
                            onClick={() => handleStatusChange(record.studentId, "absent")}
                          >
                            ขาด
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
