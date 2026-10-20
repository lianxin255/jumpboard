// Package engine runs the forwarding rules: the listeners this machine
// opens and the upstreams they lead to.
//
// The app no longer shells out to nginx. Everything a mapping needs — a
// TCP or UDP forward, an HTTP or TLS reverse proxy with host and path
// routing, an address whitelist and access logs — is a small amount of Go,
// and doing it in process makes applying a configuration transactional:
// the new listeners are opened first, the replaced ones are retargeted in
// place, and a failure leaves the previous set running.
//
// What is lost is the ability to write arbitrary nginx directives. Rendered
// configurations are still produced by the nginx package, for export and
// for reading old snapshots; the common directives a mapping carries are
// translated into engine settings, and anything else is reported rather
// than silently ignored.
package engine

import (
	"fmt"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// Status is the engine's state.
type Status string

// States.
const (
	Stopped Status = "stopped"
	Running Status = "running"
)

// RuleStatus is what one mapping is doing right now.
type RuleStatus struct {
	MappingID string
	Name      string
	// Listening is true once the rule's port is open and routing.
	Listening bool
	// Listen is the address the rule listens on, such as ":9000".
	Listen string
	// Backend is the upstream the rule forwards to.
	Backend string
	// Connections is how many client connections the rule served.
	Connections int64
	// Active is how many are open now.
	Active int
	// Err is why the rule is not running, empty when it is.
	Err string
	// Warning is what the rule asked for and the engine could not honor.
	Warning string
}

// Result is the outcome of applying a rule set.
type Result struct {
	OK bool
	// Warnings say what was not applied as written: directives the engine
	// does not know, a certificate that could not be read, a rule that
	// binds later.
	Warnings []string
	// Started and Stopped list the ports that changed.
	Started []string
	Stopped []string
	// Retargeted lists the ports whose upstream changed without closing.
	Retargeted []string
}

// Spec is a complete set of rules to run.
type Spec struct {
	// Mappings are the enabled rules.
	Mappings []model.Mapping
	// WhitelistIPs, when not empty, limits every rule to those addresses.
	WhitelistIPs []string
	// LogDir is where the access and error logs go.
	LogDir string
	// DialTimeout bounds a connection to an upstream; 10 seconds when zero.
	DialTimeout time.Duration
	// IdleTimeout closes a forwarded connection that carries nothing for
	// this long; five minutes when zero. A rule's proxy_timeout overrides
	// it.
	IdleTimeout time.Duration
}

// Engine runs a set of rules.
type Engine interface {
	// Apply replaces the running rules with spec.
	//
	// It is transactional: when a port cannot be opened, the rules that
	// were running stay running and the returned error says which port
	// failed and why.
	Apply(spec Spec) (Result, error)
	// Stop closes every listener.
	Stop() error
	// Status reports the state.
	Status() Status
	// Rules reports what each rule of the last applied spec is doing.
	Rules() []RuleStatus
	// Mode names the engine, for the window.
	Mode() string
}

// ErrBind is returned when a port cannot be opened.
type ErrBind struct {
	Port  int
	Proto string
	Err   error
}

func (e *ErrBind) Error() string {
	return fmt.Sprintf("%s 端口 %d 无法监听：%v", e.Proto, e.Port, e.Err)
}

func (e *ErrBind) Unwrap() error { return e.Err }
