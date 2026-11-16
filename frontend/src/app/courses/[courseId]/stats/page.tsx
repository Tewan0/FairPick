"use client";

import { useEffect, useState, use } from "react";
import { getCourse, getCourseStatsOverview, listCourseStudentStats } from "@/lib/api";
import { CourseDetail, CourseStatsOverview, StudentCourseStatItem } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { BarChart3, Users, Shuffle, Award, AlertCircle } from "lucide-react";

export default function CourseStatsPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [overview, setOverview] = useState<CourseStatsOverview | null>(null);
  const [studentStats, setStudentStats] = useState<StudentCourseStatItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = async () => {
    try {
      setLoading(true);
      setError(null);
      const [cRes, oRes, sRes] = await Promise.all([
        getCourse(courseId),
        getCourseStatsOverview(courseId),
        listCourseStudentStats(courseId),
      ]);
      setCourse(cRes.data);
      setOverview(oRes.data);
      setStudentStats(sRes.data || []);
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถดึงข้อมูลสถิติได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
  }, [courseId]);

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div>
        <h1 className="text-2xl font-bold tracking-tight">สถิติภาพรวมและการตอบคำถาม</h1>
        <p className="text-muted-foreground text-sm">
          วิเคราะห์ความถี่ในการถูกสุ่มของนักศึกษาแต่ละคน และความยุติธรรมของการกระจายคำถาม
        </p>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>เกิดข้อผิดพลาด</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-12 text-muted-foreground">กำลังประมวลผลสถิติ...</div>
      ) : !overview ? (
        <Card className="text-center py-12 border-dashed">
          <CardContent className="space-y-2">
            <BarChart3 className="size-10 mx-auto opacity-40" />
            <p className="text-muted-foreground">ยังไม่มีข้อมูลสถิติเพียงพอ</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-6">
          {/* Quick Metrics */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <Card>
              <CardHeader className="p-4 pb-2">
                <CardDescription className="text-xs">นักศึกษาทั้งหมด</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.totalActiveStudents}</span>
                  <Users className="size-5 text-muted-foreground" />
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="p-4 pb-2">
                <CardDescription className="text-xs">จำนวนครั้งที่สุ่มทั้งหมด</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.totalValidPicks}</span>
                  <Shuffle className="size-5 text-indigo-500" />
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="p-4 pb-2">
                <CardDescription className="text-xs">อัตราการเข้าเรียนเฉลี่ย</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.averageAttendanceRatePercent.toFixed(1)}%</span>
                  <Award className="size-5 text-emerald-500" />
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="p-4 pb-2">
                <CardDescription className="text-xs">คาบเรียนทั้งหมด</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.totalSessions}</span>
                  <BarChart3 className="size-5 text-amber-500" />
                </CardTitle>
              </CardHeader>
            </Card>
          </div>

          {/* Student Breakdown Table */}
          <Card>
            <CardHeader className="py-4">
              <CardTitle className="text-base">ความถี่ในการถูกสุ่มและการเข้าเรียนรายบุคคล</CardTitle>
              <CardDescription className="text-xs">
                ตรวจสอบว่าใครยังไม่เคยตอบ ({overview.overall.studentsNeverPickedCount} คน) หรือใครถูกสุ่มบ่อย
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[80px]">ลำดับ</TableHead>
                    <TableHead className="w-[160px]">รหัสนักศึกษา</TableHead>
                    <TableHead>ชื่อ - นามสกุล</TableHead>
                    <TableHead className="text-center w-[120px]">เข้าเรียน (%)</TableHead>
                    <TableHead className="w-[120px] text-center">ถูกสุ่ม (ครั้ง)</TableHead>
                    <TableHead className="w-[140px] text-right">สุ่มครั้งล่าสุด</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {studentStats.map((item, idx) => (
                    <TableRow key={item.studentId}>
                      <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                      <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                      <TableCell>{item.fullName}</TableCell>
                      <TableCell className="text-center font-medium">
                        {item.attendance.attendanceRatePercent.toFixed(0)}%
                      </TableCell>
                      <TableCell className="text-center font-bold text-indigo-600">
                        {item.picks.totalValidPicks}
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">
                        {item.picks.lastPickedAt
                          ? new Date(item.picks.lastPickedAt).toLocaleDateString("th-TH")
                          : "ยังไม่เคย"}
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
