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

# The UI tests are the windowless end-to-end ones plus the appt-layer and
# serial-layer behavior tests they lean on (forms, deeplinks, updates,
# reorder, terminal, serial, login items, …); together the -run families
# below cover every test those packages hold, so a new one that escapes
# the pattern is a gap to fix here, not a test that silently never runs.
test-ui:
	$(GO) test ./e2e/ ./internal/appt/ ./internal/serial/ -run 'TestMapping|TestValidation|TestTheme|TestFollow|TestForced|TestForce|TestWhitelist|TestLog|TestTemplate|TestClone|TestToggle|TestHealth|TestExport|TestImport|TestWipe|TestAI|TestAskAI|TestAIFilter|TestClearAIFilter|TestAIChat|TestChat|TestKeyboard|TestToast|TestCopyAddress|TestEngineSwitch|TestAppLog|TestAccessLog|TestHistoryRollback|TestDataRoundTrip|TestBatch|TestConf|TestExternalConfEdit|TestDiagnose|TestDirective|TestDrag|TestMultiSelect|TestMoveMapping|TestReorder|TestDrop|TestFilter|TestForm|TestGroup|TestHotkey|TestSetHotkey|TestInstallUpdate|TestCheckUpdate|TestInterpret|TestOnClose|TestOpenAtLogin|TestSetOpenAtLogin|TestSyncOpenAtLogin|TestApplyNotifies|TestPaste|TestParseDeeplink|TestHandleDeeplink|TestRate|TestSampler|TestThroughput|TestTruncate|TestSearch|TestSettings|TestSetTheme|TestSidebar|TestTerminal|TestSerial|TestTraffic|TestShowTraffic|TestRuleSnapshot|TestWatcher' -count=1 -v

# The engine tests: the forwarding engine's own package plus the
# end-to-end ones that drive traffic through a real app. The families
# cover every test in ./internal/engine/.
test-engine:
	$(GO) test ./internal/engine/ ./e2e/ -run 'TestTCP|TestUDP|TestHTTP|TestTLS|TestWhitelist|TestApply|TestCheck|TestSnapshot|TestQuit|TestProxy|TestMinimal|TestCustom|TestStop|TestRuleState|TestRuleTracks|TestEventsFire|TestAccessLog|TestBadCert|TestPathPrefix|TestReapply|TestRetarget|TestPortConflict|TestSamePort|TestTimeout|TestUnknownHost|TestUnsupported' -count=1 -v

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
