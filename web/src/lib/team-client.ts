// Hand-wired Connect+JSON client for TeamService. Mirrors the
// companies-client / install-client pattern; will be replaced when the
// proto-es codegen pipeline lands.
//
// Wire format: Connect+JSON serialises proto enums as their full
// SCREAMING_SNAKE_CASE name. We expose them as friendlier "admin" /
// "technician" / "active" / "suspended" strings to the UI and translate
// at the boundary.

import { apiCall } from "@/lib/api-client";

const SERVICE = "/gospa.team.v1.TeamService";

export type MemberRole = "admin" | "technician";

export type MemberStatus = "active" | "not_signed_in_yet" | "suspended";

export interface TeamMember {
  id: string;
  fullName: string;
  email: string;
  role: MemberRole;
  status: MemberStatus;
  lastSeenAt: string;
  createdAt: string;
}

interface WireMember {
  id: string;
  fullName?: string;
  email?: string;
  role?: WireRole;
  status?: WireStatus;
  lastSeenAt?: string;
  createdAt?: string;
}

type WireRole =
  | "MEMBER_ROLE_UNSPECIFIED"
  | "MEMBER_ROLE_ADMIN"
  | "MEMBER_ROLE_TECHNICIAN";

type WireStatus =
  | "MEMBER_STATUS_UNSPECIFIED"
  | "MEMBER_STATUS_ACTIVE"
  | "MEMBER_STATUS_NOT_SIGNED_IN_YET"
  | "MEMBER_STATUS_SUSPENDED";

function roleFromWire(w: WireRole | undefined): MemberRole {
  return w === "MEMBER_ROLE_ADMIN" ? "admin" : "technician";
}

function roleToWire(r: MemberRole): WireRole {
  return r === "admin" ? "MEMBER_ROLE_ADMIN" : "MEMBER_ROLE_TECHNICIAN";
}

function statusFromWire(w: WireStatus | undefined): MemberStatus {
  switch (w) {
    case "MEMBER_STATUS_NOT_SIGNED_IN_YET":
      return "not_signed_in_yet";
    case "MEMBER_STATUS_SUSPENDED":
      return "suspended";
    default:
      return "active";
  }
}

function memberFromWire(w: WireMember): TeamMember {
  return {
    id: w.id,
    fullName: w.fullName ?? "",
    email: w.email ?? "",
    role: roleFromWire(w.role),
    status: statusFromWire(w.status),
    lastSeenAt: w.lastSeenAt ?? "",
    createdAt: w.createdAt ?? "",
  };
}

export async function listMembers(accessToken: string): Promise<TeamMember[]> {
  const resp = await apiCall<Record<string, never>, { members?: WireMember[] }>(
    SERVICE,
    "ListMembers",
    {},
    { accessToken },
  );
  return (resp.members ?? []).map(memberFromWire);
}

export interface InviteMemberInput {
  fullName: string;
  email: string;
  role: MemberRole;
}

export interface InviteMemberResult {
  member: TeamMember;
  temporaryPassword: string;
}

export async function inviteMember(
  input: InviteMemberInput,
  accessToken: string,
): Promise<InviteMemberResult> {
  const resp = await apiCall<
    { fullName: string; email: string; role: WireRole },
    { member: WireMember; temporaryPassword: string }
  >(
    SERVICE,
    "InviteMember",
    {
      fullName: input.fullName,
      email: input.email,
      role: roleToWire(input.role),
    },
    { accessToken },
  );
  return {
    member: memberFromWire(resp.member),
    temporaryPassword: resp.temporaryPassword,
  };
}

export async function changeRole(
  contactId: string,
  role: MemberRole,
  accessToken: string,
): Promise<TeamMember> {
  const resp = await apiCall<
    { contactId: string; role: WireRole },
    { member: WireMember }
  >(
    SERVICE,
    "ChangeRole",
    { contactId, role: roleToWire(role) },
    { accessToken },
  );
  return memberFromWire(resp.member);
}

export async function suspendMember(
  contactId: string,
  accessToken: string,
): Promise<TeamMember> {
  const resp = await apiCall<{ contactId: string }, { member: WireMember }>(
    SERVICE,
    "SuspendMember",
    { contactId },
    { accessToken },
  );
  return memberFromWire(resp.member);
}

export async function reactivateMember(
  contactId: string,
  accessToken: string,
): Promise<TeamMember> {
  const resp = await apiCall<{ contactId: string }, { member: WireMember }>(
    SERVICE,
    "ReactivateMember",
    { contactId },
    { accessToken },
  );
  return memberFromWire(resp.member);
}
