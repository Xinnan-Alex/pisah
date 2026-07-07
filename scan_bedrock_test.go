package main

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func TestBedrockMessageText(t *testing.T) {
	blocks := []types.ContentBlock{
		&types.ContentBlockMemberText{Value: `{"merchant":"Cafe"}`},
	}
	got := bedrockMessageText(blocks)
	if got != `{"merchant":"Cafe"}` {
		t.Fatalf("got %q", got)
	}
}

func TestBedrockConverseModelID_mapsFoundationModel(t *testing.T) {
	got := bedrockConverseModelID("anthropic.claude-haiku-4-5-20251001-v1:0")
	want := "global.anthropic.claude-haiku-4-5-20251001-v1:0"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBedrockConverseModelID_keepsInferenceProfile(t *testing.T) {
	profile := "global.anthropic.claude-haiku-4-5-20251001-v1:0"
	if bedrockConverseModelID(profile) != profile {
		t.Fatal("expected inference profile to pass through unchanged")
	}
}

func TestBedrockRetryable_throttling(t *testing.T) {
	if !bedrockRetryable(fmt.Errorf("ThrottlingException: rate exceeded")) {
		t.Fatal("expected retryable")
	}
	if bedrockRetryable(fmt.Errorf("ValidationException: bad model")) {
		t.Fatal("expected not retryable")
	}
}
