// Tiny localStorage helper for the OIDC login_hint convenience.
// Written when the install wizard submits and on every successful
// login (auth.user.profile.email); read by the Log-in button to
// pre-fill ZITADEL's username field so repeat sign-ins are
// password-only.
//
// Browser-local by design: no PII leaves the device and no schema
// change is required. Wiping localStorage simply means the next
// sign-in shows an empty username field — never a security issue.

const KEY = "gospa:last-login-email";

export function setLastLoginEmail(email: string | undefined | null) {
  if (!email) return;
  try {
    window.localStorage.setItem(KEY, email);
  } catch {
    // Quota exceeded / disabled storage — convenience-only feature.
  }
}

export function getLastLoginEmail(): string | undefined {
  try {
    return window.localStorage.getItem(KEY) ?? undefined;
  } catch {
    return undefined;
  }
}
