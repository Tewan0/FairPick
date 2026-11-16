"use client";

import { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { listCourses, createCourse, updateCourse, deleteCourse } from "@/lib/api";
import { Course } from "@/types/fairpick";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  BookOpen,
  Plus,
  Trash2,
  Pencil,
  Users,
  Shuffle,
  ClipboardCheck,
  History,
  BarChart3,
  AlertCircle,
  Search,
  CheckCircle2,
  Radio,
} from "lucide-react";

const COURSE_CODE_REGEX = /^[A-Za-z0-9]{1,10}$/;

export default function CoursesPage() {
  const [courses, setCourses] = useState<Course[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  // Form states for Create / Edit course
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingCourse, setEditingCourse] = useState<Course | null>(null);
  const [courseCode, setCourseCode] = useState("");
  const [courseName, setCourseName] = useState("");
  const [section, setSection] = useState("001");
  const [academicYear, setAcademicYear] = useState("2569");
  const [semesterPart, setSemesterPart] = useState("1");
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Confirmation Dialog state for Delete course
  const [courseToDelete, setCourseToDelete] = useState<Course | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchCourses = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await listCourses({ limit: 100 });
      setCourses(res.data || []);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถโหลดข้อมูลรายวิชาได้ (โปรดตรวจสอบว่า Backend และ Database พร้อมทำงาน)");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCourses();
  }, []);

  const filteredCourses = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return courses;
    return courses.filter(
      (c) =>
        (c.courseCode || "").toLowerCase().includes(q) ||
        (c.courseName || "").toLowerCase().includes(q) ||
        (c.section || "").toLowerCase().includes(q) ||
        (c.semester || "").toLowerCase().includes(q)
    );
  }, [courses, searchQuery]);

  const openCreateDialog = () => {
    setEditingCourse(null);
    setCourseCode("");
    setCourseName("");
    setSection("001");
    setSemesterPart("1");
    setAcademicYear("2569");
    setFormError(null);
    setIsDialogOpen(true);
  };

  const openEditDialog = (course: Course) => {
    setEditingCourse(course);
    setCourseCode(course.courseCode || "");
    setCourseName(course.courseName || "");
    setSection(course.section || "");
    if (course.semester && course.semester.includes("/")) {
      const [sem, yr] = course.semester.split("/");
      setSemesterPart(sem || "1");
      setAcademicYear(yr || "2569");
    } else {
      setSemesterPart(course.semester || "1");
      setAcademicYear("2569");
    }
    setFormError(null);
    setIsDialogOpen(true);
  };

  const handleCourseCodeChange = (raw: string) => {
    // Allow only English letters and digits, up to 10 characters
    const cleaned = raw.replace(/[^A-Za-z0-9]/g, "").slice(0, 10);
    setCourseCode(cleaned);
    if (formError) setFormError(null);
  };

  const handleSaveCourse = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    const trimmedCode = courseCode.trim();
    const trimmedName = courseName.trim();
    const trimmedSection = section.trim();
    const combinedSemester =
      semesterPart.trim() && academicYear.trim()
        ? `${semesterPart.trim()}/${academicYear.trim()}`
        : semesterPart.trim() || academicYear.trim();

    if (!trimmedCode && !trimmedName) {
      setFormError("กรุณาระบุรหัสวิชาหรือชื่อวิชาอย่างน้อย 1 อย่าง");
      return;
    }

    if (trimmedCode && !COURSE_CODE_REGEX.test(trimmedCode)) {
      setFormError("รหัสวิชาต้องเป็นตัวอักษรภาษาอังกฤษ (A-Z, a-z) หรือตัวเลข (0-9) ไม่เกิน 10 ตัวอักษร");
      return;
    }

    try {
      setSubmitting(true);
      const payload = {
        courseCode: trimmedCode || null,
        courseName: trimmedName || null,
        section: trimmedSection || null,
        semester: combinedSemester || null,
      };

      if (editingCourse) {
        await updateCourse(editingCourse.id, payload);
        setFeedback({ type: "success", message: `แก้ไขรายวิชา "${trimmedCode || trimmedName}" สำเร็จ` });
      } else {
        await createCourse(payload);
        setFeedback({ type: "success", message: `สร้างรายวิชา "${trimmedCode || trimmedName}" สำเร็จ` });
      }

      setIsDialogOpen(false);
      await fetchCourses();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "เกิดข้อผิดพลาดในการบันทึกรายวิชา";
      setFormError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const handleConfirmDeleteCourse = async () => {
    if (!courseToDelete) return;
    try {
      setDeleting(true);
      await deleteCourse(courseToDelete.id);
      setFeedback({
        type: "success",
        message: `ลบรายวิชา "${courseToDelete.courseCode || ""} ${courseToDelete.courseName || ""}" เรียบร้อยแล้ว`,
      });
      setCourseToDelete(null);
      await fetchCourses();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "ไม่สามารถลบรายวิชาได้";
      setFeedback({ type: "error", message: `ลบรายวิชาไม่สำเร็จ: ${msg}` });
      setCourseToDelete(null);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">รายวิชาทั้งหมด</h1>
          <p className="text-muted-foreground text-sm">
            จัดการรายวิชา นักศึกษา บันทึกการเข้าเรียน และระบบสุ่มในห้องเรียน
          </p>
        </div>

        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger render={<Button className="gap-2" onClick={openCreateDialog} />}>
            <Plus className="size-4" /> สร้างรายวิชาใหม่
          </DialogTrigger>
          <DialogContent>
            <form onSubmit={handleSaveCourse}>
              <DialogHeader>
                <DialogTitle>{editingCourse ? "แก้ไขข้อมูลรายวิชา" : "เพิ่มรายวิชาใหม่"}</DialogTitle>
                <DialogDescription>
                  กรอกรหัสวิชา (ภาษาอังกฤษหรือตัวเลข ไม่เกิน 10 ตัว) ชื่อวิชา กลุ่มเรียน และภาคการศึกษา
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4 py-4">
                {formError && (
                  <Alert variant="destructive" className="py-2.5">
                    <AlertCircle className="size-4" />
                    <AlertDescription className="text-xs">{formError}</AlertDescription>
                  </Alert>
                )}

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <div className="space-y-2 sm:col-span-2">
                    <div className="flex items-center justify-between">
                      <Label htmlFor="course_code">รหัสวิชา (อังกฤษ/ตัวเลข ไม่เกิน 10 ตัว)</Label>
                      <span className="text-xs text-muted-foreground font-mono">
                        {courseCode.length}/10
                      </span>
                    </div>
                    <Input
                      id="course_code"
                      placeholder="เช่น CS101, SWE202, 001101"
                      value={courseCode}
                      maxLength={10}
                      onChange={(e) => handleCourseCodeChange(e.target.value)}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="section">กลุ่มเรียน (Section)</Label>
                    <Input
                      id="section"
                      placeholder="เช่น 001, Sec A"
                      value={section}
                      maxLength={50}
                      onChange={(e) => setSection(e.target.value)}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="name">ชื่อวิชา</Label>
                  <Input
                    id="name"
                    placeholder="เช่น Computer Programming"
                    value={courseName}
                    maxLength={255}
                    onChange={(e) => setCourseName(e.target.value)}
                    required
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="semester">ภาคเรียน (Semester)</Label>
                    <Input
                      id="semester"
                      placeholder="1, 2, 3"
                      value={semesterPart}
                      maxLength={20}
                      onChange={(e) => setSemesterPart(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="academic_year">ปีการศึกษา</Label>
                    <Input
                      id="academic_year"
                      placeholder="2569"
                      value={academicYear}
                      maxLength={20}
                      onChange={(e) => setAcademicYear(e.target.value)}
                      required
                    />
                  </div>
                </div>
              </div>

              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                  ยกเลิก
                </Button>
                <Button type="submit" disabled={submitting}>
                  {submitting ? "กำลังบันทึก..." : editingCourse ? "บันทึกการแก้ไข" : "ยืนยันสร้างวิชา"}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {/* Confirmation Dialog for Deleting a Course */}
      <Dialog open={!!courseToDelete} onOpenChange={(open) => !open && setCourseToDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="text-destructive flex items-center gap-2">
              <Trash2 className="size-5" /> ยืนยันการลบรายวิชา
            </DialogTitle>
            <DialogDescription>
              คุณต้องการลบรายวิชา{" "}
              <span className="font-semibold text-foreground">
                {courseToDelete?.courseCode ? `[${courseToDelete.courseCode}] ` : ""}
                {courseToDelete?.courseName || "ไม่มีชื่อวิชา"}
              </span>{" "}
              {courseToDelete?.section ? `(กลุ่ม ${courseToDelete.section})` : ""} ใช่หรือไม่?
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 text-xs text-muted-foreground space-y-1 bg-muted/50 p-3 rounded-lg border">
            <p>• การลบรายวิชาไม่สามารถเรียกคืนข้อมูลได้</p>
            <p>• ระบบอนุญาตให้ลบได้เฉพาะรายวิชาที่ยังไม่มีประวัติการเปิดคาบเรียนหรือประวัติการสุ่มเท่านั้น</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCourseToDelete(null)} disabled={deleting}>
              ยกเลิก
            </Button>
            <Button type="button" variant="destructive" onClick={handleConfirmDeleteCourse} disabled={deleting}>
              {deleting ? "กำลังลบ..." : "ยืนยันการลบรายวิชา"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Search Bar & Summary */}
      {courses.length > 0 && (
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="relative max-w-sm w-full">
            <Search className="size-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="ค้นหารหัสวิชา, ชื่อวิชา, กลุ่มเรียน..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>
          <div className="text-xs text-muted-foreground">
            แสดง {filteredCourses.length} จากทั้งหมด {courses.length} รายวิชา
          </div>
        </div>
      )}

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
          <AlertTitle>เกิดข้อผิดพลาดในการโหลดข้อมูล</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-12 text-muted-foreground">กำลังโหลดรายวิชา...</div>
      ) : filteredCourses.length === 0 ? (
        <Card className="text-center py-12 border-dashed">
          <CardContent className="space-y-3">
            <BookOpen className="size-12 mx-auto text-muted-foreground opacity-50" />
            <h3 className="text-lg font-semibold">
              {courses.length === 0 ? "ยังไม่มีรายวิชาในระบบ" : "ไม่พบรายวิชาที่ค้นหา"}
            </h3>
            <p className="text-sm text-muted-foreground max-w-sm mx-auto">
              {courses.length === 0
                ? 'เริ่มต้นโดยการคลิกปุ่ม "สร้างรายวิชาใหม่" ด้านบนเพื่อเริ่มบริหารจัดการการเรียนการสอน'
                : "ลองเปลี่ยนคำค้นหาใหม่อีกครั้ง"}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredCourses.map((course) => (
            <Card key={course.id} className="flex flex-col justify-between hover:shadow-md transition-shadow">
              <CardHeader>
                <div className="flex justify-between items-start gap-2">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="inline-block px-2.5 py-0.5 rounded-md bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 text-xs font-mono font-bold border border-indigo-200 dark:border-indigo-800">
                      {course.courseCode || "NO-CODE"}
                    </span>
                    {course.section && (
                      <Badge variant="outline" className="text-xs font-normal">
                        Sec {course.section}
                      </Badge>
                    )}
                    {course.activeSessionId && (
                      <Badge className="bg-emerald-600 hover:bg-emerald-600 text-white gap-1 text-[11px]">
                        <Radio className="size-3 animate-pulse" /> เปิดคาบอยู่
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-muted-foreground hover:text-foreground size-8"
                      onClick={() => openEditDialog(course)}
                      title="แก้ไขรายวิชา"
                    >
                      <Pencil className="size-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-muted-foreground hover:text-destructive size-8"
                      onClick={() => setCourseToDelete(course)}
                      title="ลบรายวิชา"
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
                <CardTitle className="line-clamp-1 text-xl pt-1">{course.courseName || "ไม่ระบุชื่อวิชา"}</CardTitle>
                <CardDescription>
                  ภาคเรียน {course.semester || "-"} • นักศึกษา {course.enrolledCount} คน • {course.totalSessions} คาบ
                </CardDescription>
              </CardHeader>

              <CardContent className="space-y-2">
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <Button render={<Link href={`/courses/${course.id}/students`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <Users className="size-3.5 text-indigo-600" /> นักศึกษา ({course.enrolledCount})
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/attendance`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <ClipboardCheck className="size-3.5 text-emerald-600" /> เช็กชื่อเข้าเรียน
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/history`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <History className="size-3.5 text-amber-600" /> ประวัติการสุ่ม
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/stats`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <BarChart3 className="size-3.5 text-violet-600" /> สถิติรายวิชา
                  </Button>
                </div>
              </CardContent>

              <CardFooter className="pt-0">
                <Button render={<Link href={`/courses/${course.id}/pick`} />} className="w-full gap-2 bg-indigo-600 hover:bg-indigo-700 text-white">
                  <Shuffle className="size-4" /> เข้าสู่หน้าระบบสุ่มวงล้อ (Fair Pick)
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
