package dynago

import (
	"testing"
)

// FuzzEncodeDecodeRoundTrip fuzzes the Marshal/Unmarshal round-trip.
// It generates random struct data and verifies that marshaling then
// unmarshaling produces equivalent values.
func FuzzEncodeDecodeRoundTrip(f *testing.F) {
	// Seed corpus with interesting values.
	f.Add("hello", int64(42), true)
	f.Add("", int64(0), false)
	f.Add("unicode: \u00e9\u00e8\u00ea", int64(-100), true)
	f.Add("special chars: \t\n\r", int64(9999999), false)
	f.Add("very long string that goes on and on", int64(1), true)

	type fuzzItem struct {
		PK     string `dynamo:"PK"`
		SK     string `dynamo:"SK"`
		Name   string `dynamo:"Name"`
		Count  int64  `dynamo:"Count"`
		Active bool   `dynamo:"Active"`
	}

	f.Fuzz(func(t *testing.T, name string, count int64, active bool) {
		original := fuzzItem{
			PK:     "fuzz#1",
			SK:     "test",
			Name:   name,
			Count:  count,
			Active: active,
		}

		// Marshal.
		av, err := Marshal(original)
		if err != nil {
			t.Fatal(err)
		}

		// Unmarshal.
		var decoded fuzzItem
		if err := Unmarshal(av, &decoded); err != nil {
			t.Fatal(err)
		}

		// Verify round-trip.
		if decoded.PK != original.PK {
			t.Fatalf("PK mismatch: %q vs %q", decoded.PK, original.PK)
		}
		if decoded.SK != original.SK {
			t.Fatalf("SK mismatch: %q vs %q", decoded.SK, original.SK)
		}
		if decoded.Name != original.Name {
			t.Fatalf("Name mismatch: %q vs %q", decoded.Name, original.Name)
		}
		if decoded.Count != original.Count {
			t.Fatalf("Count mismatch: %d vs %d", decoded.Count, original.Count)
		}
		if decoded.Active != original.Active {
			t.Fatalf("Active mismatch: %v vs %v", decoded.Active, original.Active)
		}
	})
}
