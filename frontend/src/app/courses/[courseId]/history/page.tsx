"use client";

import { useEffect, useState, use } from "react";
import { getCourse, listCoursePicks } from "@/lib/api";
import { CourseDetail, PickHistoryItem } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { History, AlertCircle } from "lucide-react";

export default function CourseHistoryPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [history, setHistory] = useState<PickHistoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchHistory = async () => {
    try {
      setLoading(true);
      setError(null);
      const [cRes, hRes] = await Promise.all([
        getCourse(courseId),
        listCoursePicks(courseId, { limit: 50 }),
      ]);
      setCourse(cRes.data);
      setHistory(hRes.data || []);
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถดึงประวัติการสุ่มได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHistory();
  }, [courseId]);

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div>
        <h1 className="text-2xl font-bold tracking-tight">ประวัติการสุ่มทั้งหมด</h1>
        <p className="text-muted-foreground text-sm">
          บันทึกการสุ่มในทุกคาบเรียนของวิชานี้ ทั้งผลที่สำเร็จและรายการที่ถูกยกเลิก (Cancelled)
        </p>
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
            <div className="text-center py-12 text-muted-foreground">กำลังโหลดประวัติ...</div>
          ) : history.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground space-y-2">
              <History className="size-10 mx-auto opacity-40" />
              <p>ยังไม่มีประวัติการสุ่มในวิชานี้</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[180px]">วัน-เวลา</TableHead>
                  <TableHead className="w-[100px]">คาบที่</TableHead>
                  <TableHead className="w-[100px]">รอบที่</TableHead>
                  <TableHead className="w-[140px]">รหัสนักศึกษา</TableHead>
                  <TableHead>ชื่อ - นามสกุล</TableHead>
                  <TableHead className="w-[120px]">โหมด</TableHead>
                  <TableHead className="w-[100px] text-right">สถานะ</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {history.map((item) => (
                  <TableRow key={item.id} className={item.status === "cancelled" ? "opacity-50 line-through" : ""}>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(item.pickedAt).toLocaleString("th-TH")}
                    </TableCell>
                    <TableCell>#{item.sessionNumber}</TableCell>
                    <TableCell>#{item.roundNumber}</TableCell>
                    <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                    <TableCell>{item.fullName}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs capitalize">
                        {item.mode}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      {item.status === "cancelled" ? (
                        <Badge variant="destructive" className="text-xs">
                          ยกเลิกแล้ว
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-xs text-emerald-600 border-emerald-300 bg-emerald-50 dark:bg-emerald-950/20">
                          สำเร็จ
                        </Badge>
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
