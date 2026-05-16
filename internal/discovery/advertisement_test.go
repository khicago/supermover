package discovery

import "testing"

func TestAdvertisementValidate(t *testing.T) {
	valid := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair", "l2"})
	tests := []struct {
		name    string
		ad      Advertisement
		wantErr bool
	}{
		{name: "valid low info", ad: valid, wantErr: false},
		{name: "missing service", ad: withService(valid, ""), wantErr: true},
		{name: "bad protocol", ad: withProtocol(valid, "one"), wantErr: true},
		{name: "short nonce", ad: withNonce(valid, "abc"), wantErr: true},
		{name: "bad capability", ad: withCapabilities(valid, []string{"pair now"}), wantErr: true},
		{name: "username txt", ad: withTXT(valid, map[string]string{"username": "alice"}), wantErr: true},
		{name: "path txt", ad: withTXT(valid, map[string]string{"path": "/Users/alice"}), wantErr: true},
		{name: "hostname txt", ad: withTXT(valid, map[string]string{"hostname": "alice-mbp.local"}), wantErr: true},
		{name: "profile label txt", ad: withTXT(valid, map[string]string{"profile_label": "work"}), wantErr: true},
		{name: "file count txt", ad: withTXT(valid, map[string]string{"file_count": "100"}), wantErr: true},
		{name: "friendly name txt", ad: withTXT(valid, map[string]string{"friendly_name": "Alice Mac"}), wantErr: true},
		{name: "allowed txt", ad: withTXT(valid, map[string]string{"svc": "_supermover._tcp", "proto": "supermover/1", "nonce": "abcdef0123456789", "caps": "l2,pair"}), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ad.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("Advertisement.Validate(%+v) error = %v, want error presence = %t", tt.ad, err, tt.wantErr)
			}
		})
	}
}

func TestAdvertisementTXT(t *testing.T) {
	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair", "l2"})
	got, err := ad.TXT()
	if err != nil {
		t.Fatalf("Advertisement.TXT(%+v) error = %v, want nil", ad, err)
	}
	want := map[string]string{
		"caps":  "l2,pair",
		"nonce": "abcdef0123456789",
		"proto": "supermover/1",
		"svc":   "_supermover._tcp",
	}
	for key, wantValue := range want {
		if gotValue := got[key]; gotValue != wantValue {
			t.Errorf("Advertisement.TXT(%+v)[%q] = %q, want %q", ad, key, gotValue, wantValue)
		}
	}
}

func withService(ad Advertisement, service string) Advertisement {
	ad.ServiceType = service
	return ad
}

func withProtocol(ad Advertisement, protocol string) Advertisement {
	ad.ProtocolVersion = protocol
	return ad
}

func withNonce(ad Advertisement, nonce string) Advertisement {
	ad.EphemeralNonce = nonce
	return ad
}

func withCapabilities(ad Advertisement, capabilities []string) Advertisement {
	ad.CapabilityFlags = capabilities
	return ad
}

func withTXT(ad Advertisement, txt map[string]string) Advertisement {
	ad.UnauthenticatedTXT = txt
	return ad
}
