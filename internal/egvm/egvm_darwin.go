package egvm

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"google.golang.org/protobuf/encoding/protodelim"
)

// VM is a running machine.
type VM struct {
	vm      *vz.VirtualMachine
	socket  *vz.VirtioSocketDevice
	stopped chan struct{}
	err     error
	closers []io.Closer
	copied  chan struct{}
}

// Boot starts the machine with Virtualization.framework, copying its console to console. The
// process needs the com.apple.security.virtualization entitlement.
func Boot(m Machine, console io.Writer) (_ *VM, err error) {
	var (
		config *vz.VirtualMachineConfiguration
		vm     *vz.VirtualMachine
		nfs    *vz.VirtioSocketListener
	)

	// the console's input is a pipe nothing writes to; its output is copied to console.
	in, stdin, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdout, out, err := os.Pipe()
	if err != nil {
		return nil, errorsx.Compact(err, in.Close(), stdin.Close())
	}
	t := &VM{stopped: make(chan struct{}), copied: make(chan struct{}), closers: []io.Closer{in, stdin, out, stdout}}
	defer func() {
		if err != nil {
			t.close()
		}
	}()

	if config, err = configure(m, in, out); err != nil {
		return nil, err
	}
	if vm, err = vz.NewVirtualMachine(config); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine")
	}

	// the root is served before the machine starts, so vminit's first mount finds it.
	if nfs, err = vm.SocketDevices()[0].Listen(NFSPort); err != nil {
		return nil, errorsx.Wrap(err, "unable to listen for the machine's root")
	}
	t.closers = append(t.closers, nfs)
	go m.Root(nfs)

	go func() {
		defer close(t.copied)
		_, cause := io.Copy(console, stdout)
		errorsx.Log(errorsx.Ignore(cause, os.ErrClosed))
	}()

	if err = vm.Start(); err != nil {
		return nil, errorsx.Wrap(err, "unable to start the machine")
	}

	t.vm, t.socket = vm, vm.SocketDevices()[0]
	go func() {
		defer close(t.stopped)
		t.err = wait(vm)
	}()

	return t, nil
}

// Exec runs req in the machine with stdin, stdout and stderr as its stdio, waits for it, and
// returns its exit code. It waits for the machine to serve commands. Cancelling ctx kills the
// command.
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (code int32, err error) {
	var (
		id      uint64
		control net.Conn
		streams [3]net.Conn
		resp    execproxy.ExecResponse
		wg      sync.WaitGroup
	)

	if err = binary.Read(rand.Reader, binary.BigEndian, &id); err != nil {
		return -1, err
	}

	if control, err = t.connect(ctx, id, StreamControl); err != nil {
		return -1, err
	}
	defer control.Close()
	stop := context.AfterFunc(ctx, func() { control.Close() })
	defer stop()

	if _, err = protodelim.MarshalTo(control, req); err != nil {
		return -1, errorsx.Wrap(err, "unable to send the command")
	}

	for i := range streams {
		if streams[i], err = t.connect(ctx, id, uint8(i)); err != nil {
			return -1, err
		}
		defer streams[i].Close()
	}

	// the stdin stream is a connection of its own, so closing it is the command's end of input.
	go func() {
		if stdin != nil {
			io.Copy(streams[StreamStdin], stdin)
		}
		streams[StreamStdin].Close()
	}()
	wg.Go(func() { io.Copy(stdout, streams[StreamStdout]) })
	wg.Go(func() { io.Copy(stderr, streams[StreamStderr]) })

	if err = protodelim.UnmarshalFrom(bufio.NewReader(control), &resp); err != nil {
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
		return -1, errorsx.Wrap(err, "unable to read the command's exit")
	}
	wg.Wait()

	return resp.Exit, nil
}

// connect opens the stream of command id, retrying until the machine serves commands.
func (t *VM) connect(ctx context.Context, id uint64, stream uint8) (net.Conn, error) {
	for {
		conn, err := t.socket.Connect(Port)
		if err == nil {
			if err = WriteHeader(conn, id, stream); err != nil {
				return nil, errorsx.Compact(err, conn.Close())
			}
			return conn, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.stopped:
			return nil, errorsx.Compact(errorsx.New("the machine stopped"), t.err)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Close stops the machine and waits for it.
func (t *VM) Close() error {
	if t.vm.CanStop() {
		errorsx.Log(t.vm.Stop())
	}
	<-t.stopped
	t.close()
	return t.err
}

func (t *VM) close() {
	for _, c := range t.closers {
		c.Close()
	}
	// the framework holds its own handle on the console, so the copy may never see the end of it.
	select {
	case <-t.copied:
	case <-time.After(time.Second):
	}
}

func configure(m Machine, in *os.File, out *os.File) (_ *vz.VirtualMachineConfiguration, err error) {
	var (
		boot     *vz.LinuxBootLoader
		config   *vz.VirtualMachineConfiguration
		platform *vz.GenericPlatformConfiguration
		entropy  *vz.VirtioEntropyDeviceConfiguration
		console  *vz.FileHandleSerialPortAttachment
		serial   *vz.VirtioConsoleDeviceSerialPortConfiguration
		socket   *vz.VirtioSocketDeviceConfiguration
		nat      *vz.NATNetworkDeviceAttachment
		network  *vz.VirtioNetworkDeviceConfiguration
		mac      *vz.MACAddress
		shares   = make([]vz.DirectorySharingDeviceConfiguration, 0, len(m.Shares))
	)

	if boot, err = vz.NewLinuxBootLoader(m.Kernel, vz.WithInitrd(m.Initrd), vz.WithCommandLine(m.Cmdline)); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the boot loader")
	}
	if config, err = vz.NewVirtualMachineConfiguration(boot, m.CPUs, m.Memory); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine configuration")
	}
	if platform, err = vz.NewGenericPlatformConfiguration(); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the platform")
	}
	config.SetPlatformVirtualMachineConfiguration(platform)

	if entropy, err = vz.NewVirtioEntropyDeviceConfiguration(); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the entropy device")
	}
	config.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{entropy})

	if console, err = vz.NewFileHandleSerialPortAttachment(in, out); err != nil {
		return nil, errorsx.Wrap(err, "unable to attach the console")
	}
	if serial, err = vz.NewVirtioConsoleDeviceSerialPortConfiguration(console); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the console")
	}
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{serial})

	if socket, err = vz.NewVirtioSocketDeviceConfiguration(); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the socket device")
	}
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{socket})

	if nat, err = vz.NewNATNetworkDeviceAttachment(); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the network attachment")
	}
	if network, err = vz.NewVirtioNetworkDeviceConfiguration(nat); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the network device")
	}
	if mac, err = vz.NewRandomLocallyAdministeredMACAddress(); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the network device's address")
	}
	network.SetMACAddress(mac)
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{network})

	for _, s := range m.Shares {
		device, err := share(s)
		if err != nil {
			return nil, err
		}
		shares = append(shares, device)
	}
	config.SetDirectorySharingDevicesVirtualMachineConfiguration(shares)

	if _, err = config.Validate(); err != nil {
		return nil, errorsx.Wrap(err, "invalid machine configuration")
	}
	return config, nil
}

func share(s Share) (_ *vz.VirtioFileSystemDeviceConfiguration, err error) {
	var (
		dir    *vz.SharedDirectory
		single *vz.SingleDirectoryShare
		device *vz.VirtioFileSystemDeviceConfiguration
	)

	if dir, err = vz.NewSharedDirectory(s.Dir, false); err != nil {
		return nil, errorsx.Wrapf(err, "unable to share %s", s.Dir)
	}
	if single, err = vz.NewSingleDirectoryShare(dir); err != nil {
		return nil, errorsx.Wrapf(err, "unable to share %s", s.Dir)
	}
	if device, err = vz.NewVirtioFileSystemDeviceConfiguration(s.Tag); err != nil {
		return nil, errorsx.Wrapf(err, "unable to create the virtio-fs device %s", s.Tag)
	}
	device.SetDirectoryShare(single)
	return device, nil
}

// wait returns once the machine stops.
func wait(vm *vz.VirtualMachine) error {
	for state := range vm.StateChangedNotify() {
		switch state {
		case vz.VirtualMachineStateStopped:
			return nil
		case vz.VirtualMachineStateError:
			return errorsx.New("the machine stopped with an error")
		}
	}
	return nil
}
