"use client";

import { useEffect, useState, use } from "react";
import {
  getCourse,
  listCourseSessions,
  startClassSession,
  getSessionPickPool,
  executeRandomPick,
  cancelPick,
  startNewPickRound,
  switchSessionPickMode,
} from "@/lib/api";
import { CourseDetail, ClassSession, PickPoolData, PickMode, ExecutePickResult } from "@/types/fairpick";
import { CourseNav } from "@/components/CourseNav";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Shuffle, Sparkles, RotateCcw, Ban, AlertCircle, Play } from "lucide-react";

export default function CoursePickPage({ params }: { params: Promise<{ courseId: string }> }) {
  const resolvedParams = use(params);
  const courseId = Number(resolvedParams.courseId);

  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [activeSession, setActiveSession] = useState<ClassSession | null>(null);
  const [pool, setPool] = useState<PickPoolData | null>(null);
  const [loading, setLoading] = useState(true);
  const [picking, setPicking] = useState(false);
  const [lastPickResult, setLastPickResult] = useState<ExecutePickResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchSessionAndPool = async () => {
    try {
      setLoading(true);
      setError(null);
      const [cRes, sRes] = await Promise.all([
        getCourse(courseId),
        listCourseSessions(courseId),
      ]);
      setCourse(cRes.data);
      const active = (sRes.data || []).find((s) => s.status === "active");

      if (active) {
        setActiveSession(active);
        const poolRes = await getSessionPickPool(active.id);
        setPool(poolRes.data);
      } else {
        setActiveSession(null);
        setPool(null);
      }
    } catch (err: any) {
      setError(err?.message || "ไม่สามารถโหลดข้อมูลสระการสุ่มได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSessionAndPool();
  }, [courseId]);

  const handleStartSession = async () => {
    try {
      setLoading(true);
      await startClassSession(courseId);
      await fetchSessionAndPool();
    } catch (err: any) {
      alert("เริ่มคาบเรียนไม่สำเร็จ: " + (err?.message || ""));
      setLoading(false);
    }
  };

  const handleModeChange = async (mode: PickMode) => {
    if (!activeSession) return;
    try {
      await switchSessionPickMode(activeSession.id, { mode });
      await fetchSessionAndPool();
    } catch (err: any) {
      alert("เปลี่ยนโหมดการสุ่มไม่สำเร็จ: " + (err?.message || ""));
    }
  };

  const handleExecutePick = async () => {
    if (!activeSession) return;
    try {
      setPicking(true);
      await new Promise((r) => setTimeout(r, 500));
      const res = await executeRandomPick(activeSession.id, {
        mode: activeSession.currentMode,
      });
      setLastPickResult(res.data);
      // Refresh pool
      const poolRes = await getSessionPickPool(activeSession.id);
      setPool(poolRes.data);
    } catch (err: any) {
      alert("การสุ่มล้มเหลว: " + (err?.message || ""));
    } finally {
      setPicking(false);
    }
  };

  const handleNewRound = async () => {
    if (!activeSession) return;
    try {
      await startNewPickRound(activeSession.id);
      const poolRes = await getSessionPickPool(activeSession.id);
      setPool(poolRes.data);
      setLastPickResult(null);
    } catch (err: any) {
      alert("เริ่มรอบใหม่ไม่สำเร็จ: " + (err?.message || ""));
    }
  };

  const handleCancelPick = async () => {
    if (!lastPickResult || !lastPickResult.pick.id) return;
    if (!confirm("คุณต้องการยกเลิกผลการสุ่มล่าสุดนี้หรือไม่?")) return;
    try {
      await cancelPick(lastPickResult.pick.id, { cancelReason: "สุ่มใหม่โดยอาจารย์" });
      setLastPickResult(null);
      if (activeSession) {
        const poolRes = await getSessionPickPool(activeSession.id);
        setPool(poolRes.data);
      }
    } catch (err: any) {
      alert("ยกเลิกผลสุ่มไม่สำเร็จ: " + (err?.message || ""));
    }
  };

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">ระบบสุ่มตอบคำถาม (Fair Pick)</h1>
          <p className="text-muted-foreground text-sm">
            สุ่มนักศึกษาในคาบเรียนแบบยุติธรรม ไม่ซ้ำคนเดิมจนกว่าจะครบทุกคนในรอบ (Fair / Weighted)
          </p>
        </div>

        {activeSession && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">โหมด:</span>
            <Select
              value={activeSession.currentMode || "weighted"}
              onValueChange={(val) => handleModeChange(val as PickMode)}
            >
              <SelectTrigger className="w-[160px] h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="weighted">Weighted (ถ่วงน้ำหนัก W)</SelectItem>
                  <SelectItem value="no_repeat">No Repeat (ไม่ซ้ำในรอบ)</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>เกิดข้อผิดพลาด</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-16 text-muted-foreground">กำลังโหลดระบบสุ่ม...</div>
      ) : !activeSession ? (
        <Card className="text-center py-16 border-dashed">
          <CardContent className="space-y-4">
            <Shuffle className="size-16 mx-auto text-muted-foreground opacity-40 animate-pulse" />
            <h3 className="text-xl font-semibold">ยังไม่มีคาบเรียนเปิดอยู่</h3>
            <p className="text-sm text-muted-foreground max-w-md mx-auto">
              ระบบสุ่ม Fair Pick ต้องทำงานร่วมกับคาบเรียนที่เปิดใช้งาน เพื่อตรวจสอบสถานะการเข้าเรียนและนับรอบการตอบ
            </p>
            <Button onClick={handleStartSession} className="gap-2 bg-emerald-600 hover:bg-emerald-700">
              <Play className="size-4 fill-current" /> เริ่มคาบเรียนเพื่อเริ่มสุ่ม
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-6">
          {/* Main Picker Card */}
          <Card className="border-2 border-indigo-100 dark:border-indigo-950 shadow-md">
            <CardHeader className="text-center pb-2">
              <Badge variant="outline" className="w-fit mx-auto mb-1">
                คาบที่ #{activeSession.sessionNumber} • รอบที่ #{pool?.currentRound || 1} • โหมด {activeSession.currentMode}
              </Badge>
              <CardTitle className="text-lg">ผู้โชคดีตอบคำถาม</CardTitle>
            </CardHeader>

            <CardContent className="flex flex-col items-center justify-center py-8">
              {lastPickResult ? (
                <div className="text-center space-y-3 animate-in fade-in zoom-in duration-300">
                  <div className="inline-flex p-3 rounded-full bg-indigo-50 dark:bg-indigo-950 text-indigo-600 mb-2">
                    <Sparkles className="size-8" />
                  </div>
                  <div className="text-3xl sm:text-4xl font-extrabold text-foreground">
                    {lastPickResult.pick.student.fullName}
                  </div>
                  <div className="text-lg font-mono text-muted-foreground">
                    {lastPickResult.pick.student.studentCode}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    สถานะการเข้าเรียน: {lastPickResult.pick.student.attendanceStatus}
                  </div>
                </div>
              ) : (
                <div className="text-center py-6 text-muted-foreground">
                  <Shuffle className="size-12 mx-auto mb-3 opacity-30" />
                  <p className="text-base font-medium">กดปุ่มด้านล่างเพื่อเริ่มการสุ่ม</p>
                  <p className="text-xs">
                    ผู้มีสิทธิ์ถูกสุ่ม {pool?.totalEligible || 0} คน
                    {activeSession.currentMode === "no_repeat" && ` (เหลือในรอบนี้ ${pool?.remainingInNoRepeatRound || 0} คน)`}
                  </p>
                </div>
              )}
            </CardContent>

            <CardFooter className="flex flex-col sm:flex-row gap-3 justify-center pt-2 pb-6">
              {pool?.isNoRepeatRoundCompleted ? (
                <Button
                  size="lg"
                  onClick={handleNewRound}
                  className="w-full sm:w-auto px-8 gap-2 bg-emerald-600 hover:bg-emerald-700 text-white font-semibold"
                >
                  <RotateCcw className="size-4" /> เริ่มรอบใหม่ (Start New Round)
                </Button>
              ) : (
                <Button
                  size="lg"
                  disabled={picking || (pool?.totalEligible || 0) === 0}
                  onClick={handleExecutePick}
                  className="w-full sm:w-auto px-8 gap-2 bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-base shadow-lg"
                >
                  <Shuffle className={`size-5 ${picking ? "animate-spin" : ""}`} />
                  {picking ? "กำลังสุ่ม..." : "สุ่มตอบคำถาม"}
                </Button>
              )}

              {lastPickResult && (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={handleCancelPick}
                  className="w-full sm:w-auto gap-2 text-rose-600 hover:text-rose-700 hover:bg-rose-50"
                >
                  <Ban className="size-4" /> ยกเลิกผลการสุ่มนี้
                </Button>
              )}
            </CardFooter>
          </Card>

          {/* Candidates in pool */}
          <Card>
            <CardHeader className="py-4">
              <div className="flex justify-between items-center">
                <CardTitle className="text-base">
                  รายชื่อในสระการสุ่ม ({pool?.pool?.length || 0} คน)
                </CardTitle>
                <Badge variant="secondary">รอบที่ #{pool?.currentRound || 1}</Badge>
              </div>
            </CardHeader>
            <CardContent>
              {pool?.pool && pool.pool.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {pool.pool.map((candidate) => (
                    <Badge
                      key={candidate.studentId}
                      variant={candidate.pickedInCurrentNoRepeatRound ? "outline" : "secondary"}
                      className={`px-2.5 py-1 text-xs font-normal ${
                        candidate.pickedInCurrentNoRepeatRound ? "opacity-40 line-through" : ""
                      }`}
                    >
                      <span className="font-mono mr-1.5 opacity-70">{candidate.studentCode}</span>
                      <span>{candidate.fullName}</span>
                      {activeSession.currentMode === "weighted" && (
                        <span className="ml-1.5 font-bold text-indigo-600">
                          ({candidate.probabilityPercent.toFixed(1)}%)
                        </span>
                      )}
                    </Badge>
                  ))}
                </div>
              ) : (
                <div className="text-center py-6 text-sm text-muted-foreground">
                  ไม่มีนักศึกษาในสระสุ่ม (เช็กชื่อนักศึกษาเป็น "มา" หรือ "สาย" ก่อน)
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
