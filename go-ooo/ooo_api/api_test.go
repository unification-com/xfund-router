package ooo_api

import "testing"

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		wantErr  bool
		base     string
		target   string
		qType    string
		minutes  int64
		adhoc    bool
		legacy   bool
		ignored  []string
	}{
		// canonical suffix-less form (always AdHoc)
		{endpoint: "WETH.USDC", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true},
		{endpoint: "WETH.USDC.30", base: "WETH", target: "USDC", qType: "AD", minutes: 30, adhoc: true},
		{endpoint: "WETH.USDC.99", base: "WETH", target: "USDC", qType: "AD", minutes: 60, adhoc: true}, // clamp >60
		{endpoint: "WETH.USDC.-5", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true},  // clamp <0
		// non-numeric window on the canonical form is silently dropped
		{endpoint: "WETH.USDC.FOO", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true, ignored: []string{"FOO"}},
		// trailing dot -> empty window, treated as minutes 0 (not an ignored field)
		{endpoint: "BTC.GBP.", base: "BTC", target: "GBP", qType: "AD", minutes: 0, adhoc: true},

		// legacy explicit-qualifier form
		{endpoint: "WETH.USDC.AD", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true, legacy: true},
		{endpoint: "WETH.USDC.AD.30", base: "WETH", target: "USDC", qType: "AD", minutes: 30, adhoc: true, legacy: true},
		{endpoint: "WETH.USDC.AD.99", base: "WETH", target: "USDC", qType: "AD", minutes: 60, adhoc: true, legacy: true}, // clamp >60
		// leftover Finchains-shaped params on a legacy .AD endpoint are dropped (silent-drop policy)
		{endpoint: "WETH.USDC.AD.AVG", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true, legacy: true, ignored: []string{"AVG"}},
		{endpoint: "WETH.USDC.AD.30.FOO", base: "WETH", target: "USDC", qType: "AD", minutes: 30, adhoc: true, legacy: true, ignored: []string{"FOO"}},
		{endpoint: "BTC.GBP.PR.AVC.1H", base: "BTC", target: "GBP", qType: "PR", minutes: 0, adhoc: false, legacy: true},

		// errors
		{endpoint: "BTC", wantErr: true},     // too short
		{endpoint: ".GBP.AD", wantErr: true}, // empty base
		{endpoint: "BTC..AD", wantErr: true}, // empty target
		{endpoint: "BTC.", wantErr: true},    // empty target
	}

	for _, tt := range tests {
		p, err := ParseEndpoint(tt.endpoint)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%q: expected an error, got none", tt.endpoint)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tt.endpoint, err)
			continue
		}
		if p.Base != tt.base || p.Target != tt.target || p.QType != tt.qType {
			t.Errorf("%q: got base=%q target=%q qType=%q", tt.endpoint, p.Base, p.Target, p.QType)
		}
		if p.Minutes != tt.minutes {
			t.Errorf("%q: minutes = %d, want %d", tt.endpoint, p.Minutes, tt.minutes)
		}
		if p.IsAdHoc() != tt.adhoc {
			t.Errorf("%q: IsAdHoc = %v, want %v", tt.endpoint, p.IsAdHoc(), tt.adhoc)
		}
		if p.Legacy != tt.legacy {
			t.Errorf("%q: Legacy = %v, want %v", tt.endpoint, p.Legacy, tt.legacy)
		}
		if !equalStrings(p.IgnoredFields, tt.ignored) {
			t.Errorf("%q: IgnoredFields = %v, want %v", tt.endpoint, p.IgnoredFields, tt.ignored)
		}
	}
}

func TestIsAdhoc(t *testing.T) {
	if ad, err := IsAdhoc("WETH.USDC.AD"); err != nil || !ad {
		t.Errorf("WETH.USDC.AD: adhoc=%v err=%v", ad, err)
	}
	if ad, err := IsAdhoc("WETH.USDC"); err != nil || !ad {
		t.Errorf("WETH.USDC (suffix-less): adhoc=%v err=%v", ad, err)
	}
	if pr, err := IsAdhoc("BTC.GBP.PR.AVC.1H"); err != nil || pr {
		t.Errorf("BTC.GBP.PR...: adhoc=%v err=%v", pr, err)
	}
	if _, err := IsAdhoc("BTC"); err == nil {
		t.Error("BTC: expected a parse error")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
