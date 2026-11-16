"use client";

import { useEffect, useState, useMemo, use } from "react";
import { getCourse, getCourseStatsOverview, listCourseStudentStats } from "@/lib/api";
import { CourseDetail, CourseStatsOverview, StudentCourseStatItem } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { BarChart3, Users, Shuffle, Award, AlertCircle, Search } from "lucide-react";

export default function CourseStatsPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [overview, setOverview] = useState<CourseStatsOverview | null>(null);
  const [studentStats, setStudentStats] = useState<StudentCourseStatItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState<"studentCode" | "attendanceRate" | "validPickCount">("studentCode");

  const fetchStats = async (sortField = sortBy) => {
    if (!Number.isInteger(courseId) || courseId <= 0) {
      setError("รหัสรายวิชาไม่ถูกต้อง");
      setLoading(false);
      return;
    }
    try {
      setLoading(true);
      setError(null);
      const [cRes, oRes, sRes] = await Promise.all([
        getCourse(courseId),
        getCourseStatsOverview(courseId),
        listCourseStudentStats(courseId, {
          sortBy: sortField,
          sortOrder: sortField === "studentCode" ? "asc" : "desc",
        }),
      ]);
      setCourse(cRes.data);
      setOverview(oRes.data);
      setStudentStats(sRes.data || []);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถดึงข้อมูลสถิติได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats(sortBy);
  }, [courseId, sortBy]);

  const filteredStats = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return studentStats;
    return studentStats.filter(
      (s) =>
        s.studentCode.toLowerCase().includes(q) ||
        s.fullName.toLowerCase().includes(q)
    );
  }, [studentStats, searchQuery]);

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">สถิติภาพรวมและการตอบคำถาม</h1>
            {course && (
              <Badge variant="outline" className="text-xs font-mono">
                {course.courseCode || "NO-CODE"} • {course.courseName}
              </Badge>
            )}
          </div>
          <p className="text-muted-foreground text-sm">
            วิเคราะห์ความถี่ในการถูกสุ่มของนักศึกษาแต่ละคน และความยุติธรรมของการกระจายคำถาม
          </p>
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
                <CardDescription className="text-xs">นักศึกษาทั้งหมด (กำลังศึกษา)</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.totalActiveStudents} คน</span>
                  <Users className="size-5 text-muted-foreground" />
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="p-4 pb-2">
                <CardDescription className="text-xs">จำนวนครั้งที่สุ่มสำเร็จ</CardDescription>
                <CardTitle className="text-2xl flex items-center justify-between">
                  <span>{overview.overall.totalValidPicks} ครั้ง</span>
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
                  <span>{overview.overall.totalSessions} คาบ</span>
                  <BarChart3 className="size-5 text-amber-500" />
                </CardTitle>
              </CardHeader>
            </Card>
          </div>

          {/* Search & Sort Controls */}
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

            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">เรียงตาม:</span>
              <Select value={sortBy} onValueChange={(v) => setSortBy(v as typeof sortBy)}>
                <SelectTrigger className="w-[210px] h-9 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="studentCode">รหัสนักศึกษา (น้อย → มาก)</SelectItem>
                    <SelectItem value="validPickCount">จำนวนครั้งที่ถูกสุ่ม (มาก → น้อย)</SelectItem>
                    <SelectItem value="attendanceRate">อัตราการเข้าเรียน (มาก → น้อย)</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Student Breakdown Table */}
          <Card>
            <CardHeader className="py-4">
              <CardTitle className="text-base">ความถี่ในการถูกสุ่มและการเข้าเรียนรายบุคคล</CardTitle>
              <CardDescription className="text-xs">
                นักศึกษาที่ยังไม่เคยถูกสุ่มเลย: {overview.overall.studentsNeverPickedCount} คน • สุ่มแบบ Weighted: {overview.overall.weightedValidPicks} ครั้ง • สุ่มแบบ No Repeat: {overview.overall.noRepeatValidPicks} ครั้ง
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              {filteredStats.length === 0 ? (
                <div className="text-center py-10 text-sm text-muted-foreground">
                  ไม่พบข้อมูลสถิตินักศึกษา
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[70px]">ลำดับ</TableHead>
                      <TableHead className="w-[150px]">รหัสนักศึกษา</TableHead>
                      <TableHead>ชื่อ - นามสกุล</TableHead>
                      <TableHead className="text-center w-[180px]">อัตราการเข้าเรียน</TableHead>
                      <TableHead className="w-[130px] text-center">ถูกสุ่ม (ครั้ง)</TableHead>
                      <TableHead className="w-[140px] text-right">สุ่มครั้งล่าสุด</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredStats.map((item, idx) => (
                      <TableRow key={item.studentId}>
                        <TableCell className="text-muted-foreground">{idx + 1}</TableCell>
                        <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                        <TableCell>{item.fullName}</TableCell>
                        <TableCell className="text-center">
                          <div className="flex items-center justify-center gap-2">
                            <div className="w-20 h-2 bg-muted rounded-full overflow-hidden">
                              <div
                                className="h-full bg-emerald-500 rounded-full"
                                style={{ width: `${Math.min(100, item.attendance.attendanceRatePercent)}%` }}
                              />
                            </div>
                            <span className="text-xs font-medium w-10 text-right">
                              {item.attendance.attendanceRatePercent.toFixed(0)}%
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="text-center">
                          <Badge
                            variant={item.picks.totalValidPicks > 0 ? "secondary" : "outline"}
                            className="font-bold text-indigo-600 dark:text-indigo-400"
                          >
                            {item.picks.totalValidPicks} ครั้ง
                          </Badge>
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
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
