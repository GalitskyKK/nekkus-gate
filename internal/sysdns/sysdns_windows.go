//go:build windows

package sysdns

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

func getCurrent() (*State, error) {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "config")
	cmd.Stderr = nil
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netsh show config: %w", err)
	}
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	state, err := parseNetshConfig(out)
	if err != nil {
		return nil, err
	}
	state.Platform = "windows"
	// Сначала — все подключённые из "netsh interface show interface" (англ. Connected / рус. Подключено).
	connected := getConnectedInterfaceNames()
	if len(connected) > 0 {
		state.Adapters = connected
	} else {
		// Иначе — все адаптеры с конфигом из show config (чтобы не промахнуться по локали).
		allFromConfig := parseNetshConfigAllAdapters(out)
		if len(allFromConfig) > 0 {
			state.Adapters = allFromConfig
		} else {
			state.Adapters = []string{state.Adapter}
		}
	}
	return state, nil
}

// parseNetshConfigAllAdapters возвращает имена всех адаптеров с конфигом из вывода "netsh interface ipv4 show config".
func parseNetshConfigAllAdapters(out []byte) []string {
	blocks := splitNetshBlocks(out)
	var names []string
	seen := make(map[string]bool)
	for _, block := range blocks {
		adapter := extractNetshAdapterName(block)
		if adapter == "" || seen[adapter] {
			continue
		}
		if !blockHasConfig(block) {
			continue
		}
		seen[adapter] = true
		names = append(names, adapter)
	}
	return names
}

// getConnectedInterfaceNames возвращает имена интерфейсов в состоянии "Connected"/"Подключено" из "netsh interface show interface".
func getConnectedInterfaceNames() []string {
	cmd := exec.Command("netsh", "interface", "show", "interface")
	cmd.Stderr = nil
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var names []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		// Англ. Connected, рус. Подключено, нем. Verbunden и т.д.
		if !strings.Contains(lower, "connected") && !strings.Contains(lower, "подключено") && !strings.Contains(lower, "verbunden") {
			continue
		}
		parts := regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(line), -1)
		if len(parts) >= 4 {
			name := strings.TrimSpace(strings.Join(parts[3:], " "))
			if name != "" && !strings.Contains(strings.ToLower(name), "loopback") {
				names = append(names, name)
			}
		}
	}
	return names
}

// parseNetshConfig разбирает вывод "netsh interface ipv4 show config".
// Приоритет: интерфейс с шлюзом по умолчанию (Default Gateway) — это активное подключение. Иначе первый с конфигом.
func parseNetshConfig(out []byte) (*State, error) {
	blocks := splitNetshBlocks(out)
	var withGateway, fallback *State
	for _, block := range blocks {
		adapter := extractNetshAdapterName(block)
		if adapter == "" {
			continue
		}
		if !blockHasConfig(block) {
			continue
		}
		wasDHCP, servers := parseNetshDNS(block)
		st := &State{Adapter: adapter, WasDHCP: wasDHCP, Servers: servers}
		if blockHasDefaultGateway(block) {
			withGateway = st
			break
		}
		if fallback == nil {
			fallback = st
		}
	}
	if withGateway != nil {
		return withGateway, nil
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("no connected interface with DNS found in netsh output")
}

func blockHasConfig(block []byte) bool {
	lower := bytes.ToLower(block)
	return bytes.Contains(lower, []byte("dns")) ||
		bytes.Contains(lower, []byte("dhcp")) ||
		bytes.Contains(lower, []byte("ip")) ||
		bytes.Contains(lower, []byte("ip-")) ||
		bytes.Contains(lower, []byte("subnet")) ||
		bytes.Contains(lower, []byte("маска")) ||
		bytes.Contains(lower, []byte("gateway"))
}

// blockHasDefaultGateway — у интерфейса есть шлюз по умолчанию (активное подключение к сети).
func blockHasDefaultGateway(block []byte) bool {
	lower := bytes.ToLower(block)
	return bytes.Contains(lower, []byte("default gateway")) ||
		bytes.Contains(lower, []byte("шлюз по умолчанию")) ||
		bytes.Contains(lower, []byte("domyślna brama"))
}

func splitNetshBlocks(out []byte) [][]byte {
	// Разбиваем по заголовку интерфейса (англ. или рус. и т.п.), чтобы не зависеть от пустых строк
	header := regexp.MustCompile(`(?m)^(?:Configuration for interface|Конфигурация интерфейса|Конфигурация для интерфейса|Konfiguracja interfejsu)\s*"([^"]+)"`)
	indices := header.FindAllIndex(out, -1)
	if len(indices) == 0 {
		// Fallback: по двойному переносу строки
		var blocks [][]byte
		start := 0
		for i := 0; i < len(out); i++ {
			if i+1 < len(out) && out[i] == '\n' && out[i+1] == '\n' {
				blocks = append(blocks, out[start:i+1])
				start = i + 2
				i++
			}
		}
		if start < len(out) {
			blocks = append(blocks, out[start:])
		}
		return blocks
	}
	var blocks [][]byte
	for i := 0; i < len(indices); i++ {
		start := indices[i][0]
		end := len(out)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		blocks = append(blocks, out[start:end])
	}
	return blocks
}

// Адаптер: английская, русская и общий шаблон (имя в кавычках после interface/интерфейса).
var reNetshAdapterEn = regexp.MustCompile(`Configuration for interface "([^"]+)"`)
var reNetshAdapterRu = regexp.MustCompile(`Конфигурация (?:для )?интерфейса "([^"]+)"`)
var reNetshAdapterGeneric = regexp.MustCompile(`(?:interface|интерфейса|interfejsu)\s*"([^"]+)"`)

func extractNetshAdapterName(block []byte) string {
	for _, re := range []*regexp.Regexp{reNetshAdapterEn, reNetshAdapterRu, reNetshAdapterGeneric} {
		m := re.FindSubmatch(block)
		if len(m) >= 2 {
			return string(m[1])
		}
	}
	return ""
}

// parseNetshDNS из блока ищет статические DNS или DHCP (англ./рус. и др. локали).
func parseNetshDNS(block []byte) (wasDHCP bool, servers []string) {
	text := string(block)
	lower := strings.ToLower(text)

	// Статические DNS: после маркера идёт IP или список
	for _, marker := range []string{
		"Statically Configured DNS Servers:",
		"Настроенные статически DNS-серверы:",
		"Statycznie skonfigurowane serwery DNS:",
	} {
		idx := strings.Index(lower, strings.ToLower(marker))
		if idx < 0 {
			continue
		}
		rest := text[idx+len(marker):]
		if nl := strings.IndexAny(rest, "\n\r"); nl >= 0 {
			rest = rest[:nl]
		}
		rest = strings.TrimSpace(rest)
		if rest != "" {
			servers = strings.Fields(rest)
			return false, servers
		}
	}

	// DHCP: при восстановлении ставим dhcp
	dhcpMarkers := []string{
		"dns servers configured through dhcp",
		"dhcp enabled: yes",
		"серверы dns, настроенные через dhcp",
		"включен dhcp: да",
		"dhcp włączone: tak",
	}
	for _, m := range dhcpMarkers {
		if strings.Contains(lower, m) {
			return true, nil
		}
	}
	return true, nil
}

func prepareForEnableImpl(dataDir string) error { return nil }
func cleanupAfterDisableImpl(dataDir string)    {}

func flushDNSCacheImpl() {
	cmd := exec.Command("ipconfig", "/flushdns")
	hideWindow(cmd)
	_ = cmd.Run()
}

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW — консоль не создаётся, не моргает.

// hideWindow скрывает консольное окно дочернего процесса (PowerShell/CMD не моргают).
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = createNoWindow
}

func setSystemDNS(adapter string, useDHCP bool, servers []string, dataDir string) error {
	if adapter == "" {
		return fmt.Errorf("adapter name is required")
	}
	// Пробуем netsh и PowerShell (на Windows 11 иногда срабатывает только один из них).
	errN := setSystemDNSNetsh(adapter, useDHCP, servers)
	errP := setSystemDNSPowerShell(adapter, useDHCP, servers)
	if errN != nil && errP != nil {
		return fmt.Errorf("netsh: %w; PowerShell: %v", errN, errP)
	}
	return nil
}

func setSystemDNSNetsh(adapter string, useDHCP bool, servers []string) error {
	adapterQuoted := `"` + adapter + `"`
	var cmd *exec.Cmd
	if useDHCP {
		cmd = exec.Command("netsh", "interface", "ipv4", "set", "dns", "name="+adapterQuoted, "dhcp")
	} else {
		if len(servers) == 0 {
			return fmt.Errorf("need at least one DNS server when not using DHCP")
		}
		cmd = exec.Command("netsh", "interface", "ipv4", "set", "dns", "name="+adapterQuoted, "static", servers[0])
	}
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, bytes.TrimSpace(out))
	}
	for i := 1; i < len(servers); i++ {
		cmd = exec.Command("netsh", "interface", "ipv4", "add", "dns", "name="+adapterQuoted, servers[i], strconv.Itoa(i+1))
		hideWindow(cmd)
		if out2, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("netsh add dns: %w (%s)", err, bytes.TrimSpace(out2))
		}
	}
	return nil
}

func setSystemDNSPowerShell(adapter string, useDHCP bool, servers []string) error {
	if useDHCP {
		script := fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceAlias %q -ResetServerAddresses`, adapter)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
		hideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w (%s)", err, bytes.TrimSpace(out))
		}
		return nil
	}
	if len(servers) == 0 {
		return fmt.Errorf("need at least one DNS server")
	}
	var quoted []string
	for _, s := range servers {
		quoted = append(quoted, `"`+s+`"`)
	}
	addrList := strings.Join(quoted, ",")
	script := fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceAlias %q -ServerAddresses @(%s)`, adapter, addrList)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, bytes.TrimSpace(out))
	}
	return nil
}
