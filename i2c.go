package i2c

import (
	"fmt"
	"syscall"
)

const (
	RETRIES     = 0x0701
	TIMEOUT     = 0x0702
	SLAVE       = 0x0703
	TENBIT      = 0x0704
	FUNCS       = 0x0705
	SLAVE_FORCE = 0x0706
	RDWR        = 0x0707
	PEC         = 0x0708
	SMBUS       = 0x0720
)

type File struct {
	bus int
	fd  int
}

func Open(bus int) (*File, error) {
	name := fmt.Sprintf("/dev/i2c-%d", bus)
	fd, err := syscall.Open(name, syscall.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	f := new(File)
	f.bus = bus
	f.fd = fd
	return f, nil
}

func (f *File) Close() {
	syscall.Close(f.fd)
}

func (f *File) SetTenbit(enable bool) error {
	var val uintptr
	if enable {
		val = 1
	}
	return f.Ioctl(TENBIT, val)
}

func (f *File) SetPEC(enable bool) error {
	var val uintptr
	if enable {
		val = 1
	}
	return f.Ioctl(PEC, val)
}

func (f *File) SetSlaveAddr(addr uint16, force bool) error {
	if force {
		return f.Ioctl(SLAVE_FORCE, uintptr(addr))
	} else {
		return f.Ioctl(SLAVE, uintptr(addr))
	}
}

func (f *File) Ioctl(cmd int, msg uintptr) error {
	_, _, e := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(f.fd),
		uintptr(cmd),
		msg,
	)
	if e == 0 {
		return nil
	}
	return e
}
