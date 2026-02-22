import type { Metadata } from "next";
import DeveloperContent from "./DeveloperContent";

export const metadata: Metadata = {
  title: "Developer Docs — Nexus Agent Protocol",
};

export default function DeveloperPage() {
  return <DeveloperContent />;
}
