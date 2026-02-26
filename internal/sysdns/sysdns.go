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

// Enable переключает систему на 127.0.0.1 (Gate). Состояние сохраняется только после успешной смены DNS.
// Требуются права администратора. Если запущен через Hub без админа — смена DNS не пройдёт, бэкап не создаётся.
func Enable(dataDir string) error {
	state, err := getCurrent()
	if err != nil {
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
	state.Mode = ModeDNS
	if err := SaveToFile(dataDir, state); err != nil {
		return err
	}
	flushDNSCache()
	return nil
}

// flushDNSCache вызывается после смены DNS (на Windows — ipconfig /flushdns).
func flushDNSCache() { flushDNSCacheImpl() }

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
	// mode=dns — восстанавливаем DNS на всех адаптерах. При ошибке (нет прав, элемент не найден) всё равно снимаем бэкап.
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
		// Игнорируем ошибку (нет прав при запуске через Hub, «Элемент не найден» и т.п.) — снимаем бэкап в любом случае.
		_ = err
	}
	cleanupAfterDisable(dataDir)
	return RemoveBackup(dataDir)
}
