package gnb

import "testing"

func TestParseUeDataPlaneIdentity(t *testing.T) {
	const imsi = "imsi-208930000000001"

	tests := []struct {
		name    string
		input   string
		wantID  int64
		wantErr bool
	}{
		{"legacy", imsi, 4, false},
		{"session4", imsi + " 4", 4, false},
		{"session5", imsi + " 5", 5, false},
		{"empty", "", 0, true},
		{"missingIMSI", "imsi- 4", 0, true},
		{"invalidID", imsi + " abc", 0, true},
		{"invalidZero", imsi + " 0", 0, true},
		{"invalid16", imsi + " 16", 0, true},
		{"tooManyFields", imsi + " 4 extra", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotIMSI, gotID, err := parseUeDataPlaneIdentity(tc.input)

			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tc.wantErr {
				if gotIMSI != imsi || gotID != tc.wantID {
					t.Fatalf(
						"got IMSI=%s ID=%d, want IMSI=%s ID=%d",
						gotIMSI, gotID, imsi, tc.wantID,
					)
				}
			}
		})
	}
}

func TestDataPlaneSessionKey(t *testing.T) {
	const imsi = "imsi-208930000000001"

	key4 := dataPlaneSessionKey(imsi, 4)
	key5 := dataPlaneSessionKey(imsi, 5)

	if key4 != imsi {
		t.Fatal("legacy Session 4 key changed")
	}

	if key4 == key5 {
		t.Fatal("different sessions must have different keys")
	}
}
