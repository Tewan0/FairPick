"use client";

import { useEffect, useState, useRef, use } from "react";
import Link from "next/link";
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
import { SpinningWheel } from "@/components/SpinningWheel";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Shuffle,
  Sparkles,
  RotateCcw,
  Ban,
  AlertCircle,
  Play,
  ClipboardCheck,
  CheckCircle2,
  Disc,
  LayoutGrid,
} from "lucide-react";

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
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Wheel animation states
  const [displayStyle, setDisplayStyle] = useState<"wheel" | "card">("wheel");
  const [wheelTargetId, setWheelTargetId] = useState<number | null>(null);
  const [spinKey, setSpinKey] = useState(0);
  const pendingResultRef = useRef<{ pickRes: ExecutePickResult; nextPool: PickPoolData } | null>(null);

  // Confirmation Dialog 1: Start Session
  const [startSessionDialogOpen, setStartSessionDialogOpen] = useState(false);
  const [newSessionTopic, setNewSessionTopic] = useState("");
  const [newSessionMode, setNewSessionMode] = useState<PickMode>("weighted");
  const [startingSession, setStartingSession] = useState(false);

  // Confirmation Dialog 2: Switch Mode
  const [pendingModeSwitch, setPendingModeSwitch] = useState<PickMode | null>(null);
  const [switchingMode, setSwitchingMode] = useState(false);

  // Confirmation Dialog 3: Start New Round
  const [newRoundDialogOpen, setNewRoundDialogOpen] = useState(false);
  const [startingNewRound, setStartingNewRound] = useState(false);

  // Confirmation Dialog 4: Cancel Pick
  const [cancelDialogOpen, setCancelDialogOpen] = useState(false);
  const [cancelReason, setCancelReason] = useState("สุ่มใหม่โดยอาจารย์");
  const [markAbsentOnCancel, setMarkAbsentOnCancel] = useState(false);
  const [cancellingPick, setCancellingPick] = useState(false);

  const fetchSessionAndPool = async () => {
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
      const active = (sRes.data || []).find((s) => s.status === "active");

      if (active) {
        setActiveSession(active);
        const poolRes = await getSessionPickPool(active.id);
        setPool(poolRes.data);
      } else {
        setActiveSession(null);
        setPool(null);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(msg || "ไม่สามารถโหลดข้อมูลสระการสุ่มได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSessionAndPool();
  }, [courseId]);

  const handleConfirmStartSession = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setStartingSession(true);
      await startClassSession(courseId, {
        topic: newSessionTopic.trim() || null,
        initialMode: newSessionMode,
      });
      setStartSessionDialogOpen(false);
      setNewSessionTopic("");
      setFeedback({ type: "success", message: "เริ่มคาบเรียนใหม่เรียบร้อยแล้ว สามารถเช็กชื่อและเริ่มสุ่มได้ทันที" });
      await fetchSessionAndPool();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เริ่มคาบเรียนไม่สำเร็จ: " + msg });
    } finally {
      setStartingSession(false);
    }
  };

  const handleRequestModeChange = (mode: PickMode) => {
    if (!activeSession || mode === activeSession.currentMode) return;
    setPendingModeSwitch(mode);
  };

  const handleConfirmModeChange = async () => {
    if (!activeSession || !pendingModeSwitch) return;
    try {
      setSwitchingMode(true);
      await switchSessionPickMode(activeSession.id, { mode: pendingModeSwitch });
      setFeedback({
        type: "success",
        message: `เปลี่ยนโหมดการสุ่มเป็น ${pendingModeSwitch === "weighted" ? "Weighted (ถ่วงน้ำหนัก)" : "No Repeat (ไม่ซ้ำในรอบ)"} สำเร็จ`,
      });
      setPendingModeSwitch(null);
      await fetchSessionAndPool();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เปลี่ยนโหมดการสุ่มไม่สำเร็จ: " + msg });
      setPendingModeSwitch(null);
    } finally {
      setSwitchingMode(false);
    }
  };

  const handleExecutePick = async () => {
    if (!activeSession || picking) return;
    try {
      setPicking(true);
      setFeedback(null);

      const res = await executeRandomPick(activeSession.id, {
        mode: activeSession.currentMode,
      });
      const poolRes = await getSessionPickPool(activeSession.id);

      if (displayStyle === "wheel") {
        // Let the wheel spin to the chosen student first before revealing result & updating pool
        pendingResultRef.current = { pickRes: res.data, nextPool: poolRes.data };
        setWheelTargetId(res.data.pick.student.id);
        setSpinKey((prev) => prev + 1);
      } else {
        await new Promise((r) => setTimeout(r, 450));
        setLastPickResult(res.data);
        setPool(poolRes.data);
        setPicking(false);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "การสุ่มล้มเหลว: " + msg });
      setPicking(false);
    }
  };

  const handleWheelSpinComplete = () => {
    if (pendingResultRef.current) {
      setLastPickResult(pendingResultRef.current.pickRes);
      setPool(pendingResultRef.current.nextPool);
      pendingResultRef.current = null;
    }
    setPicking(false);
  };

  const handleConfirmNewRound = async () => {
    if (!activeSession) return;
    try {
      setStartingNewRound(true);
      const res = await startNewPickRound(activeSession.id);
      const poolRes = await getSessionPickPool(activeSession.id);
      setPool(poolRes.data);
      setLastPickResult(null);
      setWheelTargetId(null);
      setNewRoundDialogOpen(false);
      setFeedback({
        type: "success",
        message: `เริ่มรอบการสุ่มใหม่ (รอบที่ #${res.data.currentRound}) สำเร็จ นักศึกษาทุกคนที่เข้าเรียนกลับมามีสิทธิ์ถูกสุ่มครบอีกครั้ง`,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "เริ่มรอบใหม่ไม่สำเร็จ: " + msg });
      setNewRoundDialogOpen(false);
    } finally {
      setStartingNewRound(false);
    }
  };

  const openCancelPickDialog = () => {
    setCancelReason("สุ่มใหม่โดยอาจารย์");
    setMarkAbsentOnCancel(false);
    setCancelDialogOpen(true);
  };

  const handleConfirmCancelPick = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!lastPickResult || !lastPickResult.pick.id) return;
    try {
      setCancellingPick(true);
      await cancelPick(lastPickResult.pick.id, {
        cancelReason: cancelReason.trim() || "สุ่มใหม่โดยอาจารย์",
        markStudentAbsent: markAbsentOnCancel,
      });
      const cancelledName = lastPickResult.pick.student.fullName;
      setLastPickResult(null);
      setWheelTargetId(null);
      setCancelDialogOpen(false);
      if (activeSession) {
        const poolRes = await getSessionPickPool(activeSession.id);
        setPool(poolRes.data);
      }
      setFeedback({
        type: "success",
        message: `ยกเลิกผลการสุ่มของ "${cancelledName}" เรียบร้อยแล้ว (ไม่ถูกนับเป็นจำนวนครั้งที่ตอบ)`,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setFeedback({ type: "error", message: "ยกเลิกผลสุ่มไม่สำเร็จ: " + msg });
      setCancelDialogOpen(false);
    } finally {
      setCancellingPick(false);
    }
  };

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <CourseNav courseId={courseId} />

      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">ระบบสุ่มตอบคำถาม (Fair Pick)</h1>
            {course && (
              <Badge variant="outline" className="text-xs font-mono">
                {course.courseCode || "NO-CODE"} • {course.courseName}
              </Badge>
            )}
          </div>
          <p className="text-muted-foreground text-sm">
            สุ่มนักศึกษาในคาบเรียนด้วยวงล้อแบบยุติธรรม กระจายโอกาสตอบคำถามอย่างทั่วถึง
          </p>
        </div>

        {activeSession && (
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center rounded-lg border p-0.5 bg-muted/40">
              <Button
                type="button"
                variant={displayStyle === "wheel" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2.5 text-xs gap-1"
                onClick={() => setDisplayStyle("wheel")}
                disabled={picking}
              >
                <Disc className="size-3.5" /> วงล้อหมุน
              </Button>
              <Button
                type="button"
                variant={displayStyle === "card" ? "default" : "ghost"}
                size="sm"
                className="h-7 px-2.5 text-xs gap-1"
                onClick={() => setDisplayStyle("card")}
                disabled={picking}
              >
                <LayoutGrid className="size-3.5" /> การ์ดด่วน
              </Button>
            </div>

            <div className="flex items-center gap-1.5">
              <span className="text-xs text-muted-foreground">โหมด:</span>
              <Select
                value={activeSession.currentMode || "weighted"}
                onValueChange={(val) => handleRequestModeChange(val as PickMode)}
                disabled={picking}
              >
                <SelectTrigger className="w-[180px] h-8 text-xs">
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
          </div>
        )}
      </div>

      {/* Confirmation Dialog 1: Start Session */}
      <Dialog open={startSessionDialogOpen} onOpenChange={setStartSessionDialogOpen}>
        <DialogContent>
          <form onSubmit={handleConfirmStartSession}>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Play className="size-4 text-emerald-600 fill-current" /> ยืนยันการเริ่มคาบเรียนใหม่
              </DialogTitle>
              <DialogDescription>
                ระบบจะเปิดคาบเรียนใหม่และสร้างใบเช็กชื่อเริ่มต้นสำหรับนักศึกษาที่ลงทะเบียนทั้งหมด
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="session_topic">หัวข้อการสอนประจำคาบ (ไม่บังคับ)</Label>
                <Input
                  id="session_topic"
                  placeholder="เช่น สัปดาห์ที่ 1: Introduction & Overview"
                  value={newSessionTopic}
                  maxLength={255}
                  onChange={(e) => setNewSessionTopic(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label>โหมดการสุ่มเริ่มต้น</Label>
                <Select value={newSessionMode} onValueChange={(v) => setNewSessionMode(v as PickMode)}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="weighted">Weighted (ถ่วงน้ำหนักตามประวัติ 4 คาบล่าสุด)</SelectItem>
                      <SelectItem value="no_repeat">No Repeat (สุ่มไม่ซ้ำคนในรอบเดียวกัน)</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setStartSessionDialogOpen(false)} disabled={startingSession}>
                ยกเลิก
              </Button>
              <Button type="submit" className="bg-emerald-600 hover:bg-emerald-700 text-white" disabled={startingSession}>
                {startingSession ? "กำลังเปิดคาบเรียน..." : "ยืนยันเริ่มคาบเรียน"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog 2: Switch Mode */}
      <Dialog open={!!pendingModeSwitch} onOpenChange={(open) => !open && setPendingModeSwitch(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>ยืนยันการเปลี่ยนโหมดการสุ่ม</DialogTitle>
            <DialogDescription>
              คุณต้องการสลับโหมดการสุ่มประจำคาบเรียนเป็น{" "}
              <span className="font-semibold text-foreground">
                {pendingModeSwitch === "weighted" ? "Weighted (ถ่วงน้ำหนัก W = 1 / (1 + H + 2C))" : "No Repeat (สุ่มไม่ซ้ำคนจนครบทุกคนในรอบ)"}
              </span>{" "}
              ใช่หรือไม่?
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 text-xs text-muted-foreground bg-muted/50 p-3 rounded-lg border space-y-1">
            <p>• ประวัติการสุ่มในคาบนี้และรอบการสุ่มปัจจุบันจะยังคงถูกเก็บรักษาไว้ครบถ้วน</p>
            {pendingModeSwitch === "weighted" ? (
              <p>• โหมด Weighted: ทุกคนที่มาเรียนมีโอกาสถูกสุ่ม แต่คนที่ยังไม่ค่อยถูกสุ่มจะมีช่องวงล้อและโอกาสสูงกว่า</p>
            ) : (
              <p>• โหมด No Repeat: คนที่ถูกสุ่มไปแล้วในรอบนี้จะถูกคัดออกชั่วคราวจนกว่าจะครบทุกคน</p>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setPendingModeSwitch(null)} disabled={switchingMode}>
              ยกเลิก
            </Button>
            <Button type="button" onClick={handleConfirmModeChange} disabled={switchingMode}>
              {switchingMode ? "กำลังเปลี่ยนโหมด..." : "ยืนยันเปลี่ยนโหมด"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog 3: Start New Round */}
      <Dialog open={newRoundDialogOpen} onOpenChange={setNewRoundDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <RotateCcw className="size-4 text-emerald-600" /> ยืนยันการเริ่มรอบการสุ่มใหม่
            </DialogTitle>
            <DialogDescription>
              คุณต้องการขึ้นรอบการสุ่มใหม่ (รอบที่ #{(pool?.currentRound || 1) + 1}) สำหรับโหมด No Repeat ใช่หรือไม่?
            </DialogDescription>
          </DialogHeader>
          <div className="py-2 text-xs text-muted-foreground bg-muted/50 p-3 rounded-lg border">
            เมื่อเริ่มรอบใหม่ นักศึกษาทุกคนที่มีสถานะมาเรียน (Present) หรือมาสาย (Late) ในคาบนี้ จำนวน{" "}
            <span className="font-semibold text-foreground">{pool?.totalEligible || 0} คน</span> จะกลับเข้าสู่วงล้อเพื่อสุ่มในรอบถัดไป
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setNewRoundDialogOpen(false)} disabled={startingNewRound}>
              ยกเลิก
            </Button>
            <Button type="button" className="bg-emerald-600 hover:bg-emerald-700 text-white" onClick={handleConfirmNewRound} disabled={startingNewRound}>
              {startingNewRound ? "กำลังเริ่มรอบใหม่..." : "ยืนยันเริ่มรอบใหม่"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirmation Dialog 4: Cancel Pick */}
      <Dialog open={cancelDialogOpen} onOpenChange={setCancelDialogOpen}>
        <DialogContent>
          <form onSubmit={handleConfirmCancelPick}>
            <DialogHeader>
              <DialogTitle className="text-rose-600 flex items-center gap-2">
                <Ban className="size-5" /> ยืนยันการยกเลิกผลการสุ่ม
              </DialogTitle>
              <DialogDescription>
                คุณต้องการยกเลิกผลการสุ่มของ{" "}
                <span className="font-semibold text-foreground">
                  {lastPickResult?.pick.student.studentCode} {lastPickResult?.pick.student.fullName}
                </span>{" "}
                ใช่หรือไม่?
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="cancel_reason">สาเหตุที่ยกเลิกผลการสุ่ม</Label>
                <Input
                  id="cancel_reason"
                  placeholder="เช่น ขออนุญาตเข้าห้องน้ำ, ไม่อยู่ในห้อง, สุ่มใหม่โดยอาจารย์"
                  value={cancelReason}
                  maxLength={255}
                  onChange={(e) => setCancelReason(e.target.value)}
                  required
                />
              </div>
              <div className="flex items-center justify-between gap-3 rounded-lg border p-3 bg-muted/40">
                <div className="space-y-0.5">
                  <Label className="text-sm font-medium">ปรับสถานะเป็น "ขาดเรียน (Absent)" ด้วย</Label>
                  <p className="text-xs text-muted-foreground">
                    เปิดใช้งานหากเรียกชื่อแล้วนักศึกษาไม่อยู่ในห้องเรียน เพื่อคัดออกจากวงล้อสุ่มในคาบนี้
                  </p>
                </div>
                <Switch checked={markAbsentOnCancel} onCheckedChange={setMarkAbsentOnCancel} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCancelDialogOpen(false)} disabled={cancellingPick}>
                ปิดหน้าต่าง
              </Button>
              <Button type="submit" variant="destructive" disabled={cancellingPick}>
                {cancellingPick ? "กำลังยกเลิก..." : "ยืนยันยกเลิกผลการสุ่ม"}
              </Button>
            </DialogFooter>
          </form>
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
        <div className="text-center py-16 text-muted-foreground">กำลังโหลดระบบสุ่ม...</div>
      ) : !activeSession ? (
        <Card className="text-center py-16 border-dashed">
          <CardContent className="space-y-4">
            <Shuffle className="size-16 mx-auto text-muted-foreground opacity-40 animate-pulse" />
            <h3 className="text-xl font-semibold">ยังไม่มีคาบเรียนเปิดอยู่</h3>
            <p className="text-sm text-muted-foreground max-w-md mx-auto">
              ระบบสุ่ม Fair Pick ต้องทำงานร่วมกับคาบเรียนที่เปิดใช้งาน เพื่อตรวจสอบสถานะการเข้าเรียนและนับรอบการตอบ
            </p>
            <Button onClick={() => setStartSessionDialogOpen(true)} className="gap-2 bg-emerald-600 hover:bg-emerald-700 text-white">
              <Play className="size-4 fill-current" /> เริ่มคาบเรียนเพื่อเริ่มสุ่ม
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-6">
          {/* Main Picker Card */}
          <Card className="border-2 border-indigo-100 dark:border-indigo-950 shadow-md overflow-hidden">
            <CardHeader className="text-center pb-2 bg-gradient-to-b from-indigo-50/50 to-transparent dark:from-indigo-950/30">
              <div className="flex flex-wrap items-center justify-center gap-2 mb-1">
                <Badge variant="outline" className="bg-background">
                  คาบที่ #{activeSession.sessionNumber}
                  {activeSession.topic ? `: ${activeSession.topic}` : ""}
                </Badge>
                <Badge variant="secondary">
                  รอบที่ #{pool?.currentRound || 1} • โหมด {activeSession.currentMode === "weighted" ? "Weighted (ถ่วงน้ำหนัก)" : "No Repeat (ไม่ซ้ำ)"}
                </Badge>
              </div>
              <CardTitle className="text-xl">วงล้อสุ่มผู้โชคดีตอบคำถาม</CardTitle>
              <CardDescription className="text-xs">
                ผู้มีสิทธิ์ถูกสุ่ม {pool?.totalEligible || 0} คน
                {activeSession.currentMode === "no_repeat" && ` (คงเหลือในรอบนี้ ${pool?.remainingInNoRepeatRound || 0} คน)`}
                {activeSession.currentMode === "weighted" && ` • สูตรคำนวณน้ำหนัก: ${pool?.weightFormula || "W = 1 / (1 + H + 2C)"}`}
              </CardDescription>
            </CardHeader>

            <CardContent className="flex flex-col items-center justify-center py-6 gap-6">
              {displayStyle === "wheel" && (
                <SpinningWheel
                  candidates={pool?.pool || []}
                  mode={activeSession.currentMode}
                  spinning={picking}
                  targetStudentId={wheelTargetId}
                  spinKey={spinKey}
                  disabled={picking || (pool?.totalEligible || 0) === 0 || !!pool?.isNoRepeatRoundCompleted}
                  onRequestSpin={handleExecutePick}
                  onSpinComplete={handleWheelSpinComplete}
                />
              )}

              {/* Winner Banner / Card */}
              {lastPickResult && !picking ? (
                <div className="w-full max-w-lg rounded-2xl border-2 border-indigo-200 dark:border-indigo-800 bg-gradient-to-br from-indigo-50/90 via-white to-violet-50/90 dark:from-indigo-950/80 dark:via-background dark:to-violet-950/60 p-6 text-center shadow-lg animate-in fade-in zoom-in-95 duration-300 space-y-2">
                  <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-indigo-600 text-white text-xs font-semibold mb-1 shadow-sm">
                    <Sparkles className="size-3.5" /> ผู้โชคดีที่ได้รับเลือก!
                  </div>
                  <div className="text-2xl sm:text-3xl font-extrabold text-foreground tracking-tight">
                    {lastPickResult.pick.student.fullName}
                  </div>
                  <div className="text-base font-mono font-semibold text-indigo-600 dark:text-indigo-400">
                    รหัสนักศึกษา: {lastPickResult.pick.student.studentCode}
                  </div>
                  <div className="flex flex-wrap items-center justify-center gap-2 pt-1 text-xs text-muted-foreground">
                    <Badge variant="outline" className="text-xs">
                      สถานะเช็กชื่อ: {lastPickResult.pick.student.attendanceStatus === "present" ? "มาเรียน" : "มาสาย"}
                    </Badge>
                    {lastPickResult.pick.weightScore !== null && (
                      <Badge variant="outline" className="text-xs">
                        ค่าน้ำหนัก (W): {lastPickResult.pick.weightScore}
                      </Badge>
                    )}
                  </div>
                </div>
              ) : displayStyle === "card" ? (
                <div className="text-center py-8 text-muted-foreground">
                  <Shuffle className={`size-14 mx-auto mb-3 opacity-30 ${picking ? "animate-spin text-indigo-600 opacity-80" : ""}`} />
                  <p className="text-base font-medium">
                    {picking ? "กำลังสุ่มรายชื่อนักศึกษา..." : "กดปุ่มด้านล่างเพื่อเริ่มการสุ่ม"}
                  </p>
                </div>
              ) : null}

              {(pool?.totalEligible || 0) === 0 && (
                <div className="text-center space-y-2 bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800 rounded-xl p-4 max-w-md">
                  <p className="text-sm font-medium text-amber-800 dark:text-amber-300">
                    ยังไม่มีนักศึกษาที่มีสถานะ "มาเรียน" หรือ "มาสาย" ในคาบนี้
                  </p>
                  <p className="text-xs text-amber-700/80 dark:text-amber-400/80">
                    กรุณาไปที่หน้าใบเช็กชื่อเพื่อเช็กชื่อนักศึกษาเข้าห้องเรียนก่อนเริ่มหมุนวงล้อสุ่ม
                  </p>
                  <Button render={<Link href={`/courses/${courseId}/attendance`} />} size="sm" variant="outline" className="gap-1.5 mt-1">
                    <ClipboardCheck className="size-4" /> ไปหน้าเช็กชื่อเข้าเรียน
                  </Button>
                </div>
              )}
            </CardContent>

            <CardFooter className="flex flex-col sm:flex-row flex-wrap gap-3 justify-center pt-2 pb-6">
              {pool?.isNoRepeatRoundCompleted && activeSession.currentMode === "no_repeat" ? (
                <Button
                  size="lg"
                  onClick={() => setNewRoundDialogOpen(true)}
                  className="w-full sm:w-auto px-8 gap-2 bg-emerald-600 hover:bg-emerald-700 text-white font-semibold shadow-md"
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
                  {picking ? "กำลังหมุนวงล้อสุ่ม..." : "หมุนวงล้อสุ่มตอบคำถาม"}
                </Button>
              )}

              {activeSession.currentMode === "no_repeat" && !pool?.isNoRepeatRoundCompleted && (pool?.totalEligible || 0) > 0 && (
                <Button
                  variant="outline"
                  size="lg"
                  disabled={picking}
                  onClick={() => setNewRoundDialogOpen(true)}
                  className="w-full sm:w-auto gap-1.5 text-xs"
                >
                  <RotateCcw className="size-3.5" /> รีเซ็ตเริ่มรอบใหม่
                </Button>
              )}

              {lastPickResult && !picking && (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={openCancelPickDialog}
                  className="w-full sm:w-auto gap-2 text-rose-600 border-rose-200 hover:text-rose-700 hover:bg-rose-50"
                >
                  <Ban className="size-4" /> ยกเลิกผลการสุ่มนี้
                </Button>
              )}
            </CardFooter>
          </Card>

          {/* Candidates in pool */}
          <Card>
            <CardHeader className="py-4">
              <div className="flex flex-wrap justify-between items-center gap-2">
                <div>
                  <CardTitle className="text-base">
                    รายชื่อผู้มีสิทธิ์ถูกสุ่มในสระ ({pool?.pool?.length || 0} คน)
                  </CardTitle>
                  <CardDescription className="text-xs">
                    แสดงเฉพาะนักศึกษาที่เช็กชื่อ "มาเรียน" หรือ "มาสาย" ในคาบปัจจุบัน
                  </CardDescription>
                </div>
                <Badge variant="secondary">รอบที่ #{pool?.currentRound || 1}</Badge>
              </div>
            </CardHeader>
            <CardContent>
              {pool?.pool && pool.pool.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {pool.pool.map((candidate) => {
                    const isPickedInNoRepeat =
                      activeSession.currentMode === "no_repeat" && candidate.pickedInCurrentNoRepeatRound;
                    const isLatestWinner =
                      !picking && lastPickResult?.pick.student.id === candidate.studentId;
                    return (
                      <Badge
                        key={candidate.studentId}
                        variant={isPickedInNoRepeat ? "outline" : "secondary"}
                        className={`px-2.5 py-1 text-xs font-normal transition-all ${
                          isPickedInNoRepeat ? "opacity-40 line-through" : ""
                        } ${isLatestWinner ? "ring-2 ring-indigo-600 bg-indigo-50 dark:bg-indigo-950 font-semibold" : ""}`}
                      >
                        <span className="font-mono mr-1.5 opacity-70">{candidate.studentCode}</span>
                        <span>{candidate.fullName}</span>
                        {activeSession.currentMode === "weighted" && (
                          <span className="ml-1.5 font-bold text-indigo-600 dark:text-indigo-400">
                            ({candidate.probabilityPercent.toFixed(1)}%)
                          </span>
                        )}
                      </Badge>
                    );
                  })}
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
