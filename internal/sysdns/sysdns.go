package sysdns

// GetCurrent возвращает текущее состояние DNS системы (адаптер, DHCP или список серверов).
// Нужно вызвать до Enable, чтобы сохранить в State и потом Restore.
func GetCurrent() (*State, error) {
	return getCurrent()
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
// Требуются права администратора.
func Enable(dataDir string) error {
	state, err := getCurrent()
	if err != nil {
		return err
	}
	if err := SaveToFile(dataDir, state); err != nil {
		return err
	}
	if err := prepareForEnable(dataDir); err != nil {
		return err
	}
	return setSystemDNS(state.Adapter, false, []string{"127.0.0.1"}, dataDir)
}

// Disable восстанавливает DNS из сохранённого в dataDir состояния и удаляет бэкап.
// Требуются права администратора.
func Disable(dataDir string) error {
	state, err := LoadFromFile(dataDir)
	if err != nil {
		return err
	}
	if state == nil {
		return nil // не было включено — нечего восстанавливать
	}
	// На Linux всегда восстанавливаем из бэкапа resolv.conf (полное содержимое).
	if state.Platform == "linux" {
		err = setSystemDNS(state.Adapter, true, nil, dataDir)
	} else if state.WasDHCP {
		err = setSystemDNS(state.Adapter, true, nil, dataDir)
	} else {
		err = setSystemDNS(state.Adapter, false, state.Servers, dataDir)
	}
	if err != nil {
		return err
	}
	cleanupAfterDisable(dataDir)
	return RemoveBackup(dataDir)
}
