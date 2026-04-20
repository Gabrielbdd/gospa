// Package authz implements Gospa's role-based authorization layer. It runs
// after Gofra's auth gate validates the JWT, looks up the caller's local
// team-grant in Postgres, and enforces a single per-RPC policy map.
//
// Authorization lives entirely in Gospa (workspace_grants.role); ZITADEL
// is identity-only. See repos/gospa/AGENTS.md and the team-contacts-unified
// design doc for the rationale.
package authz

import (
	companiesv1connect "github.com/Gabrielbdd/gospa/gen/gospa/companies/v1/companiesv1connect"
	contactsv1connect "github.com/Gabrielbdd/gospa/gen/gospa/contacts/v1/contactsv1connect"
	teamv1connect "github.com/Gabrielbdd/gospa/gen/gospa/team/v1/teamv1connect"
)

// Level is the access bar a Connect procedure requires.
type Level int

const (
	// LevelAuthenticated allows any team member with an active grant.
	LevelAuthenticated Level = iota
	// LevelAdminOnly restricts to grants whose role is admin.
	LevelAdminOnly
)

// String returns the level name. Used in error responses and logs.
func (l Level) String() string {
	switch l {
	case LevelAdminOnly:
		return "admin_only"
	case LevelAuthenticated:
		return "authenticated"
	default:
		return "unknown"
	}
}

// policy is the single authoritative map of Connect procedure → required
// level. Procedures absent from this map are rejected by the middleware
// with code "policy_undefined" — a deliberate default-deny so a newly
// added handler cannot accidentally serve unprotected.
//
// Public procedures (install, public config) are NOT in this map; the
// middleware short-circuits them via the same PublicProcedures matcher
// the auth gate uses. They live exclusively in cmd/app/main.go where
// the gate is constructed.
//
// Slices that add new RPCs MUST add their entries here in the same
// commit; the test suite would otherwise green-light a regression.
var policy = map[string]Level{
	companiesv1connect.CompaniesServiceListCompaniesProcedure:          LevelAuthenticated,
	companiesv1connect.CompaniesServiceCreateCompanyProcedure:          LevelAuthenticated,
	companiesv1connect.CompaniesServiceUpdateCompanyProcedure:          LevelAuthenticated,
	companiesv1connect.CompaniesServiceArchiveCompanyProcedure:         LevelAdminOnly,
	companiesv1connect.CompaniesServiceGetWorkspaceCompanyProcedure:    LevelAuthenticated,
	companiesv1connect.CompaniesServiceUpdateWorkspaceCompanyProcedure: LevelAdminOnly,

	// Team: read is open to every team member (so technicians can
	// see teammates). Mutations are admin-only.
	teamv1connect.TeamServiceListMembersProcedure:       LevelAuthenticated,
	teamv1connect.TeamServiceInviteMemberProcedure:      LevelAdminOnly,
	teamv1connect.TeamServiceChangeRoleProcedure:        LevelAdminOnly,
	teamv1connect.TeamServiceSuspendMemberProcedure:     LevelAdminOnly,
	teamv1connect.TeamServiceReactivateMemberProcedure:  LevelAdminOnly,

	// Contacts: technicians can manage contacts of customer
	// companies (creating, editing, archiving is part of normal
	// operator work).
	contactsv1connect.ContactsServiceListContactsProcedure:    LevelAuthenticated,
	contactsv1connect.ContactsServiceGetContactProcedure:      LevelAuthenticated,
	contactsv1connect.ContactsServiceCreateContactProcedure:   LevelAuthenticated,
	contactsv1connect.ContactsServiceUpdateContactProcedure:   LevelAuthenticated,
	contactsv1connect.ContactsServiceArchiveContactProcedure:  LevelAuthenticated,
}

// LevelFor returns the level required for the given Connect procedure
// path (e.g. "/gospa.companies.v1.CompaniesService/ListCompanies"). The
// boolean is false when the procedure is not in the policy map; callers
// should treat that as a hard deny rather than a permissive default.
func LevelFor(procedure string) (Level, bool) {
	l, ok := policy[procedure]
	return l, ok
}
