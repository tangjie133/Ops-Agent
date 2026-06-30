package config

import "testing"

func TestValidateWebhookListen(t *testing.T) {
	if err := ValidateWebhookListen("127.0.0.1:8765"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWebhookListen("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeWebhookPath(t *testing.T) {
	if got := NormalizeWebhookPath("webhook"); got != "/webhook" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatWebhookSecretDisplay(t *testing.T) {
	if FormatWebhookSecretDisplay("") != "未设置" {
		t.Fatal()
	}
}

func TestWebhookConnFields(t *testing.T) {
	fields := WebhookConnFields()
	if len(fields) != 4 || fields[3].Title != "Public URL" || fields[3].Description == "" {
		t.Fatal("connection field docs incomplete")
	}
}
