import { User } from "lucide-react";

import { cn } from "@/lib/utils";

const TONES = [
  "#3291ff",
  "#a1a1a1",
  "#c77dff",
  "#0cce6b",
  "#f5a623",
  "#5a9fd4",
  "#ff79c6",
];

function hashTone(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return TONES[h % TONES.length];
}

function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .map((w) => w[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

export function OwnerAvatar({
  contactId,
  fullName,
  size = 22,
  className,
}: {
  contactId?: string;
  fullName?: string;
  size?: number;
  className?: string;
}) {
  const dim = `${size}px`;
  const fontSize = Math.round(size * 0.42);
  if (!contactId || !fullName) {
    return (
      <span
        title="Sem dono"
        className={cn(
          "inline-flex flex-shrink-0 items-center justify-center rounded-full border border-dashed border-border-default text-fg-4",
          className,
        )}
        style={{ width: dim, height: dim }}
      >
        <User
          style={{ width: size * 0.45, height: size * 0.45 }}
          strokeWidth={1.5}
        />
      </span>
    );
  }
  return (
    <span
      title={fullName}
      className={cn(
        "inline-flex flex-shrink-0 items-center justify-center rounded-full font-semibold text-white",
        className,
      )}
      style={{ width: dim, height: dim, background: hashTone(contactId), fontSize }}
    >
      {initials(fullName)}
    </span>
  );
}
