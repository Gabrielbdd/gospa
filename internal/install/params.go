package install

import (
	"github.com/Gabrielbdd/gospa/db/sqlc"
)

func mapProvisioningParams(in Input) sqlc.MarkWorkspaceProvisioningParams {
	return sqlc.MarkWorkspaceProvisioningParams{
		Name:         in.WorkspaceName,
		Slug:         in.WorkspaceSlug,
		Timezone:     in.Timezone,
		CurrencyCode: in.CurrencyCode,
	}
}
