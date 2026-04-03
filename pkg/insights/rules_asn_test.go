package insights

import "testing"

func TestRuleBogonPresent_Fires(t *testing.T) {
	snap := buildSnap().withBogonShare(0.001).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "bogon_present") {
		t.Fatalf("expected bogon_present to fire at 0.1%%; got %v", insightCodes(got))
	}
}

func TestRuleBogonPresent_SuppressedOnZero(t *testing.T) {
	snap := buildSnap().withBogonShare(0).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "bogon_present") {
		t.Fatalf("expected bogon_present silent at 0; got %v", insightCodes(got))
	}
}

func TestRuleBogonPresent_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withTotalIPs(50).withBogonShare(0.10).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "bogon_present") {
		t.Fatalf("expected bogon_present silent on 50 IPs; got %v", insightCodes(got))
	}
}

func TestRuleInfrastructurePresent_Fires(t *testing.T) {
	snap := buildSnap().withInfraIPs(100).withInfraShare(0.0001).withInfraTiers([]InfraTier{
		{Tier: "soft", IPs: 100, Share: 0.0001, Providers: 1},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "infrastructure_present") {
		t.Fatalf("expected infrastructure_present to fire at 0.01%%; got %v", insightCodes(got))
	}
}

func TestRuleInfrastructurePresent_HardTierFiresOnSmallFeed(t *testing.T) {
	snap := buildSnap().
		withTotalIPs(10).
		withInfraIPs(1).
		withInfraShare(0.10).
		withInfraTiers([]InfraTier{{Tier: "hard", IPs: 1, Share: 0.10, Providers: 1}}).
		toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "infrastructure_present") {
		t.Fatalf("expected hard-tier infrastructure_present on small feed; got %v", insightCodes(got))
	}
}

func TestRuleInfrastructurePresent_SuppressedOnZero(t *testing.T) {
	snap := buildSnap().withInfraShare(0).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "infrastructure_present") {
		t.Fatalf("expected infrastructure_present silent at 0; got %v", insightCodes(got))
	}
}

func TestRuleInfrastructurePresent_SuppressedBySample(t *testing.T) {
	snap := buildSnap().
		withTotalIPs(50).
		withInfraIPs(1).
		withInfraShare(0.02).
		withInfraTiers([]InfraTier{{Tier: "contextual", IPs: 1, Share: 0.02, Providers: 1}}).
		toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "infrastructure_present") {
		t.Fatalf("expected infrastructure_present silent on 50 IPs; got %v", insightCodes(got))
	}
}

func TestRuleInfrastructurePresent_SuppressedBelowSoftContextThreshold(t *testing.T) {
	snap := buildSnap().
		withInfraIPs(1).
		withInfraShare(0.000001).
		withInfraTiers([]InfraTier{{Tier: "contextual", IPs: 1, Share: 0.000001, Providers: 1}}).
		toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "infrastructure_present") {
		t.Fatalf("expected infrastructure_present silent below soft/contextual threshold; got %v", insightCodes(got))
	}
}
