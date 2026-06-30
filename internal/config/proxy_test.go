package config

import "testing"

func TestProxyValidate(t *testing.T) {
	p := ProxyConfig{Enabled: true, HTTPSProxy: "http://127.0.0.1:7890"}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.HTTPSProxy = "bad"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestProxySummary(t *testing.T) {
	if (&ProxyConfig{}).Summary() != "关闭" {
		t.Fatal()
	}
	s := (&ProxyConfig{Enabled: true, HTTPSProxy: "http://127.0.0.1:7890"}).Summary()
	if s == "" || s == "关闭" {
		t.Fatalf("summary=%q", s)
	}
}
