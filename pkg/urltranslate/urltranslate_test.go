package urltranslate

import "testing"

/*
CASE A
https://{Bucket}.s3.{region}.amazonaws.com
THIS WORKS
*/

func TestDotRegion_VariableDetected(t *testing.T) {
	u, err := ExtractParameterisedURL("https://{Bucket}.s3.{region}.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := u.GetVarByName("Bucket"); !ok {
		t.Fatalf("expected Bucket variable to be detected")
	}

	if _, ok := u.GetVarByName("region"); !ok {
		t.Fatalf("expected region variable to be detected")
	}
}

func TestDotRegion_SanitisePreservesURL(t *testing.T) {
	out, err := SanitiseServerURL("https://{Bucket}.s3.{region}.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://{Bucket}.s3.{region}.amazonaws.com"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

/*
CASE B
https://{Bucket}.s3-{region}.amazonaws.com
THIS DOES NOT WORK UPSTACK
*/

func TestDashRegion_VariableDetected(t *testing.T) {
	u, err := ExtractParameterisedURL("https://{Bucket}.s3-{region}.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := u.GetVarByName("Bucket"); !ok {
		t.Fatalf("expected Bucket variable to be detected")
	}

	if _, ok := u.GetVarByName("region"); !ok {
		t.Fatalf("expected region variable to be detected")
	}
}

func TestDashRegion_SanitisePreservesURL(t *testing.T) {
	out, err := SanitiseServerURL("https://{Bucket}.s3-{region}.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://{Bucket}.s3-{region}.amazonaws.com"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}
