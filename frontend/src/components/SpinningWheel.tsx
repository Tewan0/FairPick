"use client";

import React, { useEffect, useRef, useState, useMemo, useCallback } from "react";
import { PickPoolCandidate, PickMode } from "@/types/fairpick";
import { Volume2, VolumeX, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";

const WHEEL_COLORS = [
  "#4f46e5", // indigo-600
  "#059669", // emerald-600
  "#d97706", // amber-600
  "#e11d48", // rose-600
  "#7c3aed", // violet-600
  "#0284c7", // sky-600
  "#ea580c", // orange-600
  "#0d9488", // teal-600
  "#2563eb", // blue-600
  "#c026d3", // fuchsia-600
];

export interface WheelSlice {
  candidate: PickPoolCandidate;
  startAngle: number; // radians, 0 = right (3 o'clock), clockwise
  endAngle: number;
  midAngle: number;
  color: string;
}

interface SpinningWheelProps {
  candidates: PickPoolCandidate[];
  mode: PickMode;
  spinning: boolean;
  targetStudentId: number | null;
  spinKey: number;
  disabled?: boolean;
  onRequestSpin: () => void;
  onSpinComplete: () => void;
}

export function SpinningWheel({
  candidates,
  mode,
  spinning,
  targetStudentId,
  spinKey,
  disabled = false,
  onRequestSpin,
  onSpinComplete,
}: SpinningWheelProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const rotationRef = useRef<number>(0); // radians
  const animFrameRef = useRef<number | null>(null);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const lastSliceIdxRef = useRef<number>(-1);

  const [soundEnabled, setSoundEnabled] = useState(true);
  const [proportionalSlices, setProportionalSlices] = useState(true);
  const [needleBounce, setNeedleBounce] = useState(false);
  const [currentHoverName, setCurrentHoverName] = useState<string | null>(null);
  // Keep a snapshot of candidates during active spin so pool updates don't shift slices mid-spin
  const [frozenCandidates, setFrozenCandidates] = useState<PickPoolCandidate[] | null>(null);

  const activeCandidates = useMemo(() => {
    const source = frozenCandidates ?? candidates;
    if (mode === "no_repeat") {
      const unpicked = source.filter((c) => !c.pickedInCurrentNoRepeatRound);
      return unpicked.length > 0 ? unpicked : source;
    }
    return source;
  }, [candidates, frozenCandidates, mode]);

  const slices: WheelSlice[] = useMemo(() => {
    if (activeCandidates.length === 0) return [];

    const useWeightedSize = mode === "weighted" && proportionalSlices;
    const totalWeight = useWeightedSize
      ? activeCandidates.reduce((acc, c) => acc + Math.max(c.weightScore, 0.05), 0)
      : activeCandidates.length;

    let currentAngle = 0;
    return activeCandidates.map((cand, idx) => {
      const weight = useWeightedSize ? Math.max(cand.weightScore, 0.05) : 1;
      const span = (weight / totalWeight) * Math.PI * 2;
      const startAngle = currentAngle;
      const endAngle = currentAngle + span;
      const midAngle = startAngle + span / 2;
      currentAngle = endAngle;

      // Ensure adjacent slices don't share the same color when last wraps to first
      let colorIdx = idx % WHEEL_COLORS.length;
      if (idx === activeCandidates.length - 1 && activeCandidates.length > 1 && colorIdx === 0) {
        colorIdx = 1;
      }

      return {
        candidate: cand,
        startAngle,
        endAngle,
        midAngle,
        color: WHEEL_COLORS[colorIdx],
      };
    });
  }, [activeCandidates, mode, proportionalSlices]);

  const playTickSound = useCallback(() => {
    if (!soundEnabled || typeof window === "undefined") return;
    try {
      const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      if (!AudioCtx) return;
      if (!audioCtxRef.current) {
        audioCtxRef.current = new AudioCtx();
      }
      const ctx = audioCtxRef.current;
      if (ctx.state === "suspended") {
        ctx.resume();
      }
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "triangle";
      osc.frequency.setValueAtTime(620, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(220, ctx.currentTime + 0.035);

      gain.gain.setValueAtTime(0.08, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.035);

      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.04);
    } catch {
      // Ignore audio errors if blocked by browser autoplay policy
    }
  }, [soundEnabled]);

  const playWinSound = useCallback(() => {
    if (!soundEnabled || typeof window === "undefined") return;
    try {
      const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      if (!AudioCtx) return;
      if (!audioCtxRef.current) {
        audioCtxRef.current = new AudioCtx();
      }
      const ctx = audioCtxRef.current;
      if (ctx.state === "suspended") {
        ctx.resume();
      }
      const notes = [523.25, 659.25, 783.99, 1046.5]; // C5, E5, G5, C6
      notes.forEach((freq, i) => {
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = "sine";
        const startTime = ctx.currentTime + i * 0.09;
        osc.frequency.setValueAtTime(freq, startTime);
        gain.gain.setValueAtTime(0.12, startTime);
        gain.gain.exponentialRampToValueAtTime(0.001, startTime + 0.35);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(startTime);
        osc.stop(startTime + 0.36);
      });
    } catch {
      // Ignore audio errors
    }
  }, [soundEnabled]);

  // Helper to find which slice is currently under the top pointer (-PI/2 = 270 deg)
  const getSliceIndexAtPointer = useCallback(
    (rot: number, currentSlices: WheelSlice[]): number => {
      if (currentSlices.length === 0) return -1;
      const pointerAngle = (3 * Math.PI) / 2; // top of wheel (12 o'clock)
      let normalized = (pointerAngle - (rot % (Math.PI * 2)) + Math.PI * 4) % (Math.PI * 2);
      for (let i = 0; i < currentSlices.length; i++) {
        if (normalized >= currentSlices[i].startAngle && normalized < currentSlices[i].endAngle) {
          return i;
        }
      }
      return currentSlices.length - 1;
    },
    []
  );

  // Draw the wheel on canvas
  const drawWheel = useCallback(
    (rot: number, highlightWinnerId: number | null) => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      const dpr = typeof window !== "undefined" ? window.devicePixelRatio || 1 : 1;
      const size = 340;
      if (canvas.width !== size * dpr || canvas.height !== size * dpr) {
        canvas.width = size * dpr;
        canvas.height = size * dpr;
      }
      ctx.save();
      ctx.scale(dpr, dpr);

      const cx = size / 2;
      const cy = size / 2;
      const outerRadius = size / 2 - 14;
      const innerRadius = 38;

      ctx.clearRect(0, 0, size, size);

      // Outer decorative rim shadow & ring
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, outerRadius + 8, 0, Math.PI * 2);
      ctx.fillStyle = "#1e1b4b";
      ctx.shadowColor = "rgba(79, 70, 229, 0.28)";
      ctx.shadowBlur = 18;
      ctx.fill();
      ctx.restore();

      if (slices.length === 0) {
        ctx.beginPath();
        ctx.arc(cx, cy, outerRadius, 0, Math.PI * 2);
        ctx.fillStyle = "#f1f5f9";
        ctx.fill();
        ctx.fillStyle = "#64748b";
        ctx.font = "600 14px sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("ยังไม่มีรายชื่อผู้มีสิทธิ์สุ่มในคาบนี้", cx, cy - 10);
        ctx.font = "400 12px sans-serif";
        ctx.fillText("กรุณาเช็กชื่อ มา หรือ สาย ก่อนเริ่มหมุนวงล้อ", cx, cy + 14);
        ctx.restore();
        return;
      }

      // Draw each slice rotated by `rot`
      slices.forEach((slice) => {
        const start = slice.startAngle + rot;
        const end = slice.endAngle + rot;
        const mid = slice.midAngle + rot;
        const span = slice.endAngle - slice.startAngle;
        const isWinner = !spinning && highlightWinnerId === slice.candidate.studentId;

        ctx.save();
        ctx.beginPath();
        ctx.moveTo(cx, cy);
        ctx.arc(cx, cy, outerRadius, start, end);
        ctx.closePath();

        ctx.fillStyle = slice.color;
        ctx.fill();

        if (isWinner) {
          ctx.lineWidth = 4;
          ctx.strokeStyle = "#fde047"; // yellow-300 highlight
          ctx.stroke();
        } else {
          ctx.lineWidth = slices.length > 25 ? 1 : 2;
          ctx.strokeStyle = "#ffffff";
          ctx.stroke();
        }
        ctx.restore();

        // Draw slice label if slice span is large enough
        if (span > 0.08) {
          ctx.save();
          ctx.translate(cx, cy);
          ctx.rotate(mid);
          ctx.textAlign = "right";
          ctx.textBaseline = "middle";
          ctx.fillStyle = "#ffffff";

          const maxTextWidth = outerRadius - innerRadius - 22;
          const fontSize = slices.length > 20 ? 10 : slices.length > 12 ? 11 : 13;
          ctx.font = `600 ${fontSize}px sans-serif`;

          let label = slice.candidate.fullName;
          if (label.length > 16) {
            label = label.slice(0, 15) + "…";
          }

          ctx.shadowColor = "rgba(0,0,0,0.35)";
          ctx.shadowBlur = 3;
          ctx.fillText(label, outerRadius - 14, 0, maxTextWidth);
          ctx.restore();
        }
      });

      // Draw rim lights (pegs around the wheel)
      const pegCount = Math.min(Math.max(slices.length * 2, 12), 36);
      for (let i = 0; i < pegCount; i++) {
        const angle = (i / pegCount) * Math.PI * 2 + rot;
        const px = cx + Math.cos(angle) * (outerRadius + 4);
        const py = cy + Math.sin(angle) * (outerRadius + 4);
        ctx.beginPath();
        ctx.arc(px, py, 2.5, 0, Math.PI * 2);
        ctx.fillStyle = i % 2 === 0 ? "#fde047" : "#ffffff";
        ctx.fill();
      }

      // Draw inner hub circle
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, innerRadius, 0, Math.PI * 2);
      ctx.fillStyle = "#ffffff";
      ctx.shadowColor = "rgba(0,0,0,0.25)";
      ctx.shadowBlur = 8;
      ctx.fill();
      ctx.lineWidth = 4;
      ctx.strokeStyle = "#4f46e5";
      ctx.stroke();
      ctx.restore();

      ctx.restore();
    },
    [slices, spinning]
  );

  // Initial & idle redraw
  useEffect(() => {
    if (!spinning) {
      drawWheel(rotationRef.current, targetStudentId);
    }
  }, [drawWheel, spinning, targetStudentId, slices]);

  // Freeze candidates when spin starts so slices don't jump before animation completes
  useEffect(() => {
    if (spinning && !frozenCandidates) {
      setFrozenCandidates(candidates);
    } else if (!spinning && frozenCandidates) {
      setFrozenCandidates(null);
    }
  }, [spinning, candidates, frozenCandidates]);

  // Trigger smooth spin animation when spinKey changes and targetStudentId is known
  useEffect(() => {
    if (spinKey === 0 || targetStudentId === null || slices.length === 0) return;

    const targetSliceIndex = slices.findIndex((s) => s.candidate.studentId === targetStudentId);
    const chosenSlice = targetSliceIndex >= 0 ? slices[targetSliceIndex] : slices[0];

    // Pointer is at 12 o'clock = 3 * PI / 2
    const pointerAngle = (3 * Math.PI) / 2;
    const sliceSpan = chosenSlice.endAngle - chosenSlice.startAngle;
    // Land near the center of the winning slice (within middle 50% of the slice)
    const jitter = (Math.random() - 0.5) * sliceSpan * 0.45;
    const targetSliceAngle = chosenSlice.midAngle + jitter;

    const startRotation = rotationRef.current;
    const currentMod = ((startRotation % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
    const desiredMod = ((pointerAngle - targetSliceAngle) % (Math.PI * 2) + Math.PI * 2) % (Math.PI * 2);

    let deltaMod = desiredMod - currentMod;
    if (deltaMod <= 0) {
      deltaMod += Math.PI * 2;
    }

    const extraSpins = 6 * Math.PI * 2; // 6 full turns
    const totalDelta = extraSpins + deltaMod;
    const durationMs = 3400;
    const startTime = performance.now();

    lastSliceIdxRef.current = getSliceIndexAtPointer(startRotation, slices);

    const easeOutQuart = (t: number) => 1 - Math.pow(1 - t, 4);

    const step = (now: number) => {
      const elapsed = now - startTime;
      const progress = Math.min(elapsed / durationMs, 1);
      const eased = easeOutQuart(progress);
      const currentRot = startRotation + totalDelta * eased;
      rotationRef.current = currentRot;

      const currentIdx = getSliceIndexAtPointer(currentRot, slices);
      if (currentIdx >= 0 && slices[currentIdx]) {
        setCurrentHoverName(slices[currentIdx].candidate.fullName);
      }

      if (currentIdx !== lastSliceIdxRef.current && lastSliceIdxRef.current !== -1) {
        lastSliceIdxRef.current = currentIdx;
        playTickSound();
        setNeedleBounce(true);
        setTimeout(() => setNeedleBounce(false), 60);
      }

      drawWheel(currentRot, progress === 1 ? targetStudentId : null);

      if (progress < 1) {
        animFrameRef.current = requestAnimationFrame(step);
      } else {
        setCurrentHoverName(chosenSlice.candidate.fullName);
        playWinSound();
        onSpinComplete();
      }
    };

    if (animFrameRef.current) {
      cancelAnimationFrame(animFrameRef.current);
    }
    animFrameRef.current = requestAnimationFrame(step);

    return () => {
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current);
      }
    };
  }, [spinKey, targetStudentId]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="flex flex-col items-center gap-4 w-full">
      {/* Top Controls Bar */}
      <div className="flex flex-wrap items-center justify-center gap-2 text-xs">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 px-2.5 gap-1.5 text-xs"
          onClick={() => setSoundEnabled((prev) => !prev)}
        >
          {soundEnabled ? <Volume2 className="size-3.5 text-indigo-600" /> : <VolumeX className="size-3.5 text-muted-foreground" />}
          <span>{soundEnabled ? "เปิดเสียงเอฟเฟกต์" : "ปิดเสียง"}</span>
        </Button>

        {mode === "weighted" && (
          <Button
            type="button"
            variant={proportionalSlices ? "secondary" : "outline"}
            size="sm"
            className="h-7 px-2.5 gap-1.5 text-xs"
            onClick={() => setProportionalSlices((prev) => !prev)}
            disabled={spinning}
          >
            <Sparkles className="size-3.5 text-amber-500" />
            <span>{proportionalSlices ? "ขนาดช่องตามโอกาสสุ่มจริง (W)" : "ขนาดช่องเท่ากัน"}</span>
          </Button>
        )}
      </div>

      {/* Wheel Container */}
      <div className="relative flex items-center justify-center select-none">
        {/* Top Pointer Needle */}
        <div
          className={`absolute -top-3 z-20 transition-transform duration-75 ${
            needleBounce ? "-rotate-12 scale-110" : "rotate-0"
          }`}
          style={{ filter: "drop-shadow(0 4px 6px rgba(0,0,0,0.35))" }}
        >
          <svg width="36" height="42" viewBox="0 0 36 42" fill="none">
            <path d="M18 40L4 10C4 4.5 10 2 18 2C26 2 32 4.5 32 10L18 40Z" fill="#ef4444" stroke="#ffffff" strokeWidth="2.5" />
            <circle cx="18" cy="12" r="4.5" fill="#fef08a" />
          </svg>
        </div>

        <canvas
          ref={canvasRef}
          style={{ width: 340, height: 340 }}
          className="rounded-full max-w-full"
        />

        {/* Center Spin Button */}
        <button
          type="button"
          disabled={disabled || spinning || activeCandidates.length === 0}
          onClick={onRequestSpin}
          className="absolute z-10 size-[72px] rounded-full bg-gradient-to-br from-indigo-600 to-violet-700 text-white font-bold text-xs shadow-lg border-4 border-white flex flex-col items-center justify-center transition-transform active:scale-95 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
          title="กดเพื่อหมุนวงล้อสุ่มชื่อ"
        >
          <span className="text-[11px] tracking-wider uppercase font-extrabold">
            {spinning ? "..." : "หมุน"}
          </span>
          <span className="text-[9px] opacity-85 font-normal">SPIN</span>
        </button>
      </div>

      {/* Live ticker name while spinning */}
      <div className="h-7 flex items-center justify-center">
        {spinning && currentHoverName ? (
          <div className="px-3.5 py-1 rounded-full bg-indigo-50 dark:bg-indigo-950/80 border border-indigo-200 dark:border-indigo-800 text-indigo-700 dark:text-indigo-300 text-sm font-semibold animate-pulse">
            🎯 {currentHoverName}
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">
            ผู้มีสิทธิ์บนวงล้อ {activeCandidates.length} คน
            {mode === "no_repeat" && ` (ไม่ซ้ำในรอบที่ปัจจุบัน)`}
          </p>
        )}
      </div>
    </div>
  );
}
