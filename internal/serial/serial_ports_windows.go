//go:build windows

package serial

import (
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// commNameRE accepts a Windows COMM name, COM5 or COM15. The plain
// enumeration is safe because it never dials a device.
var commNameRE = regexp.MustCompile(`^COM\d+$`)

// commNames reads the serial devices Windows has: their COMM names from
// HKLM\SYSTEM\CurrentControlSet\Services\Class\{4d36e978-e325-11ce-bfc1-
// 08002be10318}\<subkey>\Parameters (PortName), and, where the subkey's
// FriendlyName reads cleanly, that as the hint. A device a driver did not
// name is not listed.
func commNames() (names []string, descs map[string]string) {
	descs = map[string]string{}
	base := `SYSTEM\CurrentControlSet\Services\Class\{4d36e978-e325-11ce-bfc1-08002be10318}`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, base, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil, descs
	}
	defer k.Close()
	subs, err := k.ReadSubKeyNames(0)
	if err != nil {
		return nil, descs
	}
	for _, sub := range subs {
		sk, err := registry.OpenKey(registry.LOCAL_MACHINE, base+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		port, _, err := sk.GetStringValue("PortName")
		if err != nil || port == "" {
			sk.Close()
			continue
		}
		port = strings.ToUpper(strings.TrimSpace(port))
		if !commNameRE.MatchString(port) {
			sk.Close()
			continue
		}
		names = append(names, port)
		if hint, _, err := sk.GetStringValue("FriendlyName"); err == nil {
			descs[port] = strings.TrimSpace(hint)
		}
		sk.Close()
	}
	return names, descs
}
