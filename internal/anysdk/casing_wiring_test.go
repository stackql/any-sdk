package anysdk_test

import (
	"testing"

	. "github.com/stackql/any-sdk/internal/anysdk"
)

func TestParameterNotFoundErrorMessage(t *testing.T) {
	err := &ParameterNotFoundError{
		Key:                "foo_bar",
		AvailableWireNames: []string{"VpcId", "EnableDnsHostnames", "DryRun"},
	}
	want := "field 'foo_bar' not found; available: [VpcId, EnableDnsHostnames, DryRun]"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
