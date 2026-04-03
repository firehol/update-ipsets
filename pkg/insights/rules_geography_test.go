package insights

import "testing"

func TestRuleCountryConcentrated_Fires(t *testing.T) {
	// top3 = 0.40+0.20+0.15 = 0.75 > 0.70 threshold.
	snap := buildSnap().withTopCountries([]CountryShare{
		{Code: "FR", Name: "France", IPs: 400_000, Share: 0.40},
		{Code: "DE", Name: "Germany", IPs: 200_000, Share: 0.20},
		{Code: "RU", Name: "Russia", IPs: 150_000, Share: 0.15},
		{Code: "US", Name: "United States", IPs: 150_000, Share: 0.15},
		{Code: "BR", Name: "Brazil", IPs: 100_000, Share: 0.10},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "country_concentrated") {
		t.Fatalf("expected country_concentrated to fire on 75%% top3; got %v", insightCodes(got))
	}
}

func TestRuleCountryConcentrated_SuppressedBelowThreshold(t *testing.T) {
	// default buildSnap has 30+25+25 = 80... that would actually fire.
	// use a flatter distribution.
	snap := buildSnap().withTopCountries([]CountryShare{
		{Code: "A", IPs: 200_000, Share: 0.20},
		{Code: "B", IPs: 200_000, Share: 0.20},
		{Code: "C", IPs: 200_000, Share: 0.20},
		{Code: "D", IPs: 200_000, Share: 0.20},
		{Code: "E", IPs: 200_000, Share: 0.20},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "country_concentrated") {
		t.Fatalf("expected country_concentrated to stay silent at 60%% top3; got %v", insightCodes(got))
	}
}

func TestRuleCountryConcentrated_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withTotalIPs(10).withTopCountries([]CountryShare{
		{Code: "FR", IPs: 4, Share: 0.40},
		{Code: "DE", IPs: 3, Share: 0.30},
		{Code: "US", IPs: 3, Share: 0.30},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "country_concentrated") {
		t.Fatalf("expected country_concentrated silent on <100 IPs; got %v", insightCodes(got))
	}
}

func TestRuleCountryConcentrated_SuppressedWhenExtremeSingleCountry(t *testing.T) {
	// Single country > 95% triggers R07, not R05.
	top := []CountryShare{{Code: "US", Name: "United States", IPs: 970_000, Share: 0.97}}
	for i := 0; i < 4; i++ {
		top = append(top, CountryShare{Code: "X", IPs: 5_000, Share: 0.005})
	}
	snap := buildSnap().withTopCountries(top).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "country_concentrated") {
		t.Fatalf("country_concentrated should defer to single_country; got %v", insightCodes(got))
	}
	if !containsCode(got, "single_country") {
		t.Fatalf("expected single_country to fire; got %v", insightCodes(got))
	}
}

func TestRuleCountryDiverse_Fires(t *testing.T) {
	// 60 countries each at 1.6% — every country below 5% and count >= 50.
	top := make([]CountryShare, 0, 60)
	for i := 0; i < 60; i++ {
		top = append(top, CountryShare{Code: "X", IPs: 16_000, Share: 0.016})
	}
	snap := buildSnap().withTopCountries(top).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "country_diverse") {
		t.Fatalf("expected country_diverse to fire; got %v", insightCodes(got))
	}
}

func TestRuleCountryDiverse_SuppressedOnConcentration(t *testing.T) {
	// One country >5% blocks diversity.
	top := []CountryShare{{Code: "FR", IPs: 200_000, Share: 0.20}}
	for i := 0; i < 60; i++ {
		top = append(top, CountryShare{Code: "X", IPs: 13_000, Share: 0.013})
	}
	snap := buildSnap().withTopCountries(top).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "country_diverse") {
		t.Fatalf("expected country_diverse silent with a 20%% country; got %v", insightCodes(got))
	}
}

func TestRuleCountryDiverse_SuppressedOnFewCountries(t *testing.T) {
	// Every country below 5% but only 20 of them — below the 50-country
	// fingerprint requirement.
	top := make([]CountryShare, 0, 20)
	for i := 0; i < 20; i++ {
		top = append(top, CountryShare{Code: "X", IPs: 50_000, Share: 0.05 - 0.001})
	}
	snap := buildSnap().withTopCountries(top).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "country_diverse") {
		t.Fatalf("expected country_diverse silent with 20 countries; got %v", insightCodes(got))
	}
}

func TestRuleSingleCountry_Fires(t *testing.T) {
	snap := buildSnap().withTopCountries([]CountryShare{
		{Code: "CN", Name: "China", IPs: 970_000, Share: 0.97},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if !containsCode(got, "single_country") {
		t.Fatalf("expected single_country to fire; got %v", insightCodes(got))
	}
}

func TestRuleSingleCountry_SuppressedBelowThreshold(t *testing.T) {
	snap := buildSnap().withTopCountries([]CountryShare{
		{Code: "CN", Name: "China", IPs: 900_000, Share: 0.90},
		{Code: "US", Name: "United States", IPs: 100_000, Share: 0.10},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "single_country") {
		t.Fatalf("expected single_country silent at 90%%; got %v", insightCodes(got))
	}
}

func TestRuleSingleCountry_SuppressedBySample(t *testing.T) {
	snap := buildSnap().withTotalIPs(50).withTopCountries([]CountryShare{
		{Code: "CN", IPs: 50, Share: 1.0},
	}).toSnap()
	got := NewEngine().Derive(snap)
	if containsCode(got, "single_country") {
		t.Fatalf("expected single_country silent on 50 IPs; got %v", insightCodes(got))
	}
}
