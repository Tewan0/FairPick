"use client";

import { useEffect, useState, useMemo, useRef, use } from "react";
import {
  getCourse,
  listCourseStudents,
  enrollStudentInCourse,
  bulkEnrollCourseStudents,
  withdrawStudentFromCourse,
  importCourseStudentsFile,
} from "@/lib/api";
import { CourseDetail, EnrolledStudent, EnrollmentFilterStatus } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import {
  Users,
  UserPlus,
  Upload,
  Trash2,
  AlertCircle,
  Search,
  CheckCircle2,
  FileSpreadsheet,
  RotateCcw,
} from "lucide-react";

export default function CourseStudentsPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [students, setStudents] = useState<EnrolledStudent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Filter & Search
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<EnrollmentFilterStatus>("active");

  // Single add modal
  const [singleModalOpen, setSingleModalOpen] = useState(false);
  const [singleCode, setSingleCode] = useState("");
  const [singleName, setSingleName] = useState("");
  const [addingSingle, setAddingSingle] = useState(false);

  // Bulk paste modal
  const [pasteModalOpen, setPasteModalOpen] = useState(false);
  const [rawStudentText, setRawStudentText] = useState("");
  const [pasting, setPasting] = useState(false);

  // File import + confirmation dialog
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [fileConfirmOpen, setFileConfirmOpen] = useState(false);
  const [importingFile, setImportingFile] = useState(false);

  // Withdraw student confirmation dialog
  const [studentToWithdraw, setStudentToWithdraw] = useState<EnrolledStudent | null>(null);
  const [withdrawing, setWithdrawing] = useState(false);

  const fetchData = async (currentStatus: EnrollmentFilterStatus = statusFilter) => {
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
        listCourseStudents(courseId, { status: currentStatus, limit: 500 }),
      ]);
      setCourse(cRes.data);
      setStudents(sRes.data || []);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถดึงข้อมูลนักศึกษาได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData(statusFilter);
  }, [courseId, statusFilter]);

  const filteredStudents = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return students;
    return students.filter(
      (s) =>
        s.studentCode.toLowerCase().includes(q) ||
        s.fullName.toLowerCase().includes(q)
    );
  }, [students, searchQuery]);

  const handleSingleEnroll = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!singleCode.trim() || !singleName.trim()) return;
    try {
      setAddingSingle(true);
      await enrollStudentInCourse(courseId, {
        studentCode: singleCode.trim(),
        fullName: singleName.trim(),
      });
      setSingleModalOpen(false);
      setSingleCode("");
      setSingleName("");
      setFeedback({ type: "success", message: `เพิ่มนักศึกษา "${singleCode.trim()} ${singleName.trim()}" สำเร็จ` });
      await fetchData(statusFilter);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เพิ่มนักศึกษาไม่สำเร็จ: " + msg });
    } finally {
      setAddingSingle(false);
    }
  };

  const handleReEnroll = async (student: EnrolledStudent) => {
    try {
      await enrollStudentInCourse(courseId, { studentId: student.studentId });
      setFeedback({ type: "success", message: `คืนสถานะการลงทะเบียนของ "${student.fullName}" สำเร็จ` });
      await fetchData(statusFilter);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "คืนสถานะไม่สำเร็จ: " + msg });
    }
  };

  const handleConfirmWithdraw = async () => {
    if (!studentToWithdraw) return;
    try {
      setWithdrawing(true);
      const res = await withdrawStudentFromCourse(courseId, studentToWithdraw.studentId);
      setFeedback({
        type: "success",
        message: res.message || `นำนักศึกษา "${studentToWithdraw.fullName}" ออกจากรายวิชาสำเร็จ`,
      });
      setStudentToWithdraw(null);
      await fetchData(statusFilter);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "ถอนนักศึกษาไม่สำเร็จ: " + msg });
      setStudentToWithdraw(null);
    } finally {
      setWithdrawing(false);
    }
  };

  const handleBulkPaste = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rawStudentText.trim()) return;

    try {
      setPasting(true);
      const res = await bulkEnrollCourseStudents(courseId, { rawText: rawStudentText });
      const sum = res.data.summary;
      const addedTotal =
        sum.newlyCreatedAndEnrolled + sum.existingStudentEnrolled + sum.reactivatedEnrollment;
      setPasteModalOpen(false);
      setRawStudentText("");
      setFeedback({
        type: "success",
        message: `นำเข้ารายชื่อสำเร็จ: เพิ่มเข้าวิชา ${addedTotal} คน (ข้ามที่ซ้ำ ${sum.alreadyEnrolledSkipped} คน, แถวไม่สมบูรณ์ ${sum.invalidRowsSkipped} แถว)`,
      });
      await fetchData(statusFilter);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เพิ่มนักศึกษาไม่สำเร็จ: " + msg });
    } finally {
      setPasting(false);
    }
  };

  const handleSelectFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] || null;
    if (file) {
      setSelectedFile(file);
      setFileConfirmOpen(true);
    }
  };

  const handleConfirmFileUpload = async () => {
    if (!selectedFile) return;
    try {
      setImportingFile(true);
      const res = await importCourseStudentsFile(courseId, selectedFile);
      const sum = res.data.summary;
      const addedTotal =
        sum.newlyCreatedAndEnrolled + sum.existingStudentEnrolled + sum.reactivatedEnrollment;
      setFileConfirmOpen(false);
      setSelectedFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setFeedback({
        type: "success",
        message: `อัปโหลดไฟล์สำเร็จ: เพิ่มเข้าวิชา ${addedTotal} คน จากทั้งหมด ${sum.totalRowsRead} แถว (ข้ามซ้ำ ${sum.alreadyEnrolledSkipped} คน)`,
      });
      await fetchData(statusFilter);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "อัปโหลดไฟล์ไม่สำเร็จ: " + msg });
      setFileConfirmOpen(false);
    } finally {
      setImportingFile(false);
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex flex-wrap items-center gap-2">
            <span>รายชื่อนักศึกษา</span>
            {course && (
              <Badge variant="outline" className="text-xs font-mono">
                {course.courseCode || "NO-CODE"} • {course.courseName}
              </Badge>
            )}
          </h1>
          <p className="text-muted-foreground text-sm">
            จัดการนักศึกษาที่ลงทะเบียนในรายวิชานี้ (กำลังศึกษา {course?.enrolledCount || 0} คน)
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* Single Student Add Dialog */}
          <Dialog open={singleModalOpen} onOpenChange={setSingleModalOpen}>
            <DialogTrigger render={<Button variant="outline" size="sm" className="gap-1.5" />}>
              <UserPlus className="size-4" /> เพิ่มรายบุคคล
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={handleSingleEnroll}>
                <DialogHeader>
                  <DialogTitle>เพิ่มนักศึกษาเข้าสู่รายวิชา</DialogTitle>
                  <DialogDescription>
                    กรอกรหัสนักศึกษาและชื่อ-นามสกุลเพื่อลงทะเบียนเข้าสู่วิชานี้ทันที
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label htmlFor="single_code">รหัสนักศึกษา</Label>
                    <Input
                      id="single_code"
                      placeholder="เช่น 660510001"
                      value={singleCode}
                      maxLength={30}
                      onChange={(e) => setSingleCode(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="single_name">ชื่อ - นามสกุล</Label>
                    <Input
                      id="single_name"
                      placeholder="เช่น สมชาย ใจดี"
                      value={singleName}
                      maxLength={255}
                      onChange={(e) => setSingleName(e.target.value)}
                      required
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button type="button" variant="outline" onClick={() => setSingleModalOpen(false)}>
                    ยกเลิก
                  </Button>
                  <Button type="submit" disabled={addingSingle}>
                    {addingSingle ? "กำลังบันทึก..." : "ยืนยันเพิ่มนักศึกษา"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>

          {/* Bulk Paste Dialog */}
          <Dialog open={pasteModalOpen} onOpenChange={setPasteModalOpen}>
            <DialogTrigger render={<Button className="gap-1.5" size="sm" />}>
              <Users className="size-4" /> วางรายชื่อแบบกลุ่ม (Paste)
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <form onSubmit={handleBulkPaste}>
                <DialogHeader>
                  <DialogTitle>วางรายชื่อนักศึกษา (Bulk Paste)</DialogTitle>
                  <DialogDescription>
                    วางข้อมูลนักศึกษาทีละบรรทัด รูปแบบ: <code>รหัสนักศึกษา ชื่อ นามสกุล</code> (คั่นด้วยวรรค, คอมมา หรือ Tab จาก Excel)
                  </DialogDescription>
                </DialogHeader>

                <div className="py-4 space-y-2">
                  <Textarea
                    placeholder={"660510001 สมชาย ใจดี\n660510002 สมหญิง รักเรียน"}
                    rows={8}
                    value={rawStudentText}
                    onChange={(e) => setRawStudentText(e.target.value)}
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    พบข้อมูลพร้อมนำเข้า{" "}
                    <span className="font-semibold text-foreground">
                      {rawStudentText.split("\n").filter((l) => l.trim().length > 0).length}
                    </span>{" "}
                    บรรทัด
                  </p>
                </div>

                <DialogFooter>
                  <Button type="button" variant="outline" onClick={() => setPasteModalOpen(false)}>
                    ยกเลิก
                  </Button>
                  <Button type="submit" disabled={pasting}>
                    {pasting ? "กำลังนำเข้า..." : "ยืนยันนำเข้ารายชื่อ"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>

          {/* File Upload Trigger */}
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv,.xlsx"
            className="hidden"
            onChange={handleSelectFile}
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className="size-3.5" /> นำเข้าไฟล์ (.csv / .xlsx)
          </Button>
        </div>
      </div>

      {/* Confirmation Dialog: File Import */}
      <Dialog
        open={fileConfirmOpen}
        onOpenChange={(open) => {
          if (!open) {
            setFileConfirmOpen(false);
            setSelectedFile(null);
            if (fileInputRef.current) fileInputRef.current.value = "";
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileSpreadsheet className="size-5 text-emerald-600" /> ยืนยันการนำเข้าไฟล์รายชื่อนักศึกษา
            </DialogTitle>
            <DialogDescription>
              ตรวจสอบรายละเอียดไฟล์ก่อนนำเข้ารายชื่อนักศึกษาเข้าสู่รายวิชานี้
            </DialogDescription>
          </DialogHeader>
          {selectedFile && (
            <div className="py-3 px-4 rounded-lg border bg-muted/40 space-y-1 text-sm">
              <div className="font-medium text-foreground">{selectedFile.name}</div>
              <div className="text-xs text-muted-foreground">
                ขนาดไฟล์: {(selectedFile.size / 1024).toFixed(1)} KB • ระบบจะข้ามรหัสนักศึกษาที่ลงทะเบียนอยู่ในวิชานี้แล้วโดยอัตโนมัติ
              </div>
            </div>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={importingFile}
              onClick={() => {
                setFileConfirmOpen(false);
                setSelectedFile(null);
                if (fileInputRef.current) fileInputRef.current.value = "";
              }}
            >
              ยกเลิก
            </Button>
            <Button type="button" onClick={handleConfirmFileUpload} disabled={importingFile}>
              {importingFile ? "กำลังอัปโหลด..." : "ยืนยันนำเข้าไฟล์"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog: Withdraw Student */}
      <Dialog open={!!studentToWithdraw} onOpenChange={(open) => !open && setStudentToWithdraw(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="text-destructive flex items-center gap-2">
              <Trash2 className="size-5" /> ยืนยันการถอนนักศึกษาออกจากรายวิชา
            </DialogTitle>
            <DialogDescription>
              คุณต้องการนำนักศึกษา{" "}
              <span className="font-semibold text-foreground">
                {studentToWithdraw?.studentCode} — {studentToWithdraw?.fullName}
              </span>{" "}
              ออกจากรายวิชานี้ใช่หรือไม่?
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 text-xs text-muted-foreground bg-muted/50 p-3 rounded-lg border space-y-1">
            <p>• หากนักศึกษามีประวัติการเช็กชื่อหรือประวัติการถูกสุ่มแล้ว ระบบจะปรับสถานะเป็น "ถอนรายวิชา (Withdrawn)" เพื่อเก็บรักษาประวัติย้อนหลังไว้</p>
            <p>• นักศึกษาที่ถอนรายวิชาจะไม่ถูกนำมาคิดในวงล้อสุ่มของคาบเรียนถัดไป</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setStudentToWithdraw(null)} disabled={withdrawing}>
              ยกเลิก
            </Button>
            <Button type="button" variant="destructive" onClick={handleConfirmWithdraw} disabled={withdrawing}>
              {withdrawing ? "กำลังดำเนินการ..." : "ยืนยันการถอนนักศึกษา"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Search & Status Filter Bar */}
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

        <div className="flex items-center gap-1.5">
          {(
            [
              { key: "active", label: "กำลังศึกษา (Active)" },
              { key: "withdrawn", label: "ถอนแล้ว (Withdrawn)" },
              { key: "all", label: "ทั้งหมด" },
            ] as const
          ).map((tab) => (
            <Button
              key={tab.key}
              type="button"
              size="sm"
              variant={statusFilter === tab.key ? "default" : "outline"}
              className="h-8 text-xs px-3"
              onClick={() => setStatusFilter(tab.key)}
            >
              {tab.label}
            </Button>
          ))}
        </div>
      </div>

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

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="text-center py-12 text-muted-foreground">กำลังโหลดรายชื่อนักศึกษา...</div>
          ) : filteredStudents.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground space-y-2">
              <Users className="size-10 mx-auto opacity-40" />
              <p>ไม่พบรายชื่อนักศึกษาตามเงื่อนไขที่เลือก</p>
              <p className="text-xs">คลิก "วางรายชื่อแบบกลุ่ม (Paste)" หรือ "นำเข้าไฟล์ (.csv / .xlsx)" เพื่อเพิ่มนักศึกษาเข้าสู่ระบบ</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[100px]">ลำดับที่</TableHead>
                  <TableHead className="w-[180px]">รหัสนักศึกษา</TableHead>
                  <TableHead>ชื่อ - นามสกุล</TableHead>
                  <TableHead className="text-center w-[140px]">สถานะ</TableHead>
                  <TableHead className="text-right w-[120px]">จัดการ</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredStudents.map((item, idx) => (
                  <TableRow key={item.studentId || idx}>
                    <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                    <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                    <TableCell>{item.fullName}</TableCell>
                    <TableCell className="text-center">
                      {item.status === "withdrawn" ? (
                        <Badge variant="outline" className="text-rose-600 border-rose-200 bg-rose-50 dark:bg-rose-950/20">
                          ถอนรายวิชาแล้ว
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-emerald-600 border-emerald-200 bg-emerald-50 dark:bg-emerald-950/20">
                          กำลังศึกษา
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {item.status === "withdrawn" ? (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 px-2.5 text-xs gap-1"
                          onClick={() => handleReEnroll(item)}
                          title="ลงทะเบียนกลับเข้าวิชา"
                        >
                          <RotateCcw className="size-3" /> คืนสถานะ
                        </Button>
                      ) : (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-muted-foreground hover:text-destructive size-8"
                          onClick={() => setStudentToWithdraw(item)}
                          title="ถอนนักศึกษาออกจากวิชา"
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
