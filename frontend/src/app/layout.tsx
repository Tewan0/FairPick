import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FairPick",
  description: "FairPick Application",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="th">
      <body>{children}</body>
    </html>
  );
}

