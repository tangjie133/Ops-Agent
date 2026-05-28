package notify

import "github.com/ZzedJay/Ops-Agent/internal/config"

func FromAppConfig(cfg *config.Config) *Multi {
	if cfg == nil {
		return NewMulti()
	}
	var list []Notifier
	add := func(enabled bool, url string, factory func(string) Notifier) {
		if enabled && url != "" {
			list = append(list, factory(url))
		}
	}
	for name, ch := range cfg.Notify.Channels {
		switch name {
		case "slack":
			add(ch.Enabled, ch.WebhookURL, NewSlack)
		case "feishu":
			add(ch.Enabled, ch.WebhookURL, NewFeishu)
		case "dingtalk":
			add(ch.Enabled, ch.WebhookURL, NewDingTalk)
		}
	}
	return NewMulti(list...)
}
