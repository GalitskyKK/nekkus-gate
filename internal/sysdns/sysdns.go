package sysdns

import "runtime"

// GetCurrent возвращает текущее состояние DNS системы (адаптер, DHCP или список серверов).
// Нужно вызвать до Enable, чтобы сохранить в State и потом Restore.
func GetCurrent() (*State, error) {
	return getCurrent()
}

// SaveStateHostsMode сохраняет состояние с Mode=hosts (фильтр включён через файл hosts).
// При Disable по этому состоянию восстанавливается только hosts, DNS не трогаем.
func SaveStateHostsMode(dataDir string) error {
	return SaveToFile(dataDir, &State{Mode: ModeHosts, Platform: runtime.GOOS})
}

// prepareForEnable вызывается перед установкой 127.0.0.1 (на Linux — бэкап resolv.conf).
func prepareForEnable(dataDir string) error { return prepareForEnableImpl(dataDir) }

// cleanupAfterDisable вызывается после восстановления (на Linux — удалить бэкап resolv).
func cleanupAfterDisable(dataDir string) { cleanupAfterDisableImpl(dataDir) }

// SetSystemDNS выставляет DNS для указанного адаптера.
// useDHCP true — включить DHCP (servers игнорируется); на Linux — восстановить из бэкапа в dataDir.
// useDHCP false — статически выставить servers (например []string{"127.0.0.1"}).
func SetSystemDNS(adapter string, useDHCP bool, servers []string, dataDir string) error {
	return setSystemDNS(adapter, useDHCP, servers, dataDir)
}

// Enable сохраняет текущее DNS в dataDir и переключает систему на 127.0.0.1 (Gate).
// Требуются права администратора. На Windows ставит 127.0.0.1 на все подключённые интерфейсы.
func Enable(dataDir string) error {
	state, err := getCurrent()
	if err != nil {
		return err
	}
	state.Mode = ModeDNS
	if err := SaveToFile(dataDir, state); err != nil {
		return err
	}
	if err := prepareForEnable(dataDir); err != nil {
		return err
	}
	adapters := state.Adapters
	if len(adapters) == 0 {
		adapters = []string{state.Adapter}
	}
	for _, adapter := range adapters {
		if err := setSystemDNS(adapter, false, []string{"127.0.0.1"}, dataDir); err != nil {
			return err
		}
	}
	return nil
}

// Disable восстанавливает настройки из сохранённого состояния: DNS (mode=dns) или только hosts (mode=hosts).
// Для mode=hosts вызывающий должен вызвать hostsfilter.Disable(dataDir) отдельно; здесь только RemoveBackup для JSON.
func Disable(dataDir string) error {
	state, err := LoadFromFile(dataDir)
	if err != nil {
		return err
	}
	if state == nil {
		return nil // не было включено — нечего восстанавливать
	}
	if state.Mode == ModeHosts {
		// Восстановление hosts делается в API (hostsfilter.Disable). Здесь только удаляем наш state-файл.
		return RemoveBackup(dataDir)
	}
	// mode=dns (или пустой для совместимости) — восстанавливаем все адаптеры, на которых меняли DNS.
	adapters := state.Adapters
	if len(adapters) == 0 {
		adapters = []string{state.Adapter}
	}
	for _, adapter := range adapters {
		if state.Platform == "linux" {
			err = setSystemDNS(adapter, true, nil, dataDir)
		} else if state.WasDHCP {
			err = setSystemDNS(adapter, true, nil, dataDir)
		} else {
			err = setSystemDNS(adapter, false, state.Servers, dataDir)
		}
		if err != nil {
			return err
		}
	}
	cleanupAfterDisable(dataDir)
	return RemoveBackup(dataDir)
}
