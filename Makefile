# JumpBoard: build, test and package with mygo.
#
# `make test` runs everything: the unit tests, the UI end-to-end tests
# (which need no window) and the real forwarding tests.

MYGO ?= go tool mygo
GO   ?= go

.PHONY: all test test-race test-ui test-engine cover build build-mac build-win shots fmt vet clean

all: test build

## Tests

test:
	$(GO) test ./... -count=1

test-race:
	$(GO) test ./... -race -count=1

test-ui:
	$(GO) test ./e2e/ -run 'TestMapping|TestValidation|TestTheme|TestFollow|TestForced|TestForce|TestWhitelist|TestLogs|TestTemplate|TestClone|TestToggle|TestHealth|TestExport|TestImport|TestWipe|TestAI|TestKeyboard|TestToast|TestCopyAddress|TestEngineSwitch|TestAppLogClear|TestAccessLogClear|TestHistoryRollback|TestDataRoundTrip' -count=1 -v

test-engine:
	$(GO) test ./internal/engine/ ./e2e/ -run 'TestTCP|TestUDP|TestHTTP|TestTLS|TestWhitelist|TestApply|TestCheck|TestSnapshot|TestQuit|TestProxy|TestMinimal|TestCustom|TestStop|TestRuleState' -count=1 -v

# The end-to-end tests exercise most of the app, so the coverage counts
# every internal package, whichever test touched it.
cover:
	$(GO) test ./... -coverpkg=./internal/... -coverprofile=coverage.out -count=1
	$(GO) tool cover -func=coverage.out | tail -1

## Builds

build: build-mac build-win

build-mac:
	$(MYGO) build -platform darwin/universal

build-win:
	$(MYGO) build -platform windows/amd64

## Housekeeping

fmt:
	gofmt -l -w .

vet:
	$(GO) vet ./...
	GOOS=windows $(GO) vet ./...

## Screenshots of every page, in both themes, without a window.

shots:
	$(GO) run ./cmd/shots

clean:
	rm -rf build coverage.out
