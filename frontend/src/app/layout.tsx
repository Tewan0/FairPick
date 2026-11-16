import type { Metadata } from "next";
import "./globals.css";
import { IBM_Plex_Sans_Thai } from "next/font/google";
import { cn } from "@/lib/utils";
import { Navbar } from "@/components/Navbar";

const ibmPlexSansThai = IBM_Plex_Sans_Thai({
  weight: ["300", "400", "500", "600", "700"],
  subsets: ["thai", "latin"],
  variable: "--font-sans",
});

export const metadata: Metadata = {
  title: "FairPick - ระบบสุ่มตอบคำถามและเช็กชื่อแบบยุติธรรม",
  description: "Fair Student Picker and Attendance Tracking System",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="th" className={cn("font-sans", ibmPlexSansThai.variable)}>
      <body className="min-h-screen bg-background text-foreground antialiased flex flex-col font-sans">
        <Navbar />
        <div className="flex-1 container mx-auto p-4 md:p-6">{children}</div>
      </body>
    </html>
  );
}
