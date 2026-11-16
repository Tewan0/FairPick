"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { listCourses, createCourse, deleteCourse } from "@/lib/api";
import { Course } from "@/types/fairpick";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { BookOpen, Plus, Trash2, Users, Shuffle, ClipboardCheck, History, BarChart3, AlertCircle } from "lucide-react";

export default function CoursesPage() {
  const [courses, setCourses] = useState<Course[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Form states for new course
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [courseCode, setCourseCode] = useState("");
  const [courseName, setCourseName] = useState("");
  const [academicYear, setAcademicYear] = useState("2567");
  const [semester, setSemester] = useState("1");
  const [submitting, setSubmitting] = useState(false);

  const fetchCourses = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await listCourses();
      setCourses(res.data || []);
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถโหลดข้อมูลรายวิชาได้ (โปรดตรวจสอบว่า Backend และ Database พร้อมทำงาน)");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCourses();
  }, []);

  const handleCreateCourse = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!courseCode.trim() || !courseName.trim()) return;

    try {
      setSubmitting(true);
      await createCourse({
        courseCode: courseCode.trim(),
        courseName: courseName.trim(),
        semester: `${semester.trim()}/${academicYear.trim()}`,
      });
      setIsDialogOpen(false);
      setCourseCode("");
      setCourseName("");
      fetchCourses();
    } catch (err: any) {
      alert("สร้างรายวิชาไม่สำเร็จ: " + (err?.message || ""));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteCourse = async (id: number, name: string) => {
    if (!confirm(`คุณต้องการลบรายวิชา "${name}" ใช่หรือไม่?`)) return;
    try {
      await deleteCourse(id);
      fetchCourses();
    } catch (err: any) {
      alert("ลบรายวิชาไม่สำเร็จ: " + (err?.message || ""));
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
          <DialogTrigger render={<Button className="gap-2" />}>
            <Plus className="size-4" /> สร้างรายวิชาใหม่
          </DialogTrigger>
          <DialogContent>
            <form onSubmit={handleCreateCourse}>
              <DialogHeader>
                <DialogTitle>เพิ่มรายวิชาใหม่</DialogTitle>
                <DialogDescription>
                  กรอกรายละเอียดข้อมูลรหัสวิชา ชื่อวิชา ภาคเรียน และปีการศึกษา
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="course_code">รหัสวิชา</Label>
                  <Input
                    id="course_code"
                    placeholder="เช่น CS101, SWE202"
                    value={courseCode}
                    onChange={(e) => setCourseCode(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="name">ชื่อวิชา</Label>
                  <Input
                    id="name"
                    placeholder="เช่น Computer Programming"
                    value={courseName}
                    onChange={(e) => setCourseName(e.target.value)}
                    required
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="academic_year">ปีการศึกษา</Label>
                    <Input
                      id="academic_year"
                      placeholder="2567"
                      value={academicYear}
                      onChange={(e) => setAcademicYear(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="semester">ภาคเรียน (Semester)</Label>
                    <Input
                      id="semester"
                      placeholder="1, 2, 3"
                      value={semester}
                      onChange={(e) => setSemester(e.target.value)}
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
                  {submitting ? "กำลังบันทึก..." : "สร้างวิชา"}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>เกิดข้อผิดพลาดในการโหลดข้อมูล</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-12 text-muted-foreground">กำลังโหลดรายวิชา...</div>
      ) : courses.length === 0 ? (
        <Card className="text-center py-12 border-dashed">
          <CardContent className="space-y-3">
            <BookOpen className="size-12 mx-auto text-muted-foreground opacity-50" />
            <h3 className="text-lg font-semibold">ยังไม่มีรายวิชาในระบบ</h3>
            <p className="text-sm text-muted-foreground max-w-sm mx-auto">
              เริ่มต้นโดยการคลิกปุ่ม "สร้างรายวิชาใหม่" ด้านบนเพื่อเริ่มบริหารจัดการการเรียนการสอน
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {courses.map((course) => (
            <Card key={course.id} className="flex flex-col justify-between">
              <CardHeader>
                <div className="flex justify-between items-start gap-2">
                  <div className="inline-block px-2 py-0.5 rounded bg-muted text-xs font-semibold">
                    {course.courseCode || "NO-CODE"}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-destructive size-8"
                    onClick={() => handleDeleteCourse(course.id, course.courseName || "")}
                    title="ลบรายวิชา"
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
                <CardTitle className="line-clamp-1 text-xl">{course.courseName}</CardTitle>
                <CardDescription>
                  ภาคเรียน {course.semester || "-"} • นักศึกษา {course.enrolledCount} คน
                </CardDescription>
              </CardHeader>

              <CardContent className="space-y-2">
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <Button render={<Link href={`/courses/${course.id}/students`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <Users className="size-3.5" /> นักศึกษา
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/attendance`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <ClipboardCheck className="size-3.5" /> เช็กชื่อ
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/history`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <History className="size-3.5" /> ประวัติสุ่ม
                  </Button>
                  <Button render={<Link href={`/courses/${course.id}/stats`} />} variant="outline" size="sm" className="w-full justify-start gap-1.5">
                    <BarChart3 className="size-3.5" /> สถิติ
                  </Button>
                </div>
              </CardContent>

              <CardFooter className="pt-0">
                <Button render={<Link href={`/courses/${course.id}/pick`} />} className="w-full gap-2 bg-indigo-600 hover:bg-indigo-700">
                  <Shuffle className="size-4" /> เข้าสู่หน้าระบบสุ่ม (Pick)
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
