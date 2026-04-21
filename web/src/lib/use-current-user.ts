import { useAuth } from "react-oidc-context";

// Returns the signed-in operator's email (from the JWT's `email` claim)
// or null when unauthenticated or the claim is missing. Tables compare
// a row's owner email against this to render "Você" instead of the
// owner's full name — matches ZITADEL's email-based identity mapping
// and keeps the check hook-free-adjacent for leaf cells.
export function useCurrentUserEmail(): string | null {
  const auth = useAuth();
  const email = auth.user?.profile.email;
  return typeof email === "string" ? email : null;
}

// Returns true when the given email equals the signed-in operator's
// email. Case-insensitive so user@foo.com and User@Foo.com match — the
// JWT's email claim is not guaranteed case-canonicalised across IdPs.
export function matchesCurrentUser(
  email: string | undefined | null,
  currentUserEmail: string | null,
): boolean {
  if (!email || !currentUserEmail) return false;
  return email.toLowerCase() === currentUserEmail.toLowerCase();
}
