package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

const autoCheckInterval = 6 * time.Hour

// UpdateService bridges the frontend and the Wails built-in updater.
type UpdateService struct{}

// Version returns the running application version.
func (s *UpdateService) Version() string {
	return Version
}

// ServiceStartup starts the background update check: once shortly after launch
// (silently, so it never pops a window), then every autoCheckInterval.
// A found update is announced through the framework's
// "wails:updater:update-available" event, which the UI turns into a banner.
func (s *UpdateService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.checkSilently(ctx)
		}

		ticker := time.NewTicker(autoCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkSilently(ctx)
			}
		}
	}()
	return nil
}

func (s *UpdateService) checkSilently(ctx context.Context) {
	app := application.Get()
	if app == nil || app.Updater == nil || app.Updater.State() == updater.StateUnconfigured {
		return
	}
	if _, err := app.Updater.Check(ctx); err != nil {
		log.Printf("自动检查更新失败: %v", err)
	}
}

// CheckForUpdates opens the built-in update window and runs the framework's
// check → download → verify → install → restart flow. The call returns as soon
// as the flow starts; progress is reported in the update window.
func (s *UpdateService) CheckForUpdates() error {
	app := application.Get()
	if app == nil || app.Updater == nil || app.Updater.State() == updater.StateUnconfigured {
		return errors.New("自动更新未配置")
	}
	go func() {
		if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
			log.Printf("检查更新失败: %v", err)
		}
	}()
	return nil
}
