// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func rule(direction, protocol, port, cidr string) SecurityGroupRuleModel {
	return SecurityGroupRuleModel{
		Direction:  types.StringValue(direction),
		Ethertype:  types.StringValue("IPv4"),
		Protocol:   types.StringValue(protocol),
		PortNumber: types.StringValue(port),
		CIDR:       types.StringValue(cidr),
	}
}

// The live API does not return security group rules in a stable order
// across requests (observed directly: refreshing twice in a row swapped two
// rules back and forth forever), but "rule" is a list block, so Read must
// restore the prior order for unchanged content or every plan sees a
// permanent, non-converging reorder-only diff.
func TestReorderRulesToMatch(t *testing.T) {
	sshRule := rule("ingress", "tcp", "22", "0.0.0.0/0")
	httpsRule := rule("ingress", "tcp", "443", "0.0.0.0/0")
	icmpRule := rule("ingress", "icmp", "", "0.0.0.0/0")

	tests := []struct {
		name  string
		prior []SecurityGroupRuleModel
		fresh []SecurityGroupRuleModel
		want  []SecurityGroupRuleModel
	}{
		{
			name:  "restores prior order when the API returns rules reshuffled",
			prior: []SecurityGroupRuleModel{httpsRule, icmpRule, sshRule},
			fresh: []SecurityGroupRuleModel{sshRule, httpsRule, icmpRule},
			want:  []SecurityGroupRuleModel{httpsRule, icmpRule, sshRule},
		},
		{
			name:  "appends a genuinely new rule after the restored prior order",
			prior: []SecurityGroupRuleModel{sshRule},
			fresh: []SecurityGroupRuleModel{httpsRule, sshRule}, // added out-of-band
			want:  []SecurityGroupRuleModel{sshRule, httpsRule},
		},
		{
			name:  "returns fresh unchanged when there is no prior state",
			prior: nil,
			fresh: []SecurityGroupRuleModel{httpsRule, sshRule},
			want:  []SecurityGroupRuleModel{httpsRule, sshRule},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderRulesToMatch(tt.prior, tt.fresh)

			if len(got) != len(tt.want) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if !ruleMatches(got[i], tt.want[i]) {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
