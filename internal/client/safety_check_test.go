package client

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestSafetyChecks tests the live balance and preview APIs without creating resources.
// It is skipped unless explicitly enabled because it calls live Nayatel endpoints.
// Run with: NAYATEL_RUN_SAFETY_CHECKS=1 go test -v -run TestSafetyChecks ./internal/client/.
func TestSafetyChecks(t *testing.T) {
	if os.Getenv("NAYATEL_RUN_SAFETY_CHECKS") != "1" {
		t.Skip("Set NAYATEL_RUN_SAFETY_CHECKS=1 to run live Nayatel safety checks")
	}

	username := os.Getenv("NAYATEL_USERNAME")
	token := os.Getenv("NAYATEL_TOKEN")
	password := os.Getenv("NAYATEL_PASSWORD")

	if username == "" || (token == "" && password == "") {
		t.Skip("Set NAYATEL_USERNAME with NAYATEL_TOKEN or NAYATEL_PASSWORD to run this test")
	}

	ctx := context.Background()
	var c *Client
	if token != "" {
		c = NewClient(username, token)
	} else {
		var err error
		c, err = NewClientWithLogin(ctx, username, password)
		if err != nil {
			t.Fatalf("NewClientWithLogin failed: %s", err)
		}
	}

	fmt.Println("")
	fmt.Println("=== Testing Safety Checks (NO resources will be created) ===")
	fmt.Println("")

	// Test 1: Balance API
	fmt.Println("1. Testing Balance API...")
	balance, err := c.GetBalance(ctx)
	if err != nil {
		t.Errorf("Balance API failed: %s", err)
	} else {
		fmt.Printf("   Balance: Rs. %.2f\n", balance.Balance)
		fmt.Printf("   Available Credit: Rs. %.2f\n", balance.AvailableCredit)
		fmt.Printf("   Effective Balance: Rs. %.2f\n", balance.Balance+balance.AvailableCredit)
	}

	// Test 2: Floating IP Preview
	fmt.Println("\n2. Testing Floating IP Preview API...")
	fipPreview, err := c.FloatingIPs.Preview(ctx, 1)
	if err != nil {
		t.Errorf("Floating IP Preview failed: %s", err)
	} else {
		cost := ExtractCostFromPreview(fipPreview)
		fmt.Printf("   Floating IP cost (prorated): Rs. %.2f\n", cost)
		if cost <= 0 {
			t.Error("Floating IP cost should be > 0")
		}
	}

	// Test 3: Network Preview
	fmt.Println("\n3. Testing Network Preview API...")
	netReq := &NetworkCreateRequest{BandwidthLimit: 1}
	netPreview, err := c.Networks.Preview(ctx, netReq)
	if err != nil {
		t.Errorf("Network Preview failed: %s", err)
	} else {
		cost := ExtractCostFromPreview(netPreview)
		fmt.Printf("   Network cost (prorated): Rs. %.2f\n", cost)
		if cost <= 0 {
			t.Error("Network cost should be > 0")
		}
	}

	// Test 4: Instance Preview
	fmt.Println("\n4. Testing Instance Preview API...")
	instReq := &InstanceCreateRequest{
		Name:    "test-preview-only",
		ImageID: "7acb1e25-9ce1-4b6b-8d6e-38e7dbd20919",
		CPU:     2,
		RAM:     2,
		Disk:    20,
	}
	instPreview, err := c.Instances.Preview(ctx, instReq)
	if err != nil {
		t.Errorf("Instance Preview failed: %s", err)
	} else {
		cost := ExtractCostFromPreview(instPreview)
		fmt.Printf("   Instance cost (prorated): Rs. %.2f\n", cost)
		if cost <= 0 {
			t.Error("Instance cost should be > 0")
		}
	}

	// Test 5: VerifyBalance helper
	fmt.Println("\n5. Testing VerifyBalance helper...")
	err = c.VerifyBalance(ctx, 100, "test")
	if err != nil {
		fmt.Printf("   VerifyBalance(100): %s\n", err)
	} else {
		fmt.Println("   VerifyBalance(100): OK - sufficient balance")
	}

	fmt.Println("\n=== Safety Check Summary ===")
	fmt.Println("If all above calls succeeded, the safety checks will work.")
	fmt.Println("NO resources were created, NO charges were made.")
}
