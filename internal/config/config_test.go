package config

import "testing"

func TestValidateLoginAndListenSettings(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{"missing password", Config{Bind: "127.0.0.1"}, true},
		{"old default password", Config{Bind: "127.0.0.1", AdminPass: "admin"}, true},
		{"loopback HTTP", Config{Bind: "127.0.0.1", AdminPass: "test-config-password"}, false},
		{"IPv6 loopback HTTP", Config{Bind: "::1", AdminPass: "test-config-password"}, false},
		{"remote HTTP", Config{Bind: "0.0.0.0", AdminPass: "test-config-password"}, true},
		{"incomplete TLS", Config{Bind: "127.0.0.1", AdminPass: "test-config-password", TLSCert: "cert.pem"}, true},
		{"remote HTTPS", Config{Bind: "0.0.0.0", AdminPass: "test-config-password", TLSCert: "cert.pem", TLSKey: "key.pem"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, want error: %t", err, test.wantErr)
			}
		})
	}
}
