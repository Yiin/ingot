package theme

// gotk4 wraps GTK and Pango but not fontconfig, and fontconfig only ships
// FcConfigAppFontAddFile (no ...AddMemory in the version on the dev
// machine), so the bundled font has to land on disk before Pango can see
// it. This mirrors internal/layershell's approach: hand-write the ~1
// function of cgo a missing binding needs, rather than pull in a wrapper
// library for it.

/*
#cgo pkg-config: fontconfig
#include <fontconfig/fontconfig.h>
#include <stdlib.h>

static int ingot_register_app_font(const char *path) {
	return FcConfigAppFontAddFile(NULL, (const FcChar8 *)path);
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/Yiin/ingot/internal/ui/assets"
)

// registerBundledFont writes the embedded Inter Variable font to the user's
// cache directory (skipping the write if it is already there unchanged)
// and registers it with fontconfig as an application font, so Pango can
// find "Inter" without it being installed system-wide.
func registerBundledFont() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("theme: resolve cache dir: %w", err)
	}

	path := filepath.Join(cacheDir, "ingot", "fonts", "InterVariable.ttf")
	if err := writeIfChanged(path, assets.InterVariableTTF); err != nil {
		return fmt.Errorf("theme: write bundled font: %w", err)
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if C.ingot_register_app_font(cPath) == 0 {
		return fmt.Errorf("theme: fontconfig rejected bundled font %s", path)
	}
	return nil
}

func writeIfChanged(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
