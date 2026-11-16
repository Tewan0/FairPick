"use client";

import { useEffect, useState, use } from "react";
import { getCourse, listCourseStudents, bulkEnrollCourseStudents, withdrawStudentFromCourse, importCourseStudentsFile } from "@/lib/api";
import { CourseDetail, EnrolledStudent } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Users, UserPlus, Upload, Trash2, AlertCircle } from "lucide-react";

export default function CourseStudentsPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [students, setStudents] = useState<EnrolledStudent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Quick add / paste modal
  const [pasteModalOpen, setPasteModalOpen] = useState(false);
  const [rawStudentText, setRawStudentText] = useState("");
  const [pasting, setPasting] = useState(false);

  // File import
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [importingFile, setImportingFile] = useState(false);

  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [cRes, sRes] = await Promise.all([
        getCourse(courseId),
        listCourseStudents(courseId, { limit: 100 }),
      ]);
      setCourse(cRes.data);
      setStudents(sRes.data || []);
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถดึงข้อมูลนักศึกษาได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [courseId]);

  const handleWithdraw = async (studentId: number, name: string) => {
    if (!confirm(`ต้องการนำนักศึกษา "${name}" ออกจากรายวิชานี้หรือไม่?`)) return;
    try {
      await withdrawStudentFromCourse(courseId, studentId);
      fetchData();
    } catch (err: any) {
      alert("ถอนนักศึกษาไม่สำเร็จ: " + (err?.message || ""));
    }
  };

  const handleBulkPaste = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rawStudentText.trim()) return;

    // Parse lines: studentCode fullName
    const lines = rawStudentText.split("\n").filter((l) => l.trim().length > 0);
    const parsedStudents = lines.map((line) => {
      const parts = line.split(/[,\t ]+/).map((p) => p.trim()).filter(Boolean);
      return {
        studentCode: parts[0] || "",
        fullName: parts.slice(1).join(" ") || "นักศึกษา",
      };
    }).filter((s) => s.studentCode);

    if (parsedStudents.length === 0) {
      alert("ไม่พบรูปแบบข้อมูลนักศึกษาที่ถูกต้อง");
      return;
    }

    try {
      setPasting(true);
      await bulkEnrollCourseStudents(courseId, { students: parsedStudents });
      setPasteModalOpen(false);
      setRawStudentText("");
      fetchData();
    } catch (err: any) {
      alert("เพิ่มนักศึกษาไม่สำเร็จ: " + (err?.message || ""));
    } finally {
      setPasting(false);
    }
  };

  const handleFileUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedFile) return;

    try {
      setImportingFile(true);
      await importCourseStudentsFile(courseId, selectedFile);
      setSelectedFile(null);
      fetchData();
    } catch (err: any) {
      alert("อัปโหลดไฟล์ไม่สำเร็จ: " + (err?.message || ""));
    } finally {
      setImportingFile(false);
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <span>รายชื่อนักศึกษา</span>
            {course && (
              <Badge variant="outline" className="text-sm font-normal">
                {course.courseCode} - {course.courseName}
              </Badge>
            )}
          </h1>
          <p className="text-muted-foreground text-sm">
            จัดการนักศึกษาที่ลงทะเบียนในรายวิชานี้ (ทั้งหมด {students.length} คน)
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Dialog open={pasteModalOpen} onOpenChange={setPasteModalOpen}>
            <DialogTrigger render={<Button className="gap-1.5" size="sm" />}>
              <UserPlus className="size-4" /> เพิ่ม / วางรายชื่อ (Paste)
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <form onSubmit={handleBulkPaste}>
                <DialogHeader>
                  <DialogTitle>วางรายชื่อนักศึกษา (Bulk Paste)</DialogTitle>
                  <DialogDescription>
                    วางข้อมูลนักศึกษาทีละบรรทัด รูปแบบ: <code>รหัสนักศึกษา ชื่อ นามสกุล</code> (คั่นด้วยวรรค หรือ Tab)
                  </DialogDescription>
                </DialogHeader>

                <div className="py-4">
                  <Textarea
                    placeholder="65010001 สมชาย ใจดี&#10;65010002 สมหญิง รักเรียน"
                    rows={8}
                    value={rawStudentText}
                    onChange={(e) => setRawStudentText(e.target.value)}
                    required
                  />
                </div>

                <DialogFooter>
                  <Button type="button" variant="outline" onClick={() => setPasteModalOpen(false)}>
                    ยกเลิก
                  </Button>
                  <Button type="submit" disabled={pasting}>
                    {pasting ? "กำลังบันทึก..." : "นำเข้ารายชื่อ"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>

          <form onSubmit={handleFileUpload} className="flex items-center gap-2">
            <Input
              type="file"
              accept=".csv,.xlsx,.xls"
              className="max-w-[200px] h-9 text-xs"
              onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
            />
            <Button type="submit" variant="outline" size="sm" disabled={!selectedFile || importingFile} className="gap-1">
              <Upload className="size-3.5" />
              {importingFile ? "กำลังส่ง..." : "Import"}
            </Button>
          </form>
        </div>
      </div>

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
          ) : students.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground space-y-2">
              <Users className="size-10 mx-auto opacity-40" />
              <p>ยังไม่มีนักศึกษาลงทะเบียนในรายวิชานี้</p>
              <p className="text-xs">คลิก "เพิ่ม / วางรายชื่อ (Paste)" เพื่อเพิ่มนักศึกษาเข้าสู่ระบบ</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[120px]">ลำดับที่</TableHead>
                  <TableHead className="w-[180px]">รหัสนักศึกษา</TableHead>
                  <TableHead>ชื่อ - นามสกุล</TableHead>
                  <TableHead className="text-center w-[120px]">สถานะ</TableHead>
                  <TableHead className="text-right w-[100px]">จัดการ</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {students.map((item, idx) => (
                  <TableRow key={item.studentId || idx}>
                    <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                    <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                    <TableCell>{item.fullName}</TableCell>
                    <TableCell className="text-center">
                      <Badge variant="outline" className="text-emerald-600 border-emerald-200 bg-emerald-50 dark:bg-emerald-950/20">
                        {item.status || "active"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-muted-foreground hover:text-destructive size-8"
                        onClick={() => handleWithdraw(item.studentId, item.fullName)}
                        title="ถอนนักศึกษา"
                      >
                        <Trash2 className="size-4" />
                      </Button>
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
