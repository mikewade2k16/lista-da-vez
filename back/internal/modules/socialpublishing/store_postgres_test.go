package socialpublishing

import (
	"errors"
	"strings"
	"testing"
)

func TestListPostsOrderByUsesFixedStableClauses(t *testing.T) {
	created, err := listPostsOrderBy(PostListOrderCreated)
	if err != nil {
		t.Fatalf("listPostsOrderBy(created) error = %v", err)
	}
	if created != "p.created_at desc, p.id desc" {
		t.Fatalf("created order = %q", created)
	}

	scheduled, err := listPostsOrderBy(PostListOrderScheduled)
	if err != nil {
		t.Fatalf("listPostsOrderBy(scheduled) error = %v", err)
	}
	if scheduled != "p.scheduled_for asc nulls last, p.id asc" {
		t.Fatalf("scheduled order = %q", scheduled)
	}

	if _, err := listPostsOrderBy("p.caption"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unsafe order error = %v, want ErrInvalidInput", err)
	}
}

func TestSummaryAndAnalyticsQueriesAreTenantScoped(t *testing.T) {
	for name, query := range map[string]string{
		"summary counts":   summaryCountsQuery,
		"summary upcoming": summaryUpcomingQuery,
		"analytics":        listAnalyticsQuery,
	} {
		if !strings.Contains(query, "account_id = $1::uuid") {
			t.Fatalf("%s query is not tenant-scoped: %s", name, query)
		}
	}
	if !strings.Contains(listAnalyticsQuery, "post_id = any($2::text[]::uuid[])") {
		t.Fatalf("analytics query does not filter typed post IDs: %s", listAnalyticsQuery)
	}
	if !strings.Contains(summaryUpcomingQuery, "p.scheduled_for >= $2") ||
		!strings.Contains(summaryUpcomingQuery, "limit 10") {
		t.Fatalf("summary upcoming query is not bounded to future schedules: %s", summaryUpcomingQuery)
	}
}

func TestScheduleQueryPreservesTargetAndAtMostOnceState(t *testing.T) {
	required := []string{
		"p.connection_id is null or p.connection_id = $4::uuid",
		"active_connection.ig_user_id = target_connection.ig_user_id",
		"p.publish_attempted_at is null",
		"external_creation_id = ''",
	}
	for _, fragment := range required {
		if !strings.Contains(schedulePostUpdateQuery, fragment) {
			t.Fatalf("schedule query missing %q: %s", fragment, schedulePostUpdateQuery)
		}
	}
	if strings.Contains(schedulePostUpdateQuery, "publish_attempted_at = null") {
		t.Fatalf("schedule query must never erase an attempted publish: %s", schedulePostUpdateQuery)
	}
}

func TestAmbiguousPublishOutcomeCannotBeDowngradedByFailure(t *testing.T) {
	for _, query := range []string{protectPublishOutcomeQuery, markPublishFailedQuery} {
		if !strings.Contains(query, "publish_attempted_at is not null") ||
			!strings.Contains(query, "external_media_id = ''") ||
			!strings.Contains(query, "publish_outcome_unknown") {
			t.Fatalf("query does not preserve ambiguous publish outcome: %s", query)
		}
	}
}

func TestPublishedAnalyticsSyncQueryHasNoSilentLimit(t *testing.T) {
	if !strings.Contains(listPublishedPostIDsQuery, "account_id = $1::uuid") {
		t.Fatalf("published posts query is not tenant-scoped: %s", listPublishedPostIDsQuery)
	}
	if strings.Contains(strings.ToLower(listPublishedPostIDsQuery), "limit") {
		t.Fatalf("published posts query silently truncates analytics sync: %s", listPublishedPostIDsQuery)
	}
}

func TestAnalyticsSnapshotInsertToleratesLegacyAndJobKeyUniques(t *testing.T) {
	if !strings.Contains(analyticsSnapshotInsertQuery, "job_key") {
		t.Fatalf("snapshot insert does not persist job_key: %s", analyticsSnapshotInsertQuery)
	}
	if !strings.Contains(analyticsSnapshotInsertQuery, "on conflict do nothing") {
		t.Fatalf("snapshot insert does not tolerate both unique constraints: %s", analyticsSnapshotInsertQuery)
	}
	if strings.Contains(analyticsSnapshotInsertQuery, "on conflict (") {
		t.Fatalf("snapshot insert targets only one unique constraint: %s", analyticsSnapshotInsertQuery)
	}
}

func TestPublishingScopeQueriesPreserveClientAndMembershipBoundaries(t *testing.T) {
	for name, query := range map[string]string{
		"platform": platformPublishingScopeQuery,
		"account":  accountPublishingScopeQuery,
	} {
		for _, fragment := range []string{
			"is_active = true",
			"is_agency = false",
			"enabled_module.module_id = 'social_publishing'",
			"enabled_module.enabled = true",
		} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s scope query missing %q: %s", name, fragment, query)
			}
		}
	}
	for _, fragment := range []string{
		"current_account.organization_id is not null",
		"client.organization_id = current_account.organization_id",
		"organization_member.user_id = $2::uuid",
		"organization_member.org_role = 'agency_owner'",
		"account_member.user_id = $2::uuid",
		"account_member.account_id = client.id",
		"account_member.is_active = true",
	} {
		if !strings.Contains(accountPublishingScopeQuery, fragment) {
			t.Fatalf("account scope query missing %q: %s", fragment, accountPublishingScopeQuery)
		}
	}
}

func TestPortfolioAggregateQueryIsBoundedAndNeverSelectsSecrets(t *testing.T) {
	for _, fragment := range []string{
		"account.id = any($1::text[]::uuid[])",
		"post.scheduled_for >= $2::timestamptz",
		"account.is_active = true",
		"account.is_agency = false",
		"enabled_module.module_id = 'social_publishing'",
		"count(post.id) filter (where post.status = 'scheduled')",
		"sum(analytics.reach)",
		"sum(analytics.total_interactions)",
	} {
		if !strings.Contains(portfolioAggregateQuery, fragment) {
			t.Fatalf("portfolio query missing %q: %s", fragment, portfolioAggregateQuery)
		}
	}
	lower := strings.ToLower(portfolioAggregateQuery)
	for _, forbidden := range []string{"access_token", "ciphertext", "token_last4"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("portfolio query selects forbidden field %q: %s", forbidden, portfolioAggregateQuery)
		}
	}
}
