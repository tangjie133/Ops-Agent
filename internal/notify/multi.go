package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type webhookNotifier struct {
	name string
	url  string
	fmt  func(Alert) ([]byte, error)
}

func (n *webhookNotifier) Send(ctx context.Context, alert Alert) error {
	if n.url == "" {
		return fmt.Errorf("%s: webhook url empty", n.name)
	}
	body, err := n.fmt(alert)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s webhook: HTTP %s", n.name, resp.Status)
	}
	return nil
}

func NewSlack(url string) Notifier {
	return &webhookNotifier{name: "slack", url: url, fmt: slackPayload}
}

func NewFeishu(url string) Notifier {
	return &webhookNotifier{name: "feishu", url: url, fmt: feishuPayload}
}

func NewDingTalk(url string) Notifier {
	return &webhookNotifier{name: "dingtalk", url: url, fmt: dingtalkPayload}
}

func slackPayload(a Alert) ([]byte, error) {
	return json.Marshal(map[string]string{"text": FormatBody(a)})
}

func feishuPayload(a Alert) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": FormatBody(a)},
	})
}

func dingtalkPayload(a Alert) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": FormatBody(a)},
	})
}

type Multi struct {
	notifiers []Notifier
}

func NewMulti(notifiers ...Notifier) *Multi {
	return &Multi{notifiers: notifiers}
}

func (m *Multi) Send(ctx context.Context, alert Alert) error {
	if len(m.notifiers) == 0 {
		return nil
	}
	var errs []string
	for _, n := range m.notifiers {
		if err := n.Send(ctx, alert); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("notify: %s", strings.Join(errs, "; "))
	}
	return nil
}
