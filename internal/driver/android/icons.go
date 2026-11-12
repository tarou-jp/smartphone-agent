package android

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/driver/android/companion"
)

// packageRE is the syntax of an Android package name: two or more Java
// identifiers joined by dots.
var packageRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+$`)

// maxPackageLen bounds a package name; Android itself allows 255.
const maxPackageLen = 255

// AppIcon implements driver.AppIconer through the companion app. It uses
// the service only while it is on for the driver (an agent works on the
// phone) and never turns it on for an icon: an accessibility service that
// is on shows to the phone's apps. Otherwise, and with an app from before
// FeatureAppIcons, it returns driver.ErrUnsupported. A package the phone
// does not have, or does not show the companion (Android 11 and later show
// it only apps with a launcher entry), and a malformed package name give
// driver.ErrNotFound.
func (d *Device) AppIcon(ctx context.Context, app string) ([]byte, error) {
	if len(app) > maxPackageLen || !packageRE.MatchString(app) {
		return nil, fmt.Errorf("android: app icon: %q is not a package name: %w", app, driver.ErrNotFound)
	}
	c := d.comp.iconClient()
	if c == nil {
		return nil, fmt.Errorf("android: app icons need the companion app, on and from 2026-11-12 or later: %w", driver.ErrUnsupported)
	}
	png, err := c.AppIcon(ctx, app, driver.MaxAppIconSide)
	if err != nil {
		var cerr *companion.Error
		if errors.Is(err, companion.ErrNotRunning) || (errors.As(err, &cerr) && cerr.Code == "unknown_command") {
			// The service went off meanwhile, or an app that lists the
			// feature but lacks the command.
			return nil, fmt.Errorf("android: icon of %s: %w: %w", app, err, driver.ErrUnsupported)
		}
		return nil, fmt.Errorf("android: icon of %s: %w", app, err)
	}
	return png, nil
}

// iconClient returns the companion client while the service is on for the
// driver and the app reads icons; nil otherwise.
func (k *companionLink) iconClient() *companion.Client {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.on || k.info == nil || !k.info.Has(companion.FeatureAppIcons) {
		return nil
	}
	return k.client
}

var _ driver.AppIconer = (*Device)(nil)
