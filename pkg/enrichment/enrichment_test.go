package enrichment

import (
	"strings"
	"testing"
	"time"
)

func TestValidateAcceptsCompleteFeed(t *testing.T) {
	if err := Validate(nil); err != nil {
		t.Fatalf("Validate(nil) = %v, want nil", err)
	}
	if err := Validate(validTestFeed()); err != nil {
		t.Fatalf("Validate(valid feed) = %v, want nil", err)
	}
}

func TestValidateNamedWrapsErrors(t *testing.T) {
	feed := validTestFeed()
	feed.Derivation.Type = "bad"

	err := ValidateNamed("feed", "example", feed)
	if err == nil {
		t.Fatal("ValidateNamed() error = nil, want error")
	}
	if got := err.Error(); !strings.Contains(got, `feed "example" enrichment: derivation.type "bad" is invalid`) {
		t.Fatalf("ValidateNamed() error = %q", got)
	}
}

func TestValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Feed)
		want string
	}{
		{
			name: "schema version",
			edit: func(feed *Feed) { feed.EnrichmentSchemaVersion = 1 },
			want: "schema version = 1, want 2",
		},
		{
			name: "run_at",
			edit: func(feed *Feed) { feed.RunAt = "not-time" },
			want: `run_at "not-time" is not RFC3339`,
		},
		{
			name: "role",
			edit: func(feed *Feed) { feed.Roles[0].Role = "bad" },
			want: `roles[0].role "bad" is invalid`,
		},
		{
			name: "role name",
			edit: func(feed *Feed) { feed.Roles[0].Name = "" },
			want: "roles[0].name is empty",
		},
		{
			name: "organization type",
			edit: func(feed *Feed) { feed.Roles[0].OrganizationType = stringPtr("bad") },
			want: `roles[0].organization_type "bad" is invalid`,
		},
		{
			name: "derivation type",
			edit: func(feed *Feed) { feed.Derivation.Type = "bad" },
			want: `derivation.type "bad" is invalid`,
		},
		{
			name: "derivation description",
			edit: func(feed *Feed) { feed.Derivation.Description = "" },
			want: "derivation.description is empty",
		},
		{
			name: "source identifier",
			edit: func(feed *Feed) { feed.Derivation.SourceFeeds[0].Identifier = "" },
			want: "derivation.source_feeds[0].identifier is empty",
		},
		{
			name: "source relationship",
			edit: func(feed *Feed) { feed.Derivation.SourceFeeds[0].Relationship = stringPtr("bad") },
			want: `derivation.source_feeds[0].relationship "bad" is invalid`,
		},
		{
			name: "update frequency",
			edit: func(feed *Feed) { feed.UpdateFrequency.Frequency = stringPtr("daily") },
			want: `update_frequency.frequency "daily" is invalid`,
		},
		{
			name: "primary method",
			edit: func(feed *Feed) { feed.DetectionClassification.PrimaryMethod = "bad" },
			want: `detection_classification.primary_method "bad" is invalid`,
		},
		{
			name: "detection description",
			edit: func(feed *Feed) { feed.DetectionClassification.Description = "" },
			want: "detection_classification.description is empty",
		},
		{
			name: "secondary method",
			edit: func(feed *Feed) { feed.DetectionClassification.SecondaryMethods[0] = "bad" },
			want: `detection_classification.secondary_methods[0] "bad" is invalid`,
		},
		{
			name: "current status",
			edit: func(feed *Feed) { feed.CurrentStatus.State = "bad" },
			want: `current_status.state "bad" is invalid`,
		},
		{
			name: "current status description",
			edit: func(feed *Feed) { feed.CurrentStatus.Description = "" },
			want: "current_status.description is empty",
		},
		{
			name: "source consulted",
			edit: func(feed *Feed) { feed.SourcesConsulted[0].URL = "" },
			want: "sources_consulted[0].url is empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			feed := validTestFeed()
			tc.edit(feed)
			err := Validate(feed)
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if got := err.Error(); !strings.Contains(got, tc.want) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.want)
			}
		})
	}
}

func TestCloneCopiesMutableFields(t *testing.T) {
	original := validTestFeed()
	clone := Clone(original)
	if clone == nil {
		t.Fatal("Clone() = nil")
	}

	clone.Roles[0].Name = "changed"
	clone.Derivation.SourceFeeds[0].Identifier = "changed"
	clone.ListingPolicy.Criteria[0] = "changed"
	clone.UnlistingPolicy.Criteria[0] = "changed"
	clone.UnlistRequest.Email = stringPtr("changed@example.test")
	clone.UpdateFrequency.Frequency = stringPtr("2h")
	clone.DetectionClassification.SecondaryMethods[0] = "active_scanning"
	clone.ScopeAndIntent.IntendedFor[0] = "changed"
	clone.ScopeAndIntent.NotIntendedFor[0] = "changed"
	clone.CurrentStatus.Successor.Name = stringPtr("changed")
	clone.SourcesConsulted[0].URL = "https://changed.example.test/"

	if original.Roles[0].Name == "changed" {
		t.Fatal("Clone() shared Roles slice with original")
	}
	if original.Derivation.SourceFeeds[0].Identifier == "changed" {
		t.Fatal("Clone() shared Derivation.SourceFeeds slice with original")
	}
	if original.ListingPolicy.Criteria[0] == "changed" {
		t.Fatal("Clone() shared ListingPolicy.Criteria slice with original")
	}
	if original.UnlistingPolicy.Criteria[0] == "changed" {
		t.Fatal("Clone() shared UnlistingPolicy.Criteria slice with original")
	}
	if *original.UnlistRequest.Email == "changed@example.test" {
		t.Fatal("Clone() shared UnlistRequest pointer with original")
	}
	if *original.UpdateFrequency.Frequency == "2h" {
		t.Fatal("Clone() shared UpdateFrequency pointer with original")
	}
	if original.DetectionClassification.SecondaryMethods[0] == "active_scanning" {
		t.Fatal("Clone() shared DetectionClassification.SecondaryMethods slice with original")
	}
	if original.ScopeAndIntent.IntendedFor[0] == "changed" {
		t.Fatal("Clone() shared ScopeAndIntent.IntendedFor slice with original")
	}
	if original.ScopeAndIntent.NotIntendedFor[0] == "changed" {
		t.Fatal("Clone() shared ScopeAndIntent.NotIntendedFor slice with original")
	}
	if *original.CurrentStatus.Successor.Name == "changed" {
		t.Fatal("Clone() shared CurrentStatus.Successor pointer with original")
	}
	if original.SourcesConsulted[0].URL == "https://changed.example.test/" {
		t.Fatal("Clone() shared SourcesConsulted slice with original")
	}

	if Clone(nil) != nil {
		t.Fatal("Clone(nil) should return nil")
	}
}

func TestStringValue(t *testing.T) {
	if got := StringValue(nil); got != "" {
		t.Fatalf("StringValue(nil) = %q, want empty", got)
	}
	if got := StringValue(stringPtr("value")); got != "value" {
		t.Fatalf("StringValue() = %q, want value", got)
	}
}

func validTestFeed() *Feed {
	return &Feed{
		EnrichmentSchemaVersion: SchemaVersion,
		RunAt:                   time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		OfficialName:            stringPtr("Example Feed"),
		OfficialURL:             stringPtr("https://example.test/feed"),
		ShortDescription:        stringPtr("Short description."),
		LongDescription:         stringPtr("Long description."),
		Roles: []Role{
			{
				Role:             "maintainer",
				Name:             "Example Maintainer",
				OrganizationType: stringPtr("non_profit"),
				OfficialURL:      stringPtr("https://example.test/"),
				ContactEmail:     stringPtr("security@example.test"),
				BasedIn:          stringPtr("ZZ"),
				ActiveSince:      stringPtr("2020"),
				Notes:            stringPtr("Example notes."),
			},
		},
		Derivation: Derivation{
			Type:        "original",
			Description: "Original feed.",
			SourceFeeds: []SourceFeed{
				{
					Identifier:   "example",
					Relationship: stringPtr("mirror"),
					Notes:        stringPtr("Example source notes."),
				},
			},
		},
		ListingPolicy: &Policy{
			Summary:  "Listing summary.",
			Criteria: []string{"listed when observed"},
		},
		UnlistingPolicy: &Policy{
			Summary:  "Unlisting summary.",
			Criteria: []string{"removed when stale"},
		},
		UnlistRequest: &UnlistRequest{
			URL:          stringPtr("https://example.test/unlist"),
			Email:        stringPtr("unlist@example.test"),
			Instructions: stringPtr("Send evidence."),
		},
		UpdateFrequency: &UpdateFrequency{
			Frequency:     stringPtr("1h"),
			HumanReadable: stringPtr("Hourly"),
		},
		DetectionClassification: DetectionClassification{
			PrimaryMethod:    "honeypot",
			SecondaryMethods: []string{"malware_analysis"},
			Description:      "Observed by honeypot and confirmed with malware analysis.",
		},
		ScopeAndIntent: &ScopeAndIntent{
			Description:    stringPtr("Scope description."),
			IntendedFor:    []string{"blocking"},
			NotIntendedFor: []string{"attribution"},
		},
		License: stringPtr("public feed"),
		Redistribution: Redistribution{
			Allowed:              boolPtr(true),
			CommercialUseAllowed: boolPtr(true),
			AttributionRequired:  stringPtr("yes"),
			Terms:                stringPtr("Attribution required."),
		},
		CurrentStatus: CurrentStatus{
			State:       "active",
			Description: "Currently active.",
			Successor: &Successor{
				Name: stringPtr("Successor"),
				URL:  stringPtr("https://example.test/successor"),
			},
			AnnouncementDate: stringPtr("2026-06-03"),
		},
		Community: Community{
			Awards:     stringPtr("None."),
			Criticism:  stringPtr("None."),
			Engagement: stringPtr("Public issue tracker."),
		},
		SourcesConsulted: []SourceConsulted{
			{
				URL:            "https://example.test/source",
				DocumentDate:   stringPtr("2026-06-01"),
				ValidationDate: stringPtr("2026-06-03"),
			},
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
