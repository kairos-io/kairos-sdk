package clusterplugin

import (
	"github.com/twpayne/go-vfs/v5"
)

var filesystem vfs.FS

func init() {
	filesystem = vfs.OSFS
}
