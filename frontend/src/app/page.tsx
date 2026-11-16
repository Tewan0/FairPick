import Link from "next/link";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { BookOpen, Users, Shuffle, ClipboardCheck, BarChart3, ArrowRight } from "lucide-react";

export default function HomePage() {
  return (
    <div className="flex flex-col gap-8 max-w-5xl mx-auto py-8">
      <div className="text-center space-y-3">
        <h1 className="text-4xl font-extrabold tracking-tight lg:text-5xl">
          ยินดีต้อนรับสู่ <span className="text-indigo-600 dark:text-indigo-400">FairPick</span>
        </h1>
        <p className="text-muted-foreground text-lg max-w-2xl mx-auto">
          ระบบสุ่มตอบคำถามและเช็กชื่อนักศึกษาแบบโปร่งใส กระจายโอกาสอย่างยุติธรรม พร้อมรายงานสถิติแบบครบวงจร
        </p>
        <div className="pt-2">
          <Button render={<Link href="/courses" />} size="lg" className="gap-2">
            เข้าสู่ระบบจัดการรายวิชา <ArrowRight className="size-4" />
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 pt-6">
        <Card>
          <CardHeader>
            <div className="p-2 w-fit rounded-lg bg-indigo-50 dark:bg-indigo-950/50 text-indigo-600 mb-2">
              <BookOpen className="size-6" />
            </div>
            <CardTitle>จัดการรายวิชา</CardTitle>
            <CardDescription>สร้างและบริหารข้อมูลวิชาเรียน เทอม ปีการศึกษา</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              จัดการวิชาที่สอน จัดการโครงสร้างห้องเรียน และเชื่อมโยงข้อมูลนักศึกษา
            </p>
          </CardContent>
          <CardFooter>
            <Button render={<Link href="/courses" />} variant="outline" className="w-full">
              ไปหน้ารายวิชา
            </Button>
          </CardFooter>
        </Card>

        <Card>
          <CardHeader>
            <div className="p-2 w-fit rounded-lg bg-emerald-50 dark:bg-emerald-950/50 text-emerald-600 mb-2">
              <Shuffle className="size-6" />
            </div>
            <CardTitle>ระบบสุ่ม Fair Pick</CardTitle>
            <CardDescription>สุ่มตอบคำถามแบบโปร่งใส ยุติธรรม</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              มีทั้งโหมด Normal, Fair (กระจายโอกาส), และ Weighted พร้อมปุ่มเริ่มรอบใหม่หรือยกเลิกผล
            </p>
          </CardContent>
          <CardFooter>
            <Button render={<Link href="/courses" />} variant="outline" className="w-full">
              เลือกวิชาเพื่อเริ่มสุ่ม
            </Button>
          </CardFooter>
        </Card>

        <Card>
          <CardHeader>
            <div className="p-2 w-fit rounded-lg bg-amber-50 dark:bg-amber-950/50 text-amber-600 mb-2">
              <ClipboardCheck className="size-6" />
            </div>
            <CardTitle>ใบเช็กชื่อประจำคาบ</CardTitle>
            <CardDescription>เช็กชื่อ มา / สาย / ลา / ขาด</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              เชื่อมโยงการเช็กชื่อเข้ากับระบบสุ่ม นักศึกษาที่ไม่มาจะถูกคัดออกจากสระการสุ่มโดยอัตโนมัติ
            </p>
          </CardContent>
          <CardFooter>
            <Button render={<Link href="/courses" />} variant="outline" className="w-full">
              ดูใบเช็กชื่อ
            </Button>
          </CardFooter>
        </Card>
      </div>
    </div>
  );
}
