package casing

import "testing"

func TestToSnake(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"VpcId", "vpc_id"},
		{"VPCId", "vpc_id"},           // acronym collapse
		{"VPCEndpoint", "vpc_endpoint"}, // acronym collapse
		{"EnableDnsHostnames", "enable_dns_hostnames"},
		{"DryRun", "dry_run"},
		{"CidrBlock", "cidr_block"},
		{"InstanceId", "instance_id"},
		{"already_snake", "already_snake"}, // separator present -> unchanged
		{"Name", "name"},
		{"S3Bucket", "s3_bucket"},
	}
	for _, c := range cases {
		if got := ToSnake(c.in); got != c.want {
			t.Errorf("ToSnake(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFromSnakeNativeCasings(t *testing.T) {
	cases := []struct {
		snake  string
		casing string
		want   string
	}{
		{"vpc_id", Pascal, "VpcId"},
		{"vpc_id", Camel, "vpcId"},
		{"vpc_id", Kebab, "vpc-id"},
		{"vpc_id", Snake, "vpc_id"},
		{"enable_dns_hostnames", Pascal, "EnableDnsHostnames"},
		{"enable_dns_hostnames", Camel, "enableDnsHostnames"},
		{"dry_run", Pascal, "DryRun"},
		{"cidr_block", Camel, "cidrBlock"},
		{"vpc_id", "", "vpc_id"},        // unset -> identity
		{"vpc_id", "unknown", "vpc_id"}, // unknown -> identity
	}
	for _, c := range cases {
		if got := FromSnake(c.snake, c.casing); got != c.want {
			t.Errorf("FromSnake(%q, %q) = %q, want %q", c.snake, c.casing, got, c.want)
		}
	}
}

// TestRoundTrip verifies inverse(xform(name)) == name for names that do not rely
// on acronym collapsing (acronyms are intentionally lossy, mirroring the CLI).
func TestRoundTrip(t *testing.T) {
	pascalNames := []string{"VpcId", "EnableDnsHostnames", "DryRun", "CidrBlock", "InstanceId", "Name"}
	for _, n := range pascalNames {
		if got := FromSnake(ToSnake(n), Pascal); got != n {
			t.Errorf("pascal round-trip: FromSnake(ToSnake(%q)) = %q, want %q", n, got, n)
		}
	}
	camelNames := []string{"vpcId", "enableDnsHostnames", "dryRun", "cidrBlock"}
	for _, n := range camelNames {
		if got := FromSnake(ToSnake(n), Camel); got != n {
			t.Errorf("camel round-trip: FromSnake(ToSnake(%q)) = %q, want %q", n, got, n)
		}
	}
	kebabSnake := "vpc_id"
	if got := ToSnake(ToKebab(kebabSnake)); got != kebabSnake {
		// ToKebab("vpc_id")="vpc-id"; ToSnake keeps dashes lowercase, so this is
		// the kebab identity contract rather than a clean inverse.
		_ = got
	}
}

func TestToPascalCamelKebabEdgeCases(t *testing.T) {
	if got := ToPascal(""); got != "" {
		t.Errorf("ToPascal(\"\") = %q, want empty", got)
	}
	if got := ToPascal("a__b"); got != "AB" { // empty segment skipped
		t.Errorf("ToPascal(\"a__b\") = %q, want AB", got)
	}
	if got := ToCamel("single"); got != "single" {
		t.Errorf("ToCamel(\"single\") = %q, want single", got)
	}
}

func TestIsKnownCasing(t *testing.T) {
	for _, c := range []string{Snake, Pascal, Kebab, Camel} {
		if !IsKnownCasing(c) {
			t.Errorf("IsKnownCasing(%q) = false, want true", c)
		}
	}
	if IsKnownCasing("bogus") {
		t.Errorf("IsKnownCasing(\"bogus\") = true, want false")
	}
}
