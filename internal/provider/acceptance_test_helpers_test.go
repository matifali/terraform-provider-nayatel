// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testAccDefaultImageID = "7acb1e25-9ce1-4b6b-8d6e-38e7dbd20919"

func testAccName(prefix string) string {
	return acctest.RandomWithPrefix(prefix)
}

func testAccPublicKey(t *testing.T) string {
	t.Helper()

	if publicKey := os.Getenv("NAYATEL_ACC_PUBLIC_KEY"); publicKey != "" {
		return publicKey
	}

	publicKey, _, err := acctest.RandSSHKeyPair(testAccName("tf-acc"))
	if err != nil {
		t.Fatalf("failed to generate acceptance test SSH key: %s", err)
	}

	return publicKey
}

func testAccImageID() string {
	if imageID := os.Getenv("NAYATEL_ACC_IMAGE_ID"); imageID != "" {
		return imageID
	}

	return testAccDefaultImageID
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
