package client

import (
	"encoding/json"
	"testing"
)

// decodeInitialization unwraps the stringified INITIALIZATION payload field.
func decodeInitialization(t *testing.T, payload map[string]interface{}) map[string]interface{} {
	t.Helper()

	conf, ok := payload["conf"].(map[string]interface{})
	if !ok {
		t.Fatalf("conf missing from payload: %v", payload)
	}

	initStr, ok := conf["INITIALIZATION"].(string)
	if !ok {
		t.Fatalf("INITIALIZATION must be stringified JSON for portal parity, got %T", conf["INITIALIZATION"])
	}

	var init map[string]interface{}
	if err := json.Unmarshal([]byte(initStr), &init); err != nil {
		t.Fatalf("INITIALIZATION is not valid JSON: %v", err)
	}
	return init
}

func TestInstanceCreateRequestToAPIPayloadSSH(t *testing.T) {
	init := decodeInitialization(t, (&InstanceCreateRequest{
		Name:           "vm",
		ImageID:        "img-1",
		CPU:            2,
		RAM:            2,
		Disk:           20,
		NetworkID:      "net-1",
		SSHFingerprint: "aa:bb:cc",
	}).ToAPIPayload())

	auth, _ := init["auth"].(map[string]interface{})
	if auth["method"] != "ssh" || auth["fingerprint"] != "aa:bb:cc" {
		t.Errorf("unexpected ssh auth: %v", auth)
	}
	if _, ok := auth["password"]; ok {
		t.Errorf("password must not be present for ssh auth: %v", auth)
	}
}

func TestInstanceCreateRequestToAPIPayloadPassword(t *testing.T) {
	init := decodeInitialization(t, (&InstanceCreateRequest{
		Name:      "vm",
		ImageID:   "img-1",
		CPU:       2,
		RAM:       2,
		Disk:      20,
		NetworkID: "net-1",
		Password:  "Tvm7nayatel2Kx",
		AuthUser:  "matifali1",
	}).ToAPIPayload())

	auth, _ := init["auth"].(map[string]interface{})
	// The API only recognizes method "pwd" (not "password") and requires the
	// login user; anything else silently boots the VM with no login path.
	if auth["method"] != "pwd" {
		t.Errorf("auth method must be \"pwd\", got %v", auth["method"])
	}
	if auth["password"] != "Tvm7nayatel2Kx" || auth["user"] != "matifali1" {
		t.Errorf("unexpected pwd auth: %v", auth)
	}
	if _, ok := auth["fingerprint"]; ok {
		t.Errorf("fingerprint must not be present for pwd auth: %v", auth)
	}
}
