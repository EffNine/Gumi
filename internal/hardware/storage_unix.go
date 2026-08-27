//go:build !windows

package hardware

import (
	"fmt"
	"syscall"
)

// linuxFSTypes maps statfs f_type magic numbers to filesystem names.
var linuxFSTypes = map[int64]string{
	0xEF53:     "ext2/ext3/ext4",
	0x9123683E: "btrfs",
	0x58465342: "xfs",
	0x52654973: "reiserfs",
	0x01021994: "tmpfs",
	0x2FC12FC1: "zfs",
	0x20120186: "exfat",
	0x65735546: "fuseblk",
	0xF15F5E50: "f2fs",
	0x5346544E: "ntfs (3g)",
	0x69636C65: "overlayfs",
}

// networkFS are filesystems where mmap-backed weight loading is unreliable.
var networkFS = map[int64]bool{
	0xFF534D4D: true, // nfs
	0xFF534D42: true, // cifs/smb
	0x01021997: true, // 9p
	0x5346414F: true, // afs
}

// probeStorage inspects the filesystem containing path.
func probeStorage(path string) Storage {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Storage{Path: path}
	}
	ftype := int64(st.Type)
	name, ok := linuxFSTypes[ftype]
	if !ok {
		name = fmt.Sprintf("unknown(0x%X)", uint32(ftype))
	}
	return Storage{
		Path:        path,
		FSType:      name,
		MmapCapable: !networkFS[ftype],
		Known:       true,
	}
}
