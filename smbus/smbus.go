package smbus

import (
	"unsafe"

	"go-i2c"
)

// S     (1 bit) : Start bit
// P     (1 bit) : Stop bit
// Rd/Wr (1 bit) : Read/Write bit. Rd equals 1, Wr equals 0.
// A, NA (1 bit) : Accept and reverse accept bit.
// Addr  (7 bits): I2C 7 bit address. Note that this can be expanded as usual
//                 to get a 10 bit I2C address.
// Comm  (8 bits): Command byte, a data byte which often selects a register on
//                 the device.
// Data  (8 bits): A plain data byte. Sometimes, I write DataLow, DataHigh
//                 for 16 bit data.
// Count (8 bits): A data byte containing the length of a block operation.
// [..]: Data sent by I2C device, as opposed to data sent by the host adapter.

// SMBus Rd/Wr bit
type RWBit byte

const (
	RWBitWrite RWBit = iota
	RWBitRead
)

// SMBus transaction types
type TXType uint32

const (
	TXTypeQuick TXType = iota
	TXTypeByte
	TXTypeByteData
	TXTypeWordData
	TXTypeProcessCall
	TXTypeBlockData
	TXTypeI2CBlockBroken
	TXTypeBlockProcessCall // SMBus 2.0
	TXTypeI2CBlockData
)

type Msg struct {
	RW      RWBit
	Command byte
	pad     [2]byte
	TX      TXType
	Data    uintptr
}

type File struct {
	f *i2c.File
}

func Open(bus int) (*File, error) {
	dev, err := i2c.Open(bus)
	if err != nil {
		return nil, err
	}
	f := new(File)
	f.f = dev
	return f, nil
}

func (f *File) Close() {
	f.f.Close()
}

func (f *File) SetTenbit(enable bool) error {
	return f.f.SetTenbit(enable)
}

func (f *File) SetPEC(enable bool) error {
	return f.f.SetPEC(enable)
}

func (f *File) SetSlaveAddr(addr uint16, force bool) error {
	return f.f.SetSlaveAddr(addr, force)
}

// WriteQuick sends a single bit to the device,
// at the place of the Rd/Wr bit.
// There is no equivalent Read Quick command.
//
// A Addr Rd/Wr [A] P
func (f *File) WriteQuick(rwbit RWBit) error {
	m := Msg{
		RW: rwbit,
		TX: TXTypeQuick,
	}
	return f.Do(&m)
}

// ReadByte reads a single byte from a device,
// without specifying a device register.
// Some devices are so simple that this interface is enough;
// for others, it is a shorthand if you want to read the same register as in
// the previous SMBus command.
//
// S Addr Rd [A] [Data] NA P
func (f *File) ReadByte() (byte, error) {
	var data [4]byte
	m := Msg{
		RW:   RWBitRead,
		TX:   TXTypeByte,
		Data: uintptr(unsafe.Pointer(&data[0])),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// WriteByte is the reverse of Read Byte:
// it sends a single byte to a device.
// See Read Byte for more information.
//
// S Addr Wr [A] Data [A] P
func (f *File) WriteByte(data byte) error {
	m := Msg{
		RW:      RWBitWrite,
		Command: data,
		TX:      TXTypeByte,
	}
	return f.Do(&m)
}

// ReadByteData reads a single byte from a device,
// from a designated register.
// The register is specified through the Comm byte.
//
// S Addr Wr [A] Comm [A] S Addr Rd [A] [Data] NA P
func (f *File) ReadByteData(reg byte) (byte, error) {
	var data [4]byte
	m := Msg{
		RW:      RWBitRead,
		Command: reg,
		TX:      TXTypeByteData,
		Data:    uintptr(unsafe.Pointer(&data[0])),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// ReadWordData is very like Read Byte Data; again, data is read from a
// device, from a designated register that is specified through the Comm
// byte. But this time, the data is a complete word (16 bits).
//
// S Addr Wr [A] Comm [A] S Addr Rd [A] [DataLow] A [DataHigh] NA P
func (f *File) ReadWordData(reg byte) (uint16, error) {
	var data uint16
	m := Msg{
		RW:      RWBitRead,
		Command: reg,
		TX:      TXTypeWordData,
		Data:    uintptr(unsafe.Pointer(&data)),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	return data, nil
}

// WriteByteData writes a single byte to a device,
// to a designated register.
// The register is specified through the Comm byte. This is the opposite of
// the Read Byte Data command.
//
// S Addr Wr [A] Comm [A] Data [A] P
func (f *File) WriteByteData(reg, data byte) error {
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeByteData,
		Data:    uintptr(unsafe.Pointer(&data)),
	}
	return f.Do(&m)
}

// WriteWordData is the opposite operation of the Read Word Data command.
// 16 bits of data is read from a device,
// from a designated register that is specified through the Comm byte.
//
// S Addr Wr [A] Comm [A] DataLow [A] DataHigh [A] P
func (f *File) WriteWordData(reg byte, data uint16) error {
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeWordData,
		Data:    uintptr(unsafe.Pointer(&data)),
	}
	return f.Do(&m)
}

// ProcessCall selects a device register (through the Comm byte),
// sends 16 bits of data to it, and reads 16 bits of data in return.
//
// S Addr Wr [A] Comm [A] DataLow [A] DataHigh [A]
//                           S Addr Rd [A] [DataLow] A [DataHigh] NA P
func (f *File) ProcessCall(reg byte, data uint16) (uint16, error) {
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeProcessCall,
		Data:    uintptr(unsafe.Pointer(&data)),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	return data, nil
}

// ReadBlockData reads a block of up to 32 bytes from a device,
// from a designated register that is specified through the Comm byte.
// The amount of data is specified by the device in the Count byte.
//
// S Addr Wr [A] Comm [A]
//            S Addr Rd [A] [Count] A [Data] A [Data] A ... A [Data] NA P
func (f *File) ReadBlockData(reg byte) ([]byte, error) {
	var data [34]byte
	m := Msg{
		RW:      RWBitRead,
		Command: reg,
		TX:      TXTypeBlockData,
		Data:    uintptr(unsafe.Pointer(&data[0])),
	}
	err := f.Do(&m)
	if err != nil {
		return nil, err
	}
	n := data[0]
	return data[1 : n+1], nil
}

// WriteBlockData is the opposite of the Block Read command.
// this writes up to 32 bytes to a device,
// to a designated register that is specified through the Comm byte.
// The amount of data is specified in the Count byte.
//
// S Addr Wr [A] Comm [A] Count [A] Data [A] Data [A] ... [A] Data [A] P
func (f *File) WriteBlockData(reg byte, data []byte) (int, error) {
	var buf [34]byte
	n := copy(buf[1:], data)
	buf[0] = byte(n)
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeBlockData,
		Data:    uintptr(unsafe.Pointer(&buf[0])),
	}
	return n, f.Do(&m)
}

// BlockProcessCall was introduced in Revision 2.0 of the specification.
// This command selects a device register (through the Comm byte), sends
// 1 to 31 bytes of data to it, and reads 1 to 31 bytes of data in return.
//
// S Addr Wr [A] Comm [A] Count [A] Data [A] ...
//                              S Addr Rd [A] [Count] A [Data] ... A P
func (f *File) BlockProcessCall(reg byte, p []byte) (int, error) {
	var data [34]byte
	n := copy(data[1:], p)
	data[0] = byte(n)
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeBlockProcessCall,
		Data:    uintptr(unsafe.Pointer(&data[0])),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	n = int(data[0])
	copy(p, data[1:n+1])
	return n, nil
}

// The following I2C block transactions are supported by the
// SMBus layer and are described here for completeness.
// I2C block transactions do not limit the number of bytes transferred
// but the SMBus layer places a limit of 32 bytes.

// SMBusReadI2CBlockData reads a block of bytes from a device, from a
// designated register that is specified through the Comm byte.
//
// S Addr Wr [A] Comm [A]
//            S Addr Rd [A] [Data] A [Data] A ... A [Data] NA P
func (f *File) ReadI2CBlockData(reg byte, p []byte) (int, error) {
	var data [34]byte
	n := len(p)
	if n > 32 {
		n = 32
	}
	data[0] = byte(n)
	var tx TXType
	if n == 32 {
		tx = TXTypeI2CBlockBroken
	} else {
		tx = TXTypeI2CBlockData
	}
	m := Msg{
		RW:      RWBitRead,
		Command: reg,
		TX:      tx,
		Data:    uintptr(unsafe.Pointer(&data[0])),
	}
	err := f.Do(&m)
	if err != nil {
		return 0, err
	}
	n = int(data[0])
	copy(p, data[1:n+1])
	return n, nil
}

// WriteI2CBlockData is the opposite of the Block Read command.
// This writes bytes to a device,
// to a designated register that is specified through the Comm byte.
// Note that command lengths of 0, 2, or more bytes are supported as they are
// indistinguishable from data.
//
// S Addr Wr [A] Comm [A] Data [A] Data [A] ... [A] Data [A] P
func (f *File) WriteI2CBlockData(reg byte, data []byte) (int, error) {
	var buf [34]byte
	n := copy(buf[1:], data)
	buf[0] = byte(n)
	m := Msg{
		RW:      RWBitWrite,
		Command: reg,
		TX:      TXTypeI2CBlockBroken,
		Data:    uintptr(unsafe.Pointer(&buf[0])),
	}
	return n, f.Do(&m)
}

func (f *File) Do(msg *Msg) error {
	return f.f.Ioctl(i2c.SMBUS, uintptr(unsafe.Pointer(msg)))
}
