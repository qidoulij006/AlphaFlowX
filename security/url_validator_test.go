package security

import "testing"

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "empty allowed", rawURL: "", wantErr: false},
		{name: "public https proxy allowed", rawURL: "https://8.8.8.8:8443", wantErr: false},
		{name: "socks5 proxy allowed", rawURL: "socks5://1.1.1.1:1080", wantErr: false},
		{name: "localhost blocked", rawURL: "http://127.0.0.1:8080", wantErr: true},
		{name: "private ip blocked", rawURL: "http://10.0.0.5:8080", wantErr: true},
		{name: "metadata blocked", rawURL: "http://169.254.169.254:80", wantErr: true},
		{name: "bad scheme blocked", rawURL: "file:///tmp/proxy.sock", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyURL(tt.rawURL)
			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
