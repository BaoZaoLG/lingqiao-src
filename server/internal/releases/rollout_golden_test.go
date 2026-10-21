package releases

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type rolloutGolden struct {
	Version int `json:"version"`
	Rollout struct {
		Buckets []struct {
			ReleaseID string `json:"release_id"`
			Key       string `json:"key"`
			Bucket    int    `json:"bucket"`
		} `json:"buckets"`
	} `json:"rollout"`
	VersionCompare []struct {
		Candidate string `json:"candidate"`
		Current   string `json:"current"`
		Greater   bool   `json:"greater"`
	} `json:"version_compare"`
}

func TestRolloutAndVersionGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../docs/api/release-rollout-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden rolloutGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 {
		t.Fatalf("unexpected golden version %d", golden.Version)
	}

	for _, tc := range golden.Rollout.Buckets {
		sum := sha256.Sum256([]byte(tc.ReleaseID + "|" + tc.Key))
		bucket := int(binary.BigEndian.Uint64(sum[:8]) % 100)
		if bucket != tc.Bucket {
			t.Errorf("bucket for %s|%s = %d, want %d", tc.ReleaseID, tc.Key, bucket, tc.Bucket)
		}
		if !inRollout(tc.ReleaseID, tc.Bucket+1, ClientContext{MachineID: tc.Key}) {
			t.Errorf("%s|%s should be included at percent=%d", tc.ReleaseID, tc.Key, tc.Bucket+1)
		}
		if inRollout(tc.ReleaseID, tc.Bucket, ClientContext{MachineID: tc.Key}) {
			t.Errorf("%s|%s should be excluded at percent=%d", tc.ReleaseID, tc.Key, tc.Bucket)
		}
	}

	for _, tc := range golden.VersionCompare {
		if got := VersionGreater(tc.Candidate, tc.Current); got != tc.Greater {
			t.Errorf("VersionGreater(%q,%q) = %v, want %v", tc.Candidate, tc.Current, got, tc.Greater)
		}
	}
}
