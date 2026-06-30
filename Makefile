.PHONY: build run test tidy headless webhook-only webhook-test clean

BINARY := ops-agent
MAIN   := ./cmd/ops-agent
WEBHOOK_URL ?= http://127.0.0.1:8765/webhooks/github
WEBHOOK_SECRET ?= dev-secret

ifeq ($(OS),Windows_NT)
    BIN          := $(BINARY).exe
    BUILD_OUTPUT := -o $(BIN)
    RUN          := $(BIN)
    HEADLESS     := set OPS_AGENT_CI=1&& $(BIN)
    CLEAN        := cmd /c "del /f /q $(BIN) 2>nul"
else
    BIN          := $(BINARY)
    BUILD_OUTPUT := -o $(BIN)
    RUN          := ./$(BIN)
    HEADLESS     := OPS_AGENT_CI=1 $(RUN)
    CLEAN        := rm -f $(BIN)
endif

build:
	go build $(BUILD_OUTPUT) $(MAIN)

run: build
	$(RUN)

test:
	go test ./...

tidy:
	go mod tidy

headless: build
	$(HEADLESS)

webhook-only: build
	OPS_AGENT_WEBHOOK_ONLY=1 $(RUN)

webhook-test:
	@curl -sf http://127.0.0.1:8765/healthz >/dev/null || (echo "ops-agent 未运行或 webhook 未监听"; exit 1)
	@BODY='{"action":"opened","issue":{"number":99,"title":"webhook test","state":"open","html_url":"https://github.com/o/r/issues/99","labels":[],"assignees":[]},"repository":{"full_name":"o/r"}}'; \
	SIG=$$(printf '%s' "$$BODY" | openssl dgst -sha256 -hmac "$(WEBHOOK_SECRET)" | awk '{print "sha256="$$2}'); \
	curl -sS -X POST "$(WEBHOOK_URL)" \
	  -H "Content-Type: application/json" \
	  -H "X-GitHub-Event: issues" \
	  -H "X-Hub-Signature-256: $$SIG" \
	  -d "$$BODY" | cat; echo

clean:
	-$(CLEAN)
