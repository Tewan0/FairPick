import Link from "next/link";
import { BookOpen, Users, Shuffle, ClipboardCheck, BarChart3, Home } from "lucide-react";

export function Navbar() {
  return (
    <header className="sticky top-0 z-40 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto flex h-14 items-center px-4 justify-between">
        <div className="flex items-center gap-6">
          <Link href="/" className="flex items-center gap-2 font-bold text-lg text-primary">
            <Shuffle className="size-5 text-indigo-500" />
            <span>FairPick</span>
          </Link>
          <nav className="flex items-center gap-4 text-sm font-medium">
            <Link href="/courses" className="flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors">
              <BookOpen className="size-4" />
              <span>รายวิชา (Courses)</span>
            </Link>
          </nav>
        </div>
        <div className="text-xs text-muted-foreground">
          ระบบสุ่มชื่อและเช็กชื่อแบบยุติธรรม
        </div>
      </div>
    </header>
  );
}
