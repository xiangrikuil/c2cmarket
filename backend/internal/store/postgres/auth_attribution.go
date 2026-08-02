package postgres

import (
	"context"
	"strings"
	"time"

	"c2c-market/backend/internal/module/auth"

	"github.com/jackc/pgx/v5"
)

func insertRegistrationAttribution(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	registrationMethod string,
	attribution auth.RegistrationAttribution,
	now time.Time,
) error {
	registrationMethod = strings.TrimSpace(registrationMethod)
	if registrationMethod == "" {
		registrationMethod = "unknown"
	}

	if attribution.SourceType == auth.RegistrationSourceUnknown {
		attribution = auth.RegistrationAttribution{
			SourceType:  auth.RegistrationSourceUnknown,
			Source:      auth.RegistrationSourceUnknown,
			LandingPath: "/",
		}
	} else {
		attribution = auth.NormalizeRegistrationAttribution(attribution)
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_registration_attributions (
		  user_id,
		  registration_method,
		  source_type,
		  source,
		  medium,
		  campaign,
		  referrer_host,
		  landing_path,
		  captured_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO NOTHING
	`,
		userID,
		registrationMethod,
		attribution.SourceType,
		attribution.Source,
		nullText(attribution.Medium),
		nullText(attribution.Campaign),
		nullText(attribution.ReferrerHost),
		attribution.LandingPath,
		now,
	)
	return err
}
