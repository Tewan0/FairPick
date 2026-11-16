"use client";

import { useEffect, useState, useMemo, use } from "react";
import { getCourse, listCoursePicks, cancelPick } from "@/lib/api";
import { CourseDetail, PickHistoryItem } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { History, AlertCircle, Search, Ban, CheckCircle2 } from "lucide-react";

export default function CourseHistoryPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [history, setHistory] = useState<PickHistoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "valid" | "cancelled">("all");

  // Confirmation Dialog for Cancel Pick from history
  const [pickToCancel, setPickToCancel] = useState<PickHistoryItem | null>(null);
  const [cancelReason, setCancelReason] = useState("ยกเลิกย้อนหลังโดยอาจารย์");
  const [cancelling, setCancelling] = useState(false);

  const fetchHistory = async () => {
    if (!Number.isInteger(courseId) || courseId <= 0) {
      setError("รหัสรายวิชาไม่ถูกต้อง");
      setLoading(false);
      return;
    }
    try {
      setLoading(true);
      setError(null);
      const [cRes, hRes] = await Promise.all([
        getCourse(courseId),
        listCoursePicks(courseId, { limit: 100 }),
      ]);
      setCourse(cRes.data);
      setHistory(hRes.data || []);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถดึงประวัติการสุ่มได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHistory();
  }, [courseId]);

  const filteredHistory = useMemo(() => {
    return history.filter((item) => {
      if (statusFilter !== "all" && item.status !== statusFilter) return false;
      const q = searchQuery.trim().toLowerCase();
      if (!q) return true;
      return (
        item.studentCode.toLowerCase().includes(q) ||
        item.fullName.toLowerCase().includes(q)
      );
    });
  }, [history, statusFilter, searchQuery]);

  const handleConfirmCancelPick = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pickToCancel) return;
    try {
      setCancelling(true);
      await cancelPick(pickToCancel.id, {
        cancelReason: cancelReason.trim() || "ยกเลิกย้อนหลังโดยอาจารย์",
      });
      setFeedback({
        type: "success",
        message: `ยกเลิกผลการสุ่มของ "${pickToCancel.fullName}" เรียบร้อยแล้ว`,
      });
      setPickToCancel(null);
      await fetchHistory();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "ยกเลิกผลการสุ่มไม่สำเร็จ: " + msg });
      setPickToCancel(null);
    } finally {
      setCancelling(false);
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">ประวัติการสุ่มทั้งหมด</h1>
            {course && (
              <Badge variant="outline" className="text-xs font-mono">
                {course.courseCode || "NO-CODE"} • {course.courseName}
              </Badge>
            )}
          </div>
          <p className="text-muted-foreground text-sm">
            บันทึกการสุ่มในทุกคาบเรียนของวิชานี้ ทั้งผลที่สำเร็จและรายการที่ถูกยกเลิก (Cancelled)
          </p>
        </div>
      </div>

      {/* Confirmation Dialog for Cancel Pick */}
      <Dialog open={!!pickToCancel} onOpenChange={(open) => !open && setPickToCancel(null)}>
        <DialogContent>
          <form onSubmit={handleConfirmCancelPick}>
            <DialogHeader>
              <DialogTitle className="text-rose-600 flex items-center gap-2">
                <Ban className="size-5" /> ยืนยันการยกเลิกรายการสุ่ม
              </DialogTitle>
              <DialogDescription>
                คุณต้องการยกเลิกรายการสุ่มของ{" "}
                <span className="font-semibold text-foreground">
                  {pickToCancel?.studentCode} — {pickToCancel?.fullName}
                </span>{" "}
                (คาบที่ #{pickToCancel?.sessionNumber}) ใช่หรือไม่?
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3 py-4">
              <div className="space-y-2">
                <Label htmlFor="hist_cancel_reason">เหตุผลที่ยกเลิก</Label>
                <Input
                  id="hist_cancel_reason"
                  value={cancelReason}
                  maxLength={255}
                  onChange={(e) => setCancelReason(e.target.value)}
                  required
                />
              </div>
              <p className="text-xs text-muted-foreground">
                หมายเหตุ: ยกเลิกได้เฉพาะรายการสุ่มที่อยู่ในคาบเรียนที่กำลังเปิดอยู่ (Active Session) เท่านั้น
              </p>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setPickToCancel(null)} disabled={cancelling}>
                ปิด
              </Button>
              <Button type="submit" variant="destructive" disabled={cancelling}>
                {cancelling ? "กำลังยกเลิก..." : "ยืนยันยกเลิกผลสุ่ม"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Search & Status Filter */}
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
              { key: "all", label: `ทั้งหมด (${history.length})` },
              { key: "valid", label: `สำเร็จ (${history.filter((h) => h.status === "valid").length})` },
              { key: "cancelled", label: `ยกเลิกแล้ว (${history.filter((h) => h.status === "cancelled").length})` },
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
            <div className="text-center py-12 text-muted-foreground">กำลังโหลดประวัติ...</div>
          ) : filteredHistory.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground space-y-2">
              <History className="size-10 mx-auto opacity-40" />
              <p>ไม่พบประวัติการสุ่มตามเงื่อนไขที่เลือก</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[170px]">วัน-เวลา</TableHead>
                  <TableHead className="w-[90px]">คาบที่</TableHead>
                  <TableHead className="w-[80px]">รอบที่</TableHead>
                  <TableHead className="w-[140px]">รหัสนักศึกษา</TableHead>
                  <TableHead>ชื่อ - นามสกุล</TableHead>
                  <TableHead className="w-[120px]">โหมด</TableHead>
                  <TableHead className="w-[120px] text-center">สถานะ</TableHead>
                  <TableHead className="w-[100px] text-right">จัดการ</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredHistory.map((item) => {
                  const canCancel =
                    item.status === "valid" &&
                    course?.activeSession?.id === item.sessionId;
                  return (
                    <TableRow key={item.id} className={item.status === "cancelled" ? "opacity-60" : ""}>
                      <TableCell className="text-xs text-muted-foreground">
                        {new Date(item.pickedAt).toLocaleString("th-TH")}
                      </TableCell>
                      <TableCell>#{item.sessionNumber}</TableCell>
                      <TableCell>#{item.roundNumber}</TableCell>
                      <TableCell className="font-mono font-medium">{item.studentCode}</TableCell>
                      <TableCell>
                        <div className={item.status === "cancelled" ? "line-through" : ""}>{item.fullName}</div>
                        {item.cancelReason && (
                          <div className="text-[11px] text-rose-500">เหตุผล: {item.cancelReason}</div>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-xs">
                          {item.mode === "weighted" ? "Weighted" : "No Repeat"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        {item.status === "cancelled" ? (
                          <Badge variant="destructive" className="text-xs">
                            ยกเลิกแล้ว
                          </Badge>
                        ) : (
                          <Badge
                            variant="outline"
                            className="text-xs text-emerald-600 border-emerald-300 bg-emerald-50 dark:bg-emerald-950/20"
                          >
                            สำเร็จ
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        {canCancel && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 px-2 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50 gap-1"
                            onClick={() => {
                              setCancelReason("ยกเลิกย้อนหลังโดยอาจารย์");
                              setPickToCancel(item);
                            }}
                          >
                            <Ban className="size-3.5" /> ยกเลิก
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
