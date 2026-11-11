//go:build wails && darwin

package webshell

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// The window's toolbar in its compact style: the traffic lights sit in a
// 38 pt strip instead of 52, centred on the page's own 38 px top rows.
static void pinkyCompactTitleBar(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSWindow *w in [NSApp windows]) {
			if (w.toolbar != nil) {
				w.toolbarStyle = NSWindowToolbarStyleUnifiedCompact;
			}
		}
	});
}
*/
import "C"

// compactTitleBar makes the hidden title bar's strip as tall as the page's
// top rows (ui/frontend/src/lib/window.ts), so the traffic lights line up
// with them.
func compactTitleBar() { C.pinkyCompactTitleBar() }
