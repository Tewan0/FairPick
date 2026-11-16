"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Users, Shuffle, ClipboardCheck, History, BarChart3, ArrowLeft } from "lucide-react";

interface CourseNavProps {
  courseId: number | string;
}

export function CourseNav({ courseId }: CourseNavProps) {
  const pathname = usePathname();

  const links = [
    {
      href: `/courses/${courseId}/pick`,
      label: "สุ่มชื่อ (Fair Pick)",
      icon: Shuffle,
    },
    {
      href: `/courses/${courseId}/attendance`,
      label: "เช็กชื่อ (Attendance)",
      icon: ClipboardCheck,
    },
    {
      href: `/courses/${courseId}/students`,
      label: "นักศึกษา (Students)",
      icon: Users,
    },
    {
      href: `/courses/${courseId}/history`,
      label: "ประวัติการสุ่ม (History)",
      icon: History,
    },
    {
      href: `/courses/${courseId}/stats`,
      label: "สถิติภาพรวม (Stats)",
      icon: BarChart3,
    },
  ];

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b pb-4 mb-6">
      <div className="flex items-center gap-2">
        <Button render={<Link href="/courses" />} variant="ghost" size="sm" className="gap-1 text-muted-foreground">
          <ArrowLeft className="size-4" /> รายวิชาทั้งหมด
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        {links.map((link) => {
          const Icon = link.icon;
          const isActive = pathname.startsWith(link.href);
          return (
            <Button
              key={link.href}
              render={<Link href={link.href} />}
              variant={isActive ? "default" : "outline"}
              size="sm"
              className={cn("gap-1.5", isActive && "shadow-sm")}
            >
              <Icon className="size-3.5" />
              <span>{link.label}</span>
            </Button>
          );
        })}
      </div>
    </div>
  );
}
