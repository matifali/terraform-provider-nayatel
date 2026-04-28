// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	nayatelclient "github.com/matifali/terraform-provider-nayatel/internal/client"
)

const testAccRunRouterTestsEnvVar = "NAYATEL_ACC_RUN_ROUTER_TESTS"

const testAccDefaultNetworkBandwidthLimit = 1

var testAccNetworkPreviewCache = struct {
	sync.Mutex
	results map[int]error
}{
	results: make(map[int]error),
}

func testAccName(prefix string) string {
	return acctest.RandomWithPrefix(prefix)
}

func testAccPublicKey(t *testing.T) string {
	t.Helper()

	publicKey, _, err := acctest.RandSSHKeyPair(testAccName("tf-acc"))
	if err != nil {
		t.Fatalf("failed to generate acceptance test SSH key: %s", err)
	}

	return publicKey
}

func testAccImageIDExpression() string {
	if imageID := os.Getenv("NAYATEL_ACC_IMAGE_ID"); imageID != "" {
		return fmt.Sprintf("%q", imageID)
	}

	return "data.nayatel_images.available.images[0].id"
}

func testAccImageDataSourceConfig() string {
	if os.Getenv("NAYATEL_ACC_IMAGE_ID") != "" {
		return ""
	}

	return `
data "nayatel_images" "available" {}
`
}

func testAccPreCheckImageSelection(t *testing.T) {
	t.Helper()

	if os.Getenv("NAYATEL_ACC_IMAGE_ID") != "" {
		testAccPreCheck(t)
		return
	}

	testAccPreCheckImagesAvailable(t, "image-dependent acceptance test")
}

func testAccPreCheckImages(t *testing.T) {
	t.Helper()

	testAccPreCheckImagesAvailable(t, "images data source acceptance test")
}

func testAccPreCheckImagesAvailable(t *testing.T, description string) {
	t.Helper()

	testAccPreCheck(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*nayatelclient.DefaultTimeout)
	defer cancel()

	c, err := testAccClientFromEnv(ctx)
	if err != nil {
		t.Fatalf("failed to create Nayatel client for %s precheck: %s", description, err)
	}

	images, err := c.Images.List(ctx)
	if err != nil {
		if testAccIsRateLimitedError(err) {
			t.Skipf("Skipping %s: Nayatel API rate limited the images lookup; retry later: %s", description, err)
		}
		t.Fatalf("images lookup failed before running %s: %s", description, err)
	}
	if len(images) == 0 {
		t.Skipf("Skipping %s: the live Nayatel account/API returned no images", description)
	}

	hasImageID := false
	for _, image := range images {
		if strings.TrimSpace(image.ID) != "" {
			hasImageID = true
			break
		}
	}
	if !hasImageID {
		t.Skipf("Skipping %s: the live Nayatel account/API returned no images with a non-empty ID", description)
	}
	if description == "image-dependent acceptance test" && strings.TrimSpace(images[0].ID) == "" {
		t.Skipf("Skipping %s: the first live Nayatel image has an empty ID", description)
	}
}

func testAccPreCheckRouterTests(t *testing.T) {
	t.Helper()

	if os.Getenv(testAccRunRouterTestsEnvVar) != "1" {
		t.Skipf("Set %s=1 to run router-dependent acceptance tests; Nayatel currently exposes no working router-interface detach endpoint, so router/interface stacks can fail cleanup and leak resources", testAccRunRouterTestsEnvVar)
	}

	testAccPreCheck(t)
}

func testAccVolumeSize(t *testing.T) int {
	t.Helper()

	if size := os.Getenv("NAYATEL_ACC_VOLUME_SIZE"); size != "" {
		value, err := strconv.Atoi(size)
		if err != nil || value <= 0 {
			t.Fatalf("NAYATEL_ACC_VOLUME_SIZE must be a positive integer, got %q", size)
		}

		return value
	}

	return 1
}

func testAccNetworkBandwidthLimit(t *testing.T) int {
	t.Helper()

	if bandwidth := os.Getenv("NAYATEL_ACC_NETWORK_BANDWIDTH_LIMIT"); bandwidth != "" {
		value, err := strconv.Atoi(bandwidth)
		if err != nil || value <= 0 {
			t.Fatalf("NAYATEL_ACC_NETWORK_BANDWIDTH_LIMIT must be a positive integer, got %q", bandwidth)
		}

		return value
	}

	return testAccDefaultNetworkBandwidthLimit
}

func testAccPreCheckNetworkBandwidth(t *testing.T, bandwidth int) {
	t.Helper()

	testAccPreCheck(t)

	err := testAccNetworkBandwidthPreviewError(t, bandwidth)
	if err == nil {
		return
	}

	switch {
	case testAccIsNetworkBandwidthAlreadyExistsError(err):
		t.Skipf("Skipping network-dependent acceptance test: the live Nayatel account/project already has a network at bandwidth_limit=%d. Set NAYATEL_ACC_NETWORK_BANDWIDTH_LIMIT to another available positive integer to run this test: %s", bandwidth, err)
	case testAccIsRateLimitedError(err):
		t.Skipf("Skipping network-dependent acceptance test: Nayatel API rate limited the network preview for bandwidth_limit=%d; retry later: %s", bandwidth, err)
	default:
		t.Fatalf("network preview failed for bandwidth_limit=%d before attempting a billable create: %s", bandwidth, err)
	}
}

func testAccPreCheckVolumes(t *testing.T) {
	t.Helper()

	testAccPreCheck(t)
	if os.Getenv("NAYATEL_ACC_RUN_VOLUME_TESTS") != "1" {
		t.Skip("Set NAYATEL_ACC_RUN_VOLUME_TESTS=1 to run volume acceptance tests; recent live runs returned 404 from unverified volume endpoints, and volume creation may be billable")
	}
}

func testAccPreCheckFlavors(t *testing.T) {
	t.Helper()

	testAccPreCheck(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*nayatelclient.DefaultTimeout)
	defer cancel()

	c, err := testAccClientFromEnv(ctx)
	if err != nil {
		t.Fatalf("failed to create Nayatel client for flavors acceptance precheck: %s", err)
	}

	flavors, err := c.Flavors.List(ctx)
	if err != nil {
		if testAccIsRateLimitedError(err) {
			t.Skipf("Skipping flavors data source acceptance test: Nayatel API rate limited the flavors lookup; retry later: %s", err)
		}
		t.Fatalf("flavors lookup failed before running data source acceptance test: %s", err)
	}
	if len(flavors) == 0 {
		t.Skip("Skipping flavors data source acceptance test: the live Nayatel account/API returned no flavors")
	}
}

func testAccPreCheckVolumeAttachments(t *testing.T, bandwidth int) {
	t.Helper()

	testAccPreCheckRouterTests(t)
	testAccPreCheckVolumes(t)
	if os.Getenv("NAYATEL_ACC_RUN_VOLUME_ATTACHMENT_TESTS") != "1" {
		t.Skip("Set NAYATEL_ACC_RUN_VOLUME_ATTACHMENT_TESTS=1 in addition to NAYATEL_ACC_RUN_VOLUME_TESTS=1 and NAYATEL_ACC_RUN_ROUTER_TESTS=1 to run volume attachment acceptance tests; they create a network/router/instance/volume stack and may incur charges")
	}
	testAccPreCheckNetworkBandwidth(t, bandwidth)
}

func testAccNetworkBandwidthPreviewError(t *testing.T, bandwidth int) error {
	t.Helper()

	if bandwidth <= 0 {
		t.Fatalf("network bandwidth limit must be positive, got %d", bandwidth)
	}

	testAccNetworkPreviewCache.Lock()
	err, ok := testAccNetworkPreviewCache.results[bandwidth]
	testAccNetworkPreviewCache.Unlock()
	if ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*nayatelclient.DefaultTimeout)
	defer cancel()

	c, err := testAccClientFromEnv(ctx)
	if err == nil {
		_, err = c.Networks.Preview(ctx, &nayatelclient.NetworkCreateRequest{BandwidthLimit: bandwidth})
	}

	testAccNetworkPreviewCache.Lock()
	testAccNetworkPreviewCache.results[bandwidth] = err
	testAccNetworkPreviewCache.Unlock()

	return err
}

func testAccClientFromEnv(ctx context.Context) (*nayatelclient.Client, error) {
	options := make([]nayatelclient.ClientOption, 0, 2)
	if baseURL := os.Getenv("NAYATEL_BASE_URL"); baseURL != "" {
		options = append(options, nayatelclient.WithBaseURL(baseURL))
	}
	if projectID := os.Getenv("NAYATEL_PROJECT_ID"); projectID != "" {
		options = append(options, nayatelclient.WithProjectID(projectID))
	}

	username := os.Getenv("NAYATEL_USERNAME")
	if token := os.Getenv("NAYATEL_TOKEN"); token != "" {
		return nayatelclient.NewClient(username, token, options...), nil
	}

	return nayatelclient.NewClientWithLogin(ctx, username, os.Getenv("NAYATEL_PASSWORD"), options...)
}

func testAccIsNetworkBandwidthAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	var apiErr *nayatelclient.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode != http.StatusBadRequest {
			return false
		}
		message = strings.ToLower(apiErr.Message)
	}

	return strings.Contains(message, "network with the bandwidth") &&
		strings.Contains(message, "already exists")
}

func testAccIsRateLimitedError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *nayatelclient.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status 429") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "rate limit")
}

func testAccCheckNestedListNotEmpty(resourceName, listAttr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		state, err := testAccPrimaryInstanceState(s, resourceName)
		if err != nil {
			return err
		}

		count, err := testAccNestedListCount(state, listAttr)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", resourceName, listAttr, err)
		}
		if count == 0 {
			return fmt.Errorf("%s.%s: expected at least one element", resourceName, listAttr)
		}

		return nil
	}
}

func testAccCheckAnyNestedAttrsSet(resourceName, listAttr string, nestedAttrs ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		state, err := testAccPrimaryInstanceState(s, resourceName)
		if err != nil {
			return err
		}

		count, err := testAccNestedListCount(state, listAttr)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", resourceName, listAttr, err)
		}

		for i := 0; i < count; i++ {
			allSet := true
			for _, nestedAttr := range nestedAttrs {
				key := fmt.Sprintf("%s.%d.%s", listAttr, i, nestedAttr)
				if strings.TrimSpace(state.Attributes[key]) == "" {
					allSet = false
					break
				}
			}
			if allSet {
				return nil
			}
		}

		return fmt.Errorf("%s.%s: no element has all attributes set: %s", resourceName, listAttr, strings.Join(nestedAttrs, ", "))
	}
}

func testAccCheckAnyNestedAttrsSetAndIntAttrsPositive(resourceName, listAttr string, stringAttrs, intAttrs []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		state, err := testAccPrimaryInstanceState(s, resourceName)
		if err != nil {
			return err
		}

		count, err := testAccNestedListCount(state, listAttr)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", resourceName, listAttr, err)
		}

		for i := 0; i < count; i++ {
			allValid := true
			for _, nestedAttr := range stringAttrs {
				key := fmt.Sprintf("%s.%d.%s", listAttr, i, nestedAttr)
				if strings.TrimSpace(state.Attributes[key]) == "" {
					allValid = false
					break
				}
			}
			if !allValid {
				continue
			}
			for _, nestedAttr := range intAttrs {
				key := fmt.Sprintf("%s.%d.%s", listAttr, i, nestedAttr)
				value, err := strconv.Atoi(state.Attributes[key])
				if err != nil || value <= 0 {
					allValid = false
					break
				}
			}
			if allValid {
				return nil
			}
		}

		return fmt.Errorf("%s.%s: no element has required string attributes %s and positive integer attributes %s", resourceName, listAttr, strings.Join(stringAttrs, ", "), strings.Join(intAttrs, ", "))
	}
}

func testAccCheckNestedListContainsResourceAttr(dataSourceName, listAttr, nestedAttr, resourceName, resourceAttr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		dataState, err := testAccPrimaryInstanceState(s, dataSourceName)
		if err != nil {
			return err
		}

		resourceState, err := testAccPrimaryInstanceState(s, resourceName)
		if err != nil {
			return err
		}

		target := strings.TrimSpace(resourceState.Attributes[resourceAttr])
		if target == "" {
			return fmt.Errorf("%s.%s is empty", resourceName, resourceAttr)
		}

		count, err := testAccNestedListCount(dataState, listAttr)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", dataSourceName, listAttr, err)
		}

		for i := 0; i < count; i++ {
			key := fmt.Sprintf("%s.%d.%s", listAttr, i, nestedAttr)
			if dataState.Attributes[key] == target {
				return nil
			}
		}

		return fmt.Errorf("%s.%s: no %s value matched %s.%s (%q)", dataSourceName, listAttr, nestedAttr, resourceName, resourceAttr, target)
	}
}

func testAccCompositeImportID(resourceName string, attrs ...string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		state, err := testAccPrimaryInstanceState(s, resourceName)
		if err != nil {
			return "", err
		}

		parts := make([]string, 0, len(attrs))
		for _, attr := range attrs {
			value := strings.TrimSpace(state.Attributes[attr])
			if value == "" {
				return "", fmt.Errorf("%s.%s is empty", resourceName, attr)
			}
			parts = append(parts, value)
		}

		return strings.Join(parts, ":"), nil
	}
}

func testAccPrimaryInstanceState(s *terraform.State, name string) (*terraform.InstanceState, error) {
	for _, module := range s.Modules {
		if rs, ok := module.Resources[name]; ok {
			if rs.Primary == nil {
				return nil, fmt.Errorf("%s has no primary instance", name)
			}

			return rs.Primary, nil
		}
	}

	return nil, fmt.Errorf("resource not found in state: %s", name)
}

func testAccNestedListCount(state *terraform.InstanceState, listAttr string) (int, error) {
	countKey := listAttr + ".#"
	countString, ok := state.Attributes[countKey]
	if !ok {
		return 0, fmt.Errorf("missing count attribute %q", countKey)
	}

	count, err := strconv.Atoi(countString)
	if err != nil {
		return 0, fmt.Errorf("invalid count %q for %q: %w", countString, countKey, err)
	}

	return count, nil
}
