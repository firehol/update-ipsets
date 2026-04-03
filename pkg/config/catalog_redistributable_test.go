package config

import "testing"

// EXPLICITLY forbid redistribution (wording like "no redistribution",
// "redistribution prohibited", "reproduction or republication
// prohibited"). Non-commercial clauses, attribution requirements, "no
// resale" clauses, use restrictions, and "all rights reserved" defaults
// are NOT grounds for `redistributable: false` — they flow through to
// downstream consumers under the original license.
func TestCatalogRedistributableConsistency(t *testing.T) {
	cfg := loadCatalog(t)

	// Sources whose published license explicitly forbids
	// redistribution. Keep this list in strict alphabetical order for
	// easy diffing against the source catalog.
	//
	// Evidence was verified by fetching each source and reading the
	// actual direct-upstream feed header / page. The current confirmed
	// families with explicit prohibition language are:
	//
	//   - dataplane.org/*.txt : "Redistribution of the <report>
	//     report in whole or in part without the express permission
	//     of Dataplane.org is expressly prohibited." (in every feed
	//     file header)
	//   - greensnow.co : "Reproduction or republication strictly
	//     prohibited." (on https://greensnow.co/)
	//   - CAIDA Public AUA prohibits distributing, disclosing, or
	//     transferring the dataset outside the recipient institution.
	//   - CleanTalk terms prohibit publishing data received from the
	//     service without prior written consent.
	//   - iBlocklist Terms grant personal use only and prohibit public
	//     display, including non-commercial public display.
	//   - IP2Location LITE, IP2Proxy LITE, IPIP, and MaxMind terms
	//     explicitly prohibit redistribution/distribution except under
	//     narrow written-license or bundled-application carve-outs.
	//   - Project Honey Pot terms prohibit reproduction without written
	//     permission.
	//   - AWS, GitHub, Microsoft, Stripe, and HashiCorp provider terms
	//     prohibit or do not grant raw redistribution of the provider IP data.
	shouldNotRedistribute := []string{
		"caida_prefix2as",
		"cleantalk_new",
		"cleantalk_updated",
		"critical_context_github_hosted_compute",
		"critical_soft_aws_cloudfront",
		"critical_soft_microsoft365",
		"critical_soft_stripe_api",
		"critical_soft_stripe_webhooks",
		"critical_soft_terraform_cloud",
		"dataplane_dnsrd",
		"dataplane_dnsrdany",
		"dataplane_dnsversion",
		"dataplane_proto41",
		"dataplane_sipinvitation",
		"dataplane_sipquery",
		"dataplane_sipregistration",
		"dataplane_smtpdata",
		"dataplane_smtpgreet",
		"dataplane_sshclient",
		"dataplane_sshpwauth",
		"dataplane_telnetlogin",
		"dataplane_vncrfb",
		"greensnow",
		"iblocklist_abuse_palevo",
		"iblocklist_abuse_spyeye",
		"iblocklist_abuse_zeus",
		"iblocklist_ads",
		"iblocklist_bogons",
		"iblocklist_ciarmy_malicious",
		"iblocklist_cidr_report_bogons",
		"iblocklist_cruzit_web_attacks",
		"iblocklist_dshield",
		"iblocklist_edu",
		"iblocklist_exclusions",
		"iblocklist_fornonlancomputers",
		"iblocklist_forumspam",
		"iblocklist_hijacked",
		"iblocklist_iana_multicast",
		"iblocklist_iana_private",
		"iblocklist_iana_reserved",
		"iblocklist_isp_aol",
		"iblocklist_isp_att",
		"iblocklist_isp_cablevision",
		"iblocklist_isp_charter",
		"iblocklist_isp_comcast",
		"iblocklist_isp_embarq",
		"iblocklist_isp_sprint",
		"iblocklist_isp_suddenlink",
		"iblocklist_isp_twc",
		"iblocklist_isp_verizon",
		"iblocklist_level1",
		"iblocklist_level2",
		"iblocklist_level3",
		"iblocklist_malc0de",
		"iblocklist_onion_router",
		"iblocklist_org_activision",
		"iblocklist_org_apple",
		"iblocklist_org_blizzard",
		"iblocklist_org_crowd_control",
		"iblocklist_org_electronic_arts",
		"iblocklist_org_joost",
		"iblocklist_org_linden_lab",
		"iblocklist_org_logmein",
		"iblocklist_org_microsoft",
		"iblocklist_org_ncsoft",
		"iblocklist_org_nintendo",
		"iblocklist_org_pirate_bay",
		"iblocklist_org_punkbuster",
		"iblocklist_org_riot_games",
		"iblocklist_org_sony_online",
		"iblocklist_org_square_enix",
		"iblocklist_org_steam",
		"iblocklist_org_ubisoft",
		"iblocklist_org_xfire",
		"iblocklist_pedophiles",
		"iblocklist_proxies",
		"iblocklist_rangetest",
		"iblocklist_spamhaus_drop",
		"iblocklist_spider",
		"iblocklist_spyware",
		"iblocklist_webexploit",
		"iblocklist_yoyo_adservers",
		"ip2location_country",
		"ip2proxy_px1lite",
		"ipip_country",
		"maxmind_proxy_fraud",
		"php_bad",
		"php_commenters",
		"php_dictionary",
		"php_spammers",
		"provider_context_aws_cloud",
	}

	for _, name := range shouldNotRedistribute {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("source %q not found", name)
			continue
		}
		if src.IsRedistributable() {
			t.Errorf("source %q should NOT be redistributable but is", name)
		}
	}

	// These SHOULD be redistributable. Sampled from groups that were
	// previously (incorrectly) marked non-redistributable because a
	// non-commercial or attribution clause was conflated with a
	// redistribution prohibition. Keep a representative from each
	// flipped group so a regression that blanket-flips everything
	// back fails loudly.
	shouldRedistribute := []string{
		// Classic sanity checks
		"spamhaus_drop", "blocklist_de", "feodo",
		"tor_exits", "vxvault", "ciarmy",
		// Team Cymru bogons / fullbogons — "Free. Forever." per
		// https://www.team-cymru.com/bogon-reference-http, no
		// redistribution prohibition. Previously misclassified.
		"bogons", "fullbogons",
		// Flipped from false → default-true (NC / attribution / "no resale"
		// / use restrictions / all-rights-reserved — none are explicit
		// redistribution prohibitions).
		"bds_atif",                   // "non-commercial use only"
		"botscout",                   // Last Bots Caught feed has an explicit downstream-use carve-out.
		"botvrij_dst", "botvrij_src", // "use at own risk, no resale"
		"drb_ra_c2intel",                             // CC BY-NC-SA 4.0 permits non-commercial sharing.
		"dronebl_anonymizers", "dronebl_compromised", // BSD-style
		"dshield",                    // CC BY-NC-SA 2.5
		"griffinguard",               // "security research / monitoring only", no redistribution prohibition found.
		"provider_context_gcp_cloud", // Google IP ranges are CC BY 4.0.
		"stopforumspam",              // CC BY-NC-ND 3.0 — verbatim redistribution permitted
		"abuseipdb_1d",               // AbuseIPDB ToS — no explicit bulk-redistribution prohibition on borestad's public mirror
		"socks_proxy",                // no explicit redistribution prohibition found.
	}
	for _, name := range shouldRedistribute {
		src, ok := cfg.Sources[name]
		if !ok {
			t.Errorf("source %q not found", name)
			continue
		}
		if !src.IsRedistributable() {
			t.Errorf("source %q SHOULD be redistributable but is not", name)
		}
	}
}
