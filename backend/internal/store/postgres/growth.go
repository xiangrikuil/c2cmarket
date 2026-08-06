package postgres

import (
	"context"
	"errors"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/growth"

	"github.com/jackc/pgx/v5"
)

func (s *Store) RecordActivity(ctx context.Context, userID, activityDate string, seenAt time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_activity_daily (user_id, activity_date, first_seen_at)
		VALUES ($1, $2::date, $3)
		ON CONFLICT (user_id, activity_date) DO NOTHING
	`, userID, activityDate, seenAt)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) GrowthOverview(ctx context.Context, asOf time.Time, windowDays int) (growth.Overview, *domain.AppError) {
	if s == nil || s.pool == nil {
		return growth.Overview{}, internalStoreError()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return growth.Overview{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	zone, err := time.LoadLocation(growth.BusinessTimezone)
	if err != nil {
		zone = time.FixedZone(growth.BusinessTimezone, 8*60*60)
	}
	localNow := asOf.In(zone)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, zone)
	windowStart := today.AddDate(0, 0, -(windowDays - 1))
	windowEnd := today.AddDate(0, 0, 1)

	overview := growth.Overview{
		GeneratedAt: asOf.UTC(),
		Timezone:    growth.BusinessTimezone,
		WindowDays:  windowDays,
	}
	if appErr := loadGrowthSummary(ctx, tx, &overview, asOf, today, windowStart, windowEnd); appErr != nil {
		return growth.Overview{}, appErr
	}
	if appErr := loadGrowthRegistrationTrend(ctx, tx, &overview, windowStart, today); appErr != nil {
		return growth.Overview{}, appErr
	}
	if appErr := loadGrowthActivityTrend(ctx, tx, &overview, windowStart, today); appErr != nil {
		return growth.Overview{}, appErr
	}
	if appErr := loadGrowthAttribution(ctx, tx, &overview, windowStart, windowEnd); appErr != nil {
		return growth.Overview{}, appErr
	}
	if appErr := loadGrowthActivation(ctx, tx, &overview, asOf, windowStart, windowEnd); appErr != nil {
		return growth.Overview{}, appErr
	}
	if appErr := loadGrowthRetention(ctx, tx, &overview, windowStart, today); appErr != nil {
		return growth.Overview{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return growth.Overview{}, internalStoreError()
	}
	return overview, nil
}

func loadGrowthSummary(ctx context.Context, q queryer, overview *growth.Overview, asOf, today, windowStart, windowEnd time.Time) *domain.AppError {
	err := q.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE created_at >= $2 AND created_at < $5)::int,
		  count(*) FILTER (WHERE created_at >= $2 - interval '6 days' AND created_at < $5)::int,
		  count(*) FILTER (WHERE created_at >= $2 - interval '29 days' AND created_at < $5)::int,
		  count(*) FILTER (WHERE created_at >= $3 AND created_at < $4)::int,
		  count(*) FILTER (WHERE account_status <> 'archived')::int
		FROM users
		WHERE created_at <= $1
	`, asOf, today.UTC(), windowStart.UTC(), windowEnd.UTC(), windowEnd.UTC()).Scan(
		&overview.Summary.NewUsersToday,
		&overview.Summary.NewUsers7d,
		&overview.Summary.NewUsers30d,
		&overview.Summary.NewUsersInWindow,
		&overview.Summary.CumulativeEffectiveUsers,
	)
	if err != nil {
		return internalStoreError()
	}
	err = q.QueryRow(ctx, `
		SELECT
		  count(DISTINCT user_id) FILTER (WHERE activity_date = $1::date)::int,
		  count(DISTINCT user_id) FILTER (WHERE activity_date BETWEEN $1::date - 6 AND $1::date)::int,
		  count(DISTINCT user_id) FILTER (WHERE activity_date BETWEEN $1::date - 29 AND $1::date)::int
		FROM user_activity_daily
		WHERE activity_date BETWEEN $1::date - 29 AND $1::date
	`, today.Format(time.DateOnly)).Scan(&overview.Summary.DAU, &overview.Summary.WAU, &overview.Summary.MAU)
	if err != nil {
		return internalStoreError()
	}
	err = q.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (
		    WHERE status = 'completed' AND ended_at >= $1 AND ended_at < $2
		  )::int
		FROM carpool_memberships
	`, windowStart.UTC(), windowEnd.UTC()).Scan(&overview.Summary.CompletedCarpoolTransactions)
	if err != nil {
		return internalStoreError()
	}
	err = q.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (
		    WHERE status = 'completed' AND completed_at >= $1 AND completed_at < $2
		  )::int
		FROM api_orders
	`, windowStart.UTC(), windowEnd.UTC()).Scan(&overview.Summary.CompletedAPITransactions)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func loadGrowthRegistrationTrend(ctx context.Context, q queryer, overview *growth.Overview, windowStart, today time.Time) *domain.AppError {
	rows, err := queryRows(ctx, q, `
		WITH dates AS (
		  SELECT generate_series($1::date, $2::date, interval '1 day')::date AS day
		), daily AS (
		  SELECT (created_at AT TIME ZONE 'Asia/Shanghai')::date AS day, count(*)::int AS registrations
		  FROM users
		  WHERE created_at < (($2::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
		  GROUP BY 1
		)
		SELECT dates.day::text,
		       COALESCE(daily.registrations, 0)::int,
		       (
		         SELECT count(*)::int
		         FROM users
		         WHERE created_at < ((dates.day + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
		       )
		FROM dates
		LEFT JOIN daily ON daily.day = dates.day
		ORDER BY dates.day
	`, windowStart.Format(time.DateOnly), today.Format(time.DateOnly))
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	overview.RegistrationTrend = make([]growth.RegistrationTrendPoint, 0, overview.WindowDays)
	for rows.Next() {
		var point growth.RegistrationTrendPoint
		if err := rows.Scan(&point.Date, &point.NewUsers, &point.CumulativeUsers); err != nil {
			return internalStoreError()
		}
		overview.RegistrationTrend = append(overview.RegistrationTrend, point)
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	return nil
}

func loadGrowthActivityTrend(ctx context.Context, q queryer, overview *growth.Overview, windowStart, today time.Time) *domain.AppError {
	rows, err := queryRows(ctx, q, `
		WITH dates AS (
		  SELECT generate_series($1::date, $2::date, interval '1 day')::date AS day
		), daily AS (
		  SELECT activity_date AS day, count(*)::int AS active_users
		  FROM user_activity_daily
		  WHERE activity_date BETWEEN $1::date AND $2::date
		  GROUP BY activity_date
		)
		SELECT dates.day::text, COALESCE(daily.active_users, 0)::int
		FROM dates
		LEFT JOIN daily ON daily.day = dates.day
		ORDER BY dates.day
	`, windowStart.Format(time.DateOnly), today.Format(time.DateOnly))
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	overview.ActivityTrend = make([]growth.ActivityTrendPoint, 0, overview.WindowDays)
	for rows.Next() {
		var point growth.ActivityTrendPoint
		if err := rows.Scan(&point.Date, &point.ActiveUsers); err != nil {
			return internalStoreError()
		}
		overview.ActivityTrend = append(overview.ActivityTrend, point)
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	return nil
}

func loadGrowthAttribution(ctx context.Context, q queryer, overview *growth.Overview, windowStart, windowEnd time.Time) *domain.AppError {
	rows, err := queryRows(ctx, q, `
		SELECT
		  COALESCE(attribution.source_type, 'unknown'),
		  COALESCE(attribution.source, 'unknown'),
		  COALESCE(attribution.medium, ''),
		  COALESCE(attribution.campaign, ''),
		  count(*)::int
		FROM users
		LEFT JOIN user_registration_attributions attribution ON attribution.user_id = users.id
		WHERE users.created_at >= $1 AND users.created_at < $2
		GROUP BY 1, 2, 3, 4
		ORDER BY count(*) DESC, 1, 2, 3, 4
	`, windowStart.UTC(), windowEnd.UTC())
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	groups := make([]growth.AttributionGroup, 0, 10)
	total := 0
	for rows.Next() {
		var group growth.AttributionGroup
		if err := rows.Scan(&group.SourceType, &group.Source, &group.Medium, &group.Campaign, &group.Registrations); err != nil {
			return internalStoreError()
		}
		total += group.Registrations
		groups = append(groups, group)
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	if len(groups) > 10 {
		other := growth.AttributionGroup{SourceType: "other", Source: "other"}
		for _, group := range groups[9:] {
			other.Registrations += group.Registrations
		}
		groups = append(groups[:9], other)
	}
	for index := range groups {
		if total > 0 {
			groups[index].Share = float64(groups[index].Registrations) / float64(total)
		}
	}
	overview.Attribution = groups
	return nil
}

func loadGrowthActivation(ctx context.Context, q queryer, overview *growth.Overview, asOf, windowStart, windowEnd time.Time) *domain.AppError {
	eligibleEnd := asOf.Add(-7 * 24 * time.Hour)
	if eligibleEnd.After(windowEnd) {
		eligibleEnd = windowEnd
	}
	if !eligibleEnd.After(windowStart) {
		return nil
	}
	var median *float64
	err := q.QueryRow(ctx, `
		WITH cohort AS (
		  SELECT id, created_at
		  FROM users
		  WHERE created_at >= $1 AND created_at < $2
		), first_buyer AS (
		  SELECT user_id, min(occurred_at) AS occurred_at
		  FROM (
		    SELECT buyer_user_id AS user_id, created_at AS occurred_at FROM carpool_applications
		    UNION ALL
		    SELECT buyer_user_id AS user_id, created_at AS occurred_at FROM api_orders
		  ) buyer_events
		  GROUP BY user_id
		), first_seller AS (
		  SELECT user_id, min(occurred_at) AS occurred_at
		  FROM (
		    SELECT owner_user_id AS user_id, first_published_at AS occurred_at
		    FROM carpool_listings WHERE first_published_at IS NOT NULL
		    UNION ALL
		    SELECT owner_user_id AS user_id, first_published_at AS occurred_at
		    FROM api_services WHERE first_published_at IS NOT NULL
		  ) seller_events
		  GROUP BY user_id
		), activation AS (
		  SELECT cohort.id,
		         cohort.created_at,
		         first_buyer.occurred_at AS buyer_at,
		         first_seller.occurred_at AS seller_at,
		         LEAST(first_buyer.occurred_at, first_seller.occurred_at) AS activated_at
		  FROM cohort
		  LEFT JOIN first_buyer ON first_buyer.user_id = cohort.id
		  LEFT JOIN first_seller ON first_seller.user_id = cohort.id
		)
		SELECT
		  count(*)::int,
		  count(*) FILTER (WHERE buyer_at BETWEEN created_at AND created_at + interval '7 days')::int,
		  count(*) FILTER (WHERE seller_at BETWEEN created_at AND created_at + interval '7 days')::int,
		  count(*) FILTER (WHERE activated_at BETWEEN created_at AND created_at + interval '7 days')::int,
		  percentile_cont(0.5) WITHIN GROUP (
		    ORDER BY extract(epoch FROM (activated_at - created_at)) / 3600.0
		  ) FILTER (WHERE activated_at BETWEEN created_at AND created_at + interval '7 days')::double precision
		FROM activation
	`, windowStart.UTC(), eligibleEnd.UTC()).Scan(
		&overview.Activation.CohortUsers,
		&overview.Activation.BuyerActivatedUsers,
		&overview.Activation.SellerActivatedUsers,
		&overview.Activation.ActivatedUsers,
		&median,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalStoreError()
	}
	if overview.Activation.CohortUsers > 0 {
		overview.Activation.BuyerActivationRate = ratioPtr(overview.Activation.BuyerActivatedUsers, overview.Activation.CohortUsers)
		overview.Activation.SellerActivationRate = ratioPtr(overview.Activation.SellerActivatedUsers, overview.Activation.CohortUsers)
		overview.Activation.ActivationRate = ratioPtr(overview.Activation.ActivatedUsers, overview.Activation.CohortUsers)
	}
	overview.Summary.ActivatedUsers = overview.Activation.ActivatedUsers
	overview.Summary.ActivationRate = overview.Activation.ActivationRate
	overview.Summary.MedianActivationHours = median
	return nil
}

func loadGrowthRetention(ctx context.Context, q queryer, overview *growth.Overview, windowStart, today time.Time) *domain.AppError {
	rows, err := queryRows(ctx, q, `
		WITH dates AS (
		  SELECT generate_series($1::date, $2::date, interval '1 day')::date AS cohort_date
		), cohort_users AS (
		  SELECT id, (created_at AT TIME ZONE 'Asia/Shanghai')::date AS cohort_date
		  FROM users
		  WHERE created_at >= ($1::date::timestamp AT TIME ZONE 'Asia/Shanghai')
		    AND created_at < (($2::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
		)
		SELECT dates.cohort_date::text,
		       count(DISTINCT cohort_users.id)::int,
		       CASE WHEN dates.cohort_date <= $2::date - 2
		         THEN count(DISTINCT day_one.user_id)::int
		       END,
		       CASE WHEN dates.cohort_date <= $2::date - 8
		         THEN count(DISTINCT day_seven.user_id)::int
		       END
		FROM dates
		LEFT JOIN cohort_users ON cohort_users.cohort_date = dates.cohort_date
		LEFT JOIN user_activity_daily day_one
		  ON day_one.user_id = cohort_users.id
		 AND day_one.activity_date = dates.cohort_date + 1
		LEFT JOIN user_activity_daily day_seven
		  ON day_seven.user_id = cohort_users.id
		 AND day_seven.activity_date = dates.cohort_date + 7
		GROUP BY dates.cohort_date
		ORDER BY dates.cohort_date
	`, windowStart.Format(time.DateOnly), today.Format(time.DateOnly))
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	overview.RetentionCohorts = make([]growth.RetentionCohort, 0, overview.WindowDays)
	var d1Users, d1Retained, d7Users, d7Retained int
	for rows.Next() {
		var cohort growth.RetentionCohort
		if err := rows.Scan(&cohort.CohortDate, &cohort.RegisteredUsers, &cohort.D1RetainedUsers, &cohort.D7RetainedUsers); err != nil {
			return internalStoreError()
		}
		if cohort.D1RetainedUsers != nil {
			if cohort.RegisteredUsers > 0 {
				cohort.D1Rate = ratioPtr(*cohort.D1RetainedUsers, cohort.RegisteredUsers)
				d1Users += cohort.RegisteredUsers
				d1Retained += *cohort.D1RetainedUsers
			}
		}
		if cohort.D7RetainedUsers != nil {
			if cohort.RegisteredUsers > 0 {
				cohort.D7Rate = ratioPtr(*cohort.D7RetainedUsers, cohort.RegisteredUsers)
				d7Users += cohort.RegisteredUsers
				d7Retained += *cohort.D7RetainedUsers
			}
		}
		overview.RetentionCohorts = append(overview.RetentionCohorts, cohort)
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	if d1Users > 0 {
		overview.Summary.D1RetentionRate = ratioPtr(d1Retained, d1Users)
	}
	if d7Users > 0 {
		overview.Summary.D7RetentionRate = ratioPtr(d7Retained, d7Users)
	}
	return nil
}

func ratioPtr(numerator, denominator int) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}
