.PHONY: build run test tidy headless clean

BINARY := ops-agent
MAIN   := ./cmd/ops-agent

build:
	go build -o $(BINARY).exe $(MAIN)

run: build
	./$(BINARY).exe

test:
	go test ./...

tidy:
	go mod tidy

headless:
	set OPS_AGENT_CI=1&& $(BINARY).exe

clean:
	del /f $(BINARY).exe 2>nul || true
