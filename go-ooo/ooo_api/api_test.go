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
	}{
		{endpoint: "WETH.USDC.AD", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true},
		{endpoint: "WETH.USDC.AD.30", base: "WETH", target: "USDC", qType: "AD", minutes: 30, adhoc: true},
		{endpoint: "WETH.USDC.AD.99", base: "WETH", target: "USDC", qType: "AD", minutes: 60, adhoc: true}, // clamp >60
		{endpoint: "WETH.USDC.AD.-5", base: "WETH", target: "USDC", qType: "AD", minutes: 0, adhoc: true},  // clamp <0
		{endpoint: "BTC.GBP.PR.AVC.1H", base: "BTC", target: "GBP", qType: "PR", minutes: 0, adhoc: false},
		{endpoint: "BTC.GBP", wantErr: true},  // too short
		{endpoint: ".GBP.AD", wantErr: true},  // empty base
		{endpoint: "BTC..AD", wantErr: true},  // empty target
		{endpoint: "BTC.GBP.", wantErr: true}, // empty qType
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
	}
}

func TestIsAdhoc(t *testing.T) {
	if ad, err := IsAdhoc("WETH.USDC.AD"); err != nil || !ad {
		t.Errorf("WETH.USDC.AD: adhoc=%v err=%v", ad, err)
	}
	if pr, err := IsAdhoc("BTC.GBP.PR.AVC.1H"); err != nil || pr {
		t.Errorf("BTC.GBP.PR...: adhoc=%v err=%v", pr, err)
	}
	if _, err := IsAdhoc("BTC.GBP"); err == nil {
		t.Error("BTC.GBP: expected a parse error")
	}
}
