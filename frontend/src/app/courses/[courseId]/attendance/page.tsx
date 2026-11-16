"use client";

import { useEffect, useState, useMemo, use } from "react";
import Link from "next/link";
import {
  getCourse,
  listCourseSessions,
  startClassSession,
  endClassSession,
  getSessionAttendance,
  updateStudentAttendance,
  bulkUpdateSessionAttendance,
} from "@/lib/api";
import { CourseDetail, ClassSession, SessionAttendanceData, AttendanceStatus, PickMode } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Play,
  Square,
  Clock,
  AlertCircle,
  CheckCheck,
  Search,
  Shuffle,
  CheckCircle2,
} from "lucide-react";

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
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Filter & Search
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | AttendanceStatus>("all");

  // Confirmation Dialog 1: Start Session
  const [startDialogOpen, setStartDialogOpen] = useState(false);
  const [topicInput, setTopicInput] = useState("");
  const [initialMode, setInitialMode] = useState<PickMode>("weighted");

  // Confirmation Dialog 2: End Session
  const [endDialogOpen, setEndDialogOpen] = useState(false);
  const [markUnconfirmedAbsent, setMarkUnconfirmedAbsent] = useState(true);

  // Confirmation Dialog 3: Mark All Present
  const [markAllDialogOpen, setMarkAllDialogOpen] = useState(false);

  const fetchSessionData = async () => {
    if (!Number.isInteger(courseId) || courseId <= 0) {
      setError("รหัสรายวิชาไม่ถูกต้อง");
      setLoading(false);
      return;
    }
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

      const active = list.find((s) => s.status === "active");
      if (active) {
        setActiveSession(active);
        const attRes = await getSessionAttendance(active.id);
        setAttendanceSheet(attRes.data);
      } else {
        setActiveSession(null);
        setAttendanceSheet(null);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถโหลดข้อมูลคาบเรียนและใบเช็กชื่อได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSessionData();
  }, [courseId]);

  const filteredRecords = useMemo(() => {
    const records = attendanceSheet?.records || [];
    return records.filter((r) => {
      if (statusFilter !== "all" && r.status !== statusFilter) return false;
      const q = searchQuery.trim().toLowerCase();
      if (!q) return true;
      return (
        r.studentCode.toLowerCase().includes(q) ||
        r.fullName.toLowerCase().includes(q)
      );
    });
  }, [attendanceSheet, statusFilter, searchQuery]);

  const handleConfirmStartSession = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setActionLoading(true);
      await startClassSession(courseId, {
        topic: topicInput.trim() || null,
        initialMode,
      });
      setStartDialogOpen(false);
      setTopicInput("");
      setFeedback({ type: "success", message: "เปิดคาบเรียนใหม่และสร้างใบเช็กชื่อสำเร็จ" });
      await fetchSessionData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เริ่มคาบเรียนไม่สำเร็จ: " + msg });
    } finally {
      setActionLoading(false);
    }
  };

  const handleConfirmEndSession = async () => {
    if (!activeSession) return;
    try {
      setActionLoading(true);
      await endClassSession(activeSession.id, {
        markUnconfirmedAsAbsent: markUnconfirmedAbsent,
      });
      setEndDialogOpen(false);
      setFeedback({
        type: "success",
        message: `ปิดคาบเรียนที่ #${activeSession.sessionNumber} และบันทึกสรุปผลการเข้าเรียนเรียบร้อยแล้ว`,
      });
      await fetchSessionData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "ปิดคาบเรียนไม่สำเร็จ: " + msg });
      setEndDialogOpen(false);
    } finally {
      setActionLoading(false);
    }
  };

  const handleConfirmMarkAllPresent = async () => {
    if (!activeSession) return;
    try {
      setActionLoading(true);
      await bulkUpdateSessionAttendance(activeSession.id, {
        markAllStatus: "present",
      });
      const attRes = await getSessionAttendance(activeSession.id);
      setAttendanceSheet(attRes.data);
      setMarkAllDialogOpen(false);
      setFeedback({ type: "success", message: "เช็กชื่อนักศึกษาทุกคนเป็น 'มาเรียน (Present)' สำเร็จ" });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เช็กชื่อทั้งหมดไม่สำเร็จ: " + msg });
      setMarkAllDialogOpen(false);
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
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "บันทึกการเช็กชื่อไม่สำเร็จ: " + msg });
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">ใบเช็กชื่อประจำคาบเรียน</h1>
            {course && (
              <Badge variant="outline" className="text-xs font-mono">
                {course.courseCode || "NO-CODE"} • {course.courseName}
              </Badge>
            )}
          </div>
          <p className="text-muted-foreground text-sm">
            จัดการสถานะการเข้าเรียน (มา / สาย / ขาด) เพื่อคัดกรองผู้มีสิทธิ์ถูกสุ่มในคาบนี้
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {activeSession ? (
            <>
              <Button
                variant="outline"
                size="sm"
                className="gap-1.5 text-emerald-700 border-emerald-200 hover:bg-emerald-50"
                disabled={actionLoading || (attendanceSheet?.records?.length || 0) === 0}
                onClick={() => setMarkAllDialogOpen(true)}
              >
                <CheckCheck className="size-4" /> เช็กมาเรียนทั้งหมด
              </Button>
              <Button
                render={<Link href={`/courses/${courseId}/pick`} />}
                size="sm"
                className="gap-1.5 bg-indigo-600 hover:bg-indigo-700 text-white"
              >
                <Shuffle className="size-4" /> ไปหน้าสุ่มวงล้อ
              </Button>
              <Button
                variant="destructive"
                size="sm"
                className="gap-1.5"
                disabled={actionLoading}
                onClick={() => setEndDialogOpen(true)}
              >
                <Square className="size-4 fill-current" /> ปิดคาบเรียน (จบคาบ #{activeSession.sessionNumber})
              </Button>
            </>
          ) : (
            <Button
              size="sm"
              className="gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
              disabled={actionLoading}
              onClick={() => setStartDialogOpen(true)}
            >
              <Play className="size-4 fill-current" /> เริ่มคาบเรียนใหม่
            </Button>
          )}
        </div>
      </div>

      {/* Confirmation Dialog 1: Start Session */}
      <Dialog open={startDialogOpen} onOpenChange={setStartDialogOpen}>
        <DialogContent>
          <form onSubmit={handleConfirmStartSession}>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Play className="size-4 text-emerald-600 fill-current" /> ยืนยันการเริ่มคาบเรียนใหม่
              </DialogTitle>
              <DialogDescription>
                สร้างคาบเรียนที่ #{sessions.length + 1} และเปิดใบเช็กชื่อสำหรับนักศึกษาในรายวิชา
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="att_topic">หัวข้อคาบเรียน (ไม่บังคับ)</Label>
                <Input
                  id="att_topic"
                  placeholder="เช่น บทที่ 1: โครงสร้างข้อมูลเบื้องต้น"
                  value={topicInput}
                  maxLength={255}
                  onChange={(e) => setTopicInput(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label>โหมดการสุ่มเริ่มต้นของคาบนี้</Label>
                <Select value={initialMode} onValueChange={(v) => setInitialMode(v as PickMode)}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="weighted">Weighted (ถ่วงน้ำหนัก W = 1 / (1 + H + 2C))</SelectItem>
                      <SelectItem value="no_repeat">No Repeat (สุ่มไม่ซ้ำคนในรอบเดียวกัน)</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setStartDialogOpen(false)} disabled={actionLoading}>
                ยกเลิก
              </Button>
              <Button type="submit" className="bg-emerald-600 hover:bg-emerald-700 text-white" disabled={actionLoading}>
                {actionLoading ? "กำลังเริ่มคาบเรียน..." : "ยืนยันเริ่มคาบเรียน"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog 2: End Session */}
      <Dialog open={endDialogOpen} onOpenChange={setEndDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="text-destructive flex items-center gap-2">
              <Square className="size-4 fill-current" /> ยืนยันการปิดคาบเรียนที่ #{activeSession?.sessionNumber}
            </DialogTitle>
            <DialogDescription>
              เมื่อปิดคาบเรียนแล้ว จะไม่สามารถแก้ไขสถานะเช็กชื่อหรือสุ่มรายชื่อในคาบนี้เพิ่มเติมได้อีก
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-3">
            {attendanceSheet?.summary && (
              <div className="grid grid-cols-4 gap-2 text-center bg-muted/50 p-3 rounded-lg border text-xs">
                <div>
                  <div className="text-emerald-600 font-bold text-base">{attendanceSheet.summary.present}</div>
                  <div className="text-muted-foreground">มาเรียน</div>
                </div>
                <div>
                  <div className="text-amber-600 font-bold text-base">{attendanceSheet.summary.late}</div>
                  <div className="text-muted-foreground">มาสาย</div>
                </div>
                <div>
                  <div className="text-rose-600 font-bold text-base">{attendanceSheet.summary.absent}</div>
                  <div className="text-muted-foreground">ขาดเรียน</div>
                </div>
                <div>
                  <div className="text-foreground font-bold text-base">{attendanceSheet.summary.unconfirmed}</div>
                  <div className="text-muted-foreground">ยังไม่เช็ก</div>
                </div>
              </div>
            )}

            <div className="flex items-center justify-between gap-3 rounded-lg border p-3 bg-muted/30">
              <div className="space-y-0.5">
                <Label className="text-sm font-medium">ปรับผู้ที่ "ยังไม่เช็ก" เป็น "ขาดเรียน" อัตโนมัติ</Label>
                <p className="text-xs text-muted-foreground">
                  หากเปิดไว้ นักศึกษาที่ยังไม่ถูกเช็กชื่อ ({attendanceSheet?.summary?.unconfirmed || 0} คน) จะถูกบันทึกเป็นขาดเรียนทันที
                </p>
              </div>
              <Switch checked={markUnconfirmedAbsent} onCheckedChange={setMarkUnconfirmedAbsent} />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setEndDialogOpen(false)} disabled={actionLoading}>
              ยกเลิก
            </Button>
            <Button type="button" variant="destructive" onClick={handleConfirmEndSession} disabled={actionLoading}>
              {actionLoading ? "กำลังปิดคาบเรียน..." : "ยืนยันปิดคาบเรียน"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog 3: Mark All Present */}
      <Dialog open={markAllDialogOpen} onOpenChange={setMarkAllDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-emerald-700">
              <CheckCheck className="size-5" /> ยืนยันการเช็กชื่อ "มาเรียนทั้งหมด"
            </DialogTitle>
            <DialogDescription>
              คุณต้องการปรับสถานะของนักศึกษาทุกคนในคาบนี้ ({attendanceSheet?.summary?.totalEnrolled || 0} คน) เป็น{" "}
              <span className="font-semibold text-emerald-600">มาเรียน (Present)</span> พร้อมกันใช่หรือไม่?
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 text-xs text-muted-foreground bg-muted/50 p-3 rounded-lg border">
            คุณยังสามารถกดเปลี่ยนสถานะเป็น "สาย" หรือ "ขาด" รายบุคคลในตารางได้ตามปกติหลังจากยืนยันแล้ว
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setMarkAllDialogOpen(false)} disabled={actionLoading}>
              ยกเลิก
            </Button>
            <Button
              type="button"
              className="bg-emerald-600 hover:bg-emerald-700 text-white"
              onClick={handleConfirmMarkAllPresent}
              disabled={actionLoading}
            >
              {actionLoading ? "กำลังบันทึก..." : "ยืนยันเช็กมาเรียนทั้งหมด"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {feedback && (
        <Alert variant={feedback.type === "error" ? "destructive" : "default"} className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {feedback.type === "error" ? <AlertCircle className="size-4" /> : <CheckCircle2 className="size-4 text-emerald-600" />}
            <AlertDescription>{feedback.message}</AlertDescription>
          </div>
          <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={() => setFeedback(null)}>
            ปิด
          </Button>
        </Alert>
      )}

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
            <Button
              onClick={() => setStartDialogOpen(true)}
              className="gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
            >
              <Play className="size-4 fill-current" /> เริ่มคาบเรียนใหม่
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <Card className="bg-muted/40 border">
            <CardHeader className="py-4">
              <div className="flex flex-wrap justify-between items-center gap-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge className="bg-emerald-600 text-white">กำลังเรียนอยู่</Badge>
                  <CardTitle className="text-lg">
                    คาบที่ #{activeSession.sessionNumber}
                    {activeSession.topic ? ` — ${activeSession.topic}` : ""}
                  </CardTitle>
                  <span className="text-xs text-muted-foreground">
                    เริ่มเมื่อ: {new Date(activeSession.startedAt || "").toLocaleTimeString("th-TH")}
                  </span>
                </div>

                {attendanceSheet?.summary && (
                  <div className="flex flex-wrap items-center gap-3 text-xs font-medium">
                    <span className="text-emerald-600">มา: {attendanceSheet.summary.present}</span>
                    <span className="text-amber-600">สาย: {attendanceSheet.summary.late}</span>
                    <span className="text-rose-600">ขาด: {attendanceSheet.summary.absent}</span>
                    <span className="text-muted-foreground">ยังไม่เช็ก: {attendanceSheet.summary.unconfirmed}</span>
                    <Badge variant="secondary">มีสิทธิ์สุ่ม: {attendanceSheet.summary.eligibleForPick} คน</Badge>
                  </div>
                )}
              </div>
            </CardHeader>
          </Card>

          {/* Search & Filter Controls */}
          <div className="flex flex-col sm:flex-row justify-between items-stretch sm:items-center gap-3">
            <div className="relative max-w-sm w-full">
              <Search className="size-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="ค้นหารหัสนักศึกษา หรือ ชื่อ-นามสกุล..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 h-9"
              />
            </div>

            <div className="flex flex-wrap items-center gap-1.5">
              {(
                [
                  { key: "all", label: `ทั้งหมด (${attendanceSheet?.records?.length || 0})` },
                  { key: "unconfirmed", label: `ยังไม่เช็ก (${attendanceSheet?.summary?.unconfirmed || 0})` },
                  { key: "present", label: `มา (${attendanceSheet?.summary?.present || 0})` },
                  { key: "late", label: `สาย (${attendanceSheet?.summary?.late || 0})` },
                  { key: "absent", label: `ขาด (${attendanceSheet?.summary?.absent || 0})` },
                ] as const
              ).map((tab) => (
                <Button
                  key={tab.key}
                  type="button"
                  size="sm"
                  variant={statusFilter === tab.key ? "default" : "outline"}
                  className="h-8 text-xs px-2.5"
                  onClick={() => setStatusFilter(tab.key)}
                >
                  {tab.label}
                </Button>
              ))}
            </div>
          </div>

          <Card>
            <CardContent className="p-0">
              {filteredRecords.length === 0 ? (
                <div className="text-center py-10 text-sm text-muted-foreground">
                  ไม่พบรายชื่อนักศึกษาตามเงื่อนไขที่เลือก
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[80px]">ลำดับ</TableHead>
                      <TableHead className="w-[160px]">รหัสนักศึกษา</TableHead>
                      <TableHead>ชื่อ - นามสกุล</TableHead>
                      <TableHead className="w-[130px] text-center">สถานะปัจจุบัน</TableHead>
                      <TableHead className="text-right">เช็กชื่อ</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredRecords.map((record, idx) => (
                      <TableRow key={record.studentId}>
                        <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                        <TableCell className="font-mono font-medium">{record.studentCode}</TableCell>
                        <TableCell>{record.fullName}</TableCell>
                        <TableCell className="text-center">
                          <Badge
                            variant="outline"
                            className={
                              record.status === "present"
                                ? "bg-emerald-50 text-emerald-700 border-emerald-300 dark:bg-emerald-950/40 dark:text-emerald-300"
                                : record.status === "late"
                                ? "bg-amber-50 text-amber-700 border-amber-300 dark:bg-amber-950/40 dark:text-amber-300"
                                : record.status === "absent"
                                ? "bg-rose-50 text-rose-700 border-rose-300 dark:bg-rose-950/40 dark:text-rose-300"
                                : "bg-gray-100 text-gray-700 border-gray-300 dark:bg-gray-800 dark:text-gray-300"
                            }
                          >
                            {record.status === "present" && "มาเรียน"}
                            {record.status === "late" && "มาสาย"}
                            {record.status === "absent" && "ขาดเรียน"}
                            {record.status === "unconfirmed" && "ยังไม่เช็ก"}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-1">
                            <Button
                              size="sm"
                              variant={record.status === "present" ? "default" : "outline"}
                              className={`h-7 px-2.5 text-xs ${
                                record.status === "present" ? "bg-emerald-600 hover:bg-emerald-700 text-white" : ""
                              }`}
                              onClick={() => handleStatusChange(record.studentId, "present")}
                            >
                              มา
                            </Button>
                            <Button
                              size="sm"
                              variant={record.status === "late" ? "default" : "outline"}
                              className={`h-7 px-2.5 text-xs ${
                                record.status === "late" ? "bg-amber-600 hover:bg-amber-700 text-white" : ""
                              }`}
                              onClick={() => handleStatusChange(record.studentId, "late")}
                            >
                              สาย
                            </Button>
                            <Button
                              size="sm"
                              variant={record.status === "absent" ? "destructive" : "outline"}
                              className="h-7 px-2.5 text-xs"
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
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
