package client

import "testing"

func TestSecurityGroupRuleCreateRequestToAPIPayloadICMP(t *testing.T) {
	payload := (&SecurityGroupRuleCreateRequest{
		Direction: "Ingress",
		Ethertype: "IPv4",
		Protocol:  "icmp",
		CIDR:      "0.0.0.0/0",
	}).ToAPIPayload()

	if got, want := payload["ruleName"], "All ICMP"; got != want {
		t.Fatalf("ruleName = %v, want %v", got, want)
	}
	if got, want := payload["openPort"], false; got != want {
		t.Fatalf("openPort = %v, want %v", got, want)
	}
	if got, want := payload["cidr"], "0.0.0.0/0"; got != want {
		t.Fatalf("cidr = %v, want %v", got, want)
	}
}
