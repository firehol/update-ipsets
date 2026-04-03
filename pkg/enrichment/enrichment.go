package enrichment

import (
	"fmt"
	"regexp"
	"slices"
	"time"
)

const SchemaVersion = 2

var compactFrequencyRE = regexp.MustCompile(`^\d+[mhdw]$`)

var validRoleValues = []string{
	"maintainer",
	"publisher",
	"aggregator",
	"source_contributor",
	"original_author",
	"successor",
}

var validOrganizationTypes = []string{
	"non_profit",
	"commercial",
	"research_institution",
	"government",
	"individual",
	"informal_collective",
	"unknown",
}

var validDerivationTypes = []string{
	"original",
	"derivative",
	"extraction",
	"partial_mirror",
	"aggregate_merge",
	"reformat",
	"fork",
	"unknown",
}

var validSourceRelationships = []string{
	"subset",
	"superset",
	"filtered",
	"enriched",
	"mirror",
	"aggregate_component",
	"fork",
}

var validDetectionMethods = []string{
	"honeypot",
	"network_telescope",
	"active_scanning",
	"user_submission",
	"malware_analysis",
	"reputation_aggregation",
	"policy_assignment",
	"commercial_threat_intel",
	"mixed",
	"unknown",
}

var validSecondaryDetectionMethods = []string{
	"honeypot",
	"network_telescope",
	"active_scanning",
	"user_submission",
	"malware_analysis",
	"reputation_aggregation",
	"policy_assignment",
	"commercial_threat_intel",
}

var validCurrentStatusStates = []string{
	"active",
	"discontinued",
	"merged",
	"forked",
	"reformatted",
	"altered_scope",
	"unknown",
}

type Feed struct {
	EnrichmentSchemaVersion int                     `yaml:"enrichment_schema_version" json:"enrichment_schema_version"`
	RunAt                   string                  `yaml:"run_at" json:"run_at"`
	OfficialName            *string                 `yaml:"official_name" json:"official_name"`
	OfficialURL             *string                 `yaml:"official_url" json:"official_url"`
	ShortDescription        *string                 `yaml:"short_description" json:"short_description"`
	LongDescription         *string                 `yaml:"long_description" json:"long_description"`
	Roles                   []Role                  `yaml:"roles" json:"roles"`
	Derivation              Derivation              `yaml:"derivation" json:"derivation"`
	ListingPolicy           *Policy                 `yaml:"listing_policy" json:"listing_policy"`
	UnlistingPolicy         *Policy                 `yaml:"unlisting_policy" json:"unlisting_policy"`
	UnlistRequest           *UnlistRequest          `yaml:"unlist_request" json:"unlist_request"`
	UpdateFrequency         *UpdateFrequency        `yaml:"update_frequency" json:"update_frequency"`
	DetectionClassification DetectionClassification `yaml:"detection_classification" json:"detection_classification"`
	ScopeAndIntent          *ScopeAndIntent         `yaml:"scope_and_intent" json:"scope_and_intent"`
	License                 *string                 `yaml:"license" json:"license"`
	Redistribution          Redistribution          `yaml:"redistribution" json:"redistribution"`
	CurrentStatus           CurrentStatus           `yaml:"current_status" json:"current_status"`
	Community               Community               `yaml:"community" json:"community"`
	SourcesConsulted        []SourceConsulted       `yaml:"sources_consulted" json:"sources_consulted"`
}

type Role struct {
	Role             string  `yaml:"role" json:"role"`
	Name             string  `yaml:"name" json:"name"`
	OrganizationType *string `yaml:"organization_type" json:"organization_type"`
	OfficialURL      *string `yaml:"official_url" json:"official_url"`
	ContactEmail     *string `yaml:"contact_email" json:"contact_email"`
	BasedIn          *string `yaml:"based_in" json:"based_in"`
	ActiveSince      *string `yaml:"active_since" json:"active_since"`
	Notes            *string `yaml:"notes" json:"notes"`
}

type Derivation struct {
	Type        string       `yaml:"type" json:"type"`
	Description string       `yaml:"description" json:"description"`
	SourceFeeds []SourceFeed `yaml:"source_feeds" json:"source_feeds"`
}

type SourceFeed struct {
	Identifier   string  `yaml:"identifier" json:"identifier"`
	Relationship *string `yaml:"relationship" json:"relationship"`
	Notes        *string `yaml:"notes" json:"notes"`
}

type Policy struct {
	Summary  string   `yaml:"summary" json:"summary"`
	Criteria []string `yaml:"criteria" json:"criteria"`
}

type UnlistRequest struct {
	URL          *string `yaml:"url" json:"url"`
	Email        *string `yaml:"email" json:"email"`
	Instructions *string `yaml:"instructions" json:"instructions"`
}

type UpdateFrequency struct {
	Frequency     *string `yaml:"frequency" json:"frequency"`
	HumanReadable *string `yaml:"human_readable" json:"human_readable"`
}

type DetectionClassification struct {
	PrimaryMethod    string   `yaml:"primary_method" json:"primary_method"`
	SecondaryMethods []string `yaml:"secondary_methods" json:"secondary_methods"`
	Description      string   `yaml:"description" json:"description"`
}

type ScopeAndIntent struct {
	Description    *string  `yaml:"description" json:"description"`
	IntendedFor    []string `yaml:"intended_for" json:"intended_for"`
	NotIntendedFor []string `yaml:"not_intended_for" json:"not_intended_for"`
}

type Redistribution struct {
	Allowed              *bool   `yaml:"allowed" json:"allowed"`
	CommercialUseAllowed *bool   `yaml:"commercial_use_allowed" json:"commercial_use_allowed"`
	AttributionRequired  *string `yaml:"attribution_required" json:"attribution_required"`
	Terms                *string `yaml:"terms" json:"terms"`
}

type CurrentStatus struct {
	State            string     `yaml:"state" json:"state"`
	Description      string     `yaml:"description" json:"description"`
	Successor        *Successor `yaml:"successor" json:"successor"`
	AnnouncementDate *string    `yaml:"announcement_date" json:"announcement_date"`
}

type Successor struct {
	Name *string `yaml:"name" json:"name"`
	URL  *string `yaml:"url" json:"url"`
}

type Community struct {
	Awards     *string `yaml:"awards" json:"awards"`
	Criticism  *string `yaml:"criticism" json:"criticism"`
	Engagement *string `yaml:"engagement" json:"engagement"`
}

type SourceConsulted struct {
	URL            string  `yaml:"url" json:"url"`
	DocumentDate   *string `yaml:"document_date" json:"document_date"`
	ValidationDate *string `yaml:"validation_date" json:"validation_date"`
}

func Validate(feed *Feed) error {
	if feed == nil {
		return nil
	}
	if feed.EnrichmentSchemaVersion != SchemaVersion {
		return fmt.Errorf("schema version = %d, want %d", feed.EnrichmentSchemaVersion, SchemaVersion)
	}
	if _, err := time.Parse(time.RFC3339, feed.RunAt); err != nil {
		return fmt.Errorf("run_at %q is not RFC3339: %w", feed.RunAt, err)
	}
	if err := validateRoles(feed.Roles); err != nil {
		return err
	}
	if err := validateDerivation(feed.Derivation); err != nil {
		return err
	}
	if err := validateUpdateFrequency(feed.UpdateFrequency); err != nil {
		return err
	}
	if err := validateDetection(feed.DetectionClassification); err != nil {
		return err
	}
	if err := validateCurrentStatus(feed.CurrentStatus); err != nil {
		return err
	}
	if err := validateSourcesConsulted(feed.SourcesConsulted); err != nil {
		return err
	}
	return nil
}

func ValidateNamed(kind, name string, feed *Feed) error {
	if err := Validate(feed); err != nil {
		return fmt.Errorf("%s %q enrichment: %w", kind, name, err)
	}
	return nil
}

func Clone(in *Feed) *Feed {
	if in == nil {
		return nil
	}
	out := *in
	out.Roles = append([]Role(nil), in.Roles...)
	out.Derivation = cloneDerivation(in.Derivation)
	out.ListingPolicy = clonePolicy(in.ListingPolicy)
	out.UnlistingPolicy = clonePolicy(in.UnlistingPolicy)
	out.UnlistRequest = cloneUnlistRequest(in.UnlistRequest)
	out.UpdateFrequency = cloneUpdateFrequency(in.UpdateFrequency)
	out.DetectionClassification.SecondaryMethods = append([]string(nil), in.DetectionClassification.SecondaryMethods...)
	out.ScopeAndIntent = cloneScopeAndIntent(in.ScopeAndIntent)
	out.CurrentStatus = cloneCurrentStatus(in.CurrentStatus)
	out.SourcesConsulted = append([]SourceConsulted(nil), in.SourcesConsulted...)
	return &out
}

func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validateRoles(roles []Role) error {
	for i, role := range roles {
		if !slices.Contains(validRoleValues, role.Role) {
			return fmt.Errorf("roles[%d].role %q is invalid", i, role.Role)
		}
		if role.Name == "" {
			return fmt.Errorf("roles[%d].name is empty", i)
		}
		if role.OrganizationType != nil && !slices.Contains(validOrganizationTypes, *role.OrganizationType) {
			return fmt.Errorf("roles[%d].organization_type %q is invalid", i, *role.OrganizationType)
		}
	}
	return nil
}

func validateDerivation(derivation Derivation) error {
	if !slices.Contains(validDerivationTypes, derivation.Type) {
		return fmt.Errorf("derivation.type %q is invalid", derivation.Type)
	}
	if derivation.Description == "" {
		return fmt.Errorf("derivation.description is empty")
	}
	for i, source := range derivation.SourceFeeds {
		if source.Identifier == "" {
			return fmt.Errorf("derivation.source_feeds[%d].identifier is empty", i)
		}
		if source.Relationship != nil && !slices.Contains(validSourceRelationships, *source.Relationship) {
			return fmt.Errorf("derivation.source_feeds[%d].relationship %q is invalid", i, *source.Relationship)
		}
	}
	return nil
}

func validateUpdateFrequency(updateFrequency *UpdateFrequency) error {
	if updateFrequency == nil || updateFrequency.Frequency == nil {
		return nil
	}
	if !compactFrequencyRE.MatchString(*updateFrequency.Frequency) {
		return fmt.Errorf("update_frequency.frequency %q is invalid", *updateFrequency.Frequency)
	}
	return nil
}

func validateDetection(detection DetectionClassification) error {
	if !slices.Contains(validDetectionMethods, detection.PrimaryMethod) {
		return fmt.Errorf("detection_classification.primary_method %q is invalid", detection.PrimaryMethod)
	}
	if detection.Description == "" {
		return fmt.Errorf("detection_classification.description is empty")
	}
	for i, method := range detection.SecondaryMethods {
		if !slices.Contains(validSecondaryDetectionMethods, method) {
			return fmt.Errorf("detection_classification.secondary_methods[%d] %q is invalid", i, method)
		}
	}
	return nil
}

func validateCurrentStatus(status CurrentStatus) error {
	if !slices.Contains(validCurrentStatusStates, status.State) {
		return fmt.Errorf("current_status.state %q is invalid", status.State)
	}
	if status.Description == "" {
		return fmt.Errorf("current_status.description is empty")
	}
	return nil
}

func validateSourcesConsulted(sources []SourceConsulted) error {
	for i, source := range sources {
		if source.URL == "" {
			return fmt.Errorf("sources_consulted[%d].url is empty", i)
		}
	}
	return nil
}

func cloneDerivation(in Derivation) Derivation {
	out := in
	out.SourceFeeds = append([]SourceFeed(nil), in.SourceFeeds...)
	return out
}

func clonePolicy(in *Policy) *Policy {
	if in == nil {
		return nil
	}
	out := *in
	out.Criteria = append([]string(nil), in.Criteria...)
	return &out
}

func cloneUnlistRequest(in *UnlistRequest) *UnlistRequest {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneUpdateFrequency(in *UpdateFrequency) *UpdateFrequency {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneScopeAndIntent(in *ScopeAndIntent) *ScopeAndIntent {
	if in == nil {
		return nil
	}
	out := *in
	out.IntendedFor = append([]string(nil), in.IntendedFor...)
	out.NotIntendedFor = append([]string(nil), in.NotIntendedFor...)
	return &out
}

func cloneCurrentStatus(in CurrentStatus) CurrentStatus {
	out := in
	if in.Successor != nil {
		successor := *in.Successor
		out.Successor = &successor
	}
	return out
}
