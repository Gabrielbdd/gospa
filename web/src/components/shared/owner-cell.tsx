import { OwnerAvatar } from "@/components/companies/owner-avatar";

// Renders an owner inside a table cell: avatar + display name.
// Display rules:
//  - If owner is null: dashed avatar + "Sem dono" (muted italic).
//  - If owner matches the signed-in operator: "Você" (short, friendly).
//  - Else: the owner's first name (compact for dense tables).
// The full name is always carried as a tooltip via the avatar's
// `title` so the operator can still see the full name on hover.
export function OwnerCell({
  owner,
  currentUserEmail,
  ownerEmail,
}: {
  owner: { contactId: string; fullName: string } | null | undefined;
  currentUserEmail: string | null;
  // ownerEmail is optional — used only to match the "is it me?" check.
  // Tickets today don't ship owner email per row (mock data), so they
  // fall back to the handle === "you" convention via the consumer.
  ownerEmail?: string | null;
}) {
  if (!owner) {
    return (
      <span className="inline-flex items-center gap-1.5 text-[12px] italic text-fg-4">
        <OwnerAvatar size={20} />
        <span className="truncate">Sem dono</span>
      </span>
    );
  }

  const isSelf =
    !!ownerEmail &&
    !!currentUserEmail &&
    ownerEmail.toLowerCase() === currentUserEmail.toLowerCase();

  const display = isSelf ? "Você" : firstName(owner.fullName);

  return (
    <span
      title={owner.fullName}
      className="inline-flex min-w-0 items-center gap-1.5 text-[12.5px] text-fg-1"
    >
      <OwnerAvatar
        contactId={owner.contactId}
        fullName={owner.fullName}
        size={20}
      />
      <span className="truncate">{display}</span>
    </span>
  );
}

function firstName(fullName: string): string {
  const parts = fullName.trim().split(/\s+/);
  return parts[0] ?? fullName;
}
