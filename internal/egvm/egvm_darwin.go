package egvm

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/interp/execproxy"
	"golang.org/x/crypto/ssh"
)

// VM is a running machine.
type VM struct {
	vm      *vz.VirtualMachine
	socket  *vz.VirtioSocketDevice
	client  *ssh.ClientConfig
	stopped chan struct{}
	err     error
	closers []io.Closer
	copied  chan struct{}
}

// Boot starts the machine with Virtualization.framework, copying its console to console. The
// process needs the com.apple.security.virtualization entitlement.
func Boot(m Machine, console io.Writer) (_ *VM, err error) {
	var (
		config     *vz.VirtualMachineConfiguration
		vm         *vz.VirtualMachine
		nfs        *vz.VirtioSocketListener
		login      *ssh.ClientConfig
		authorized string
	)

	if login, authorized, err = client(m.root.Upper()); err != nil {
		return nil, err
	}

	// the console's input is a pipe nothing writes to; its output is copied to console.
	in, stdin, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdout, out, err := os.Pipe()
	if err != nil {
		return nil, errorsx.Compact(err, in.Close(), stdin.Close())
	}
	t := &VM{client: login, stopped: make(chan struct{}), copied: make(chan struct{}), closers: []io.Closer{in, stdin, out, stdout}}
	defer func() {
		if err != nil {
			t.close()
		}
	}()

	if config, err = configure(m, authorized, in, out); err != nil {
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
	go egiso.ServeNFS(nfs, m.root, m.nfs...)

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

// Exec runs req in the machine and waits for it, with no input and its output discarded unless
// options say otherwise. A command that exits with other than 0 returns an ExitError. It waits for
// the machine to serve commands. Cancelling ctx kills the command.
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, options ...ExecOption) (err error) {
	var (
		conn net.Conn
	)

	if conn, err = redial(ctx, func() (net.Conn, error) { return t.socket.Connect(22) }, t.stopped, func() error { return t.err }); err != nil {
		return err
	}
	return command(ctx, conn, t.client, req, options...)
}

// Forward makes the host's unix socket host reachable in the machine at path, a socket the machine's
// sshd listens on until the machine stops. It waits for the machine to serve commands.
func (t *VM) Forward(ctx context.Context, host string, path string) (err error) {
	var (
		conn      net.Conn
		forwarded io.Closer
	)

	if conn, err = redial(ctx, func() (net.Conn, error) { return t.socket.Connect(22) }, t.stopped, func() error { return t.err }); err != nil {
		return err
	}
	if forwarded, err = forward(conn, t.client, host, path); err != nil {
		return err
	}
	t.closers = append(t.closers, forwarded)
	return nil
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

func configure(m Machine, authorized string, in *os.File, out *os.File) (_ *vz.VirtualMachineConfiguration, err error) {
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
		device   *vz.VirtioFileSystemDeviceConfiguration
		devices  = make([]vz.DirectorySharingDeviceConfiguration, 0, len(m.mounts))
	)

	if boot, err = vz.NewLinuxBootLoader(filepath.Join(m.dir, "Image"), vz.WithInitrd(filepath.Join(m.dir, "initramfs")), vz.WithCommandLine(m.cmdline(authorized))); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the boot loader")
	}
	// a virtual machine reserves its memory up front, so one sized as the host is given 4GiB.
	if config, err = vz.NewVirtualMachineConfiguration(boot, m.cpus, langx.FirstNonZero(m.memory, 4<<30)); err != nil {
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

	for _, mnt := range m.mounts {
		if device, err = share(mnt); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	config.SetDirectorySharingDevicesVirtualMachineConfiguration(devices)

	if _, err = config.Validate(); err != nil {
		return nil, errorsx.Wrap(err, "invalid machine configuration")
	}
	return config, nil
}

func share(mnt mount) (_ *vz.VirtioFileSystemDeviceConfiguration, err error) {
	var (
		dir    *vz.SharedDirectory
		single *vz.SingleDirectoryShare
		device *vz.VirtioFileSystemDeviceConfiguration
	)

	if dir, err = vz.NewSharedDirectory(mnt.dir, mnt.readonly); err != nil {
		return nil, errorsx.Wrapf(err, "unable to share %s", mnt.dir)
	}
	if single, err = vz.NewSingleDirectoryShare(dir); err != nil {
		return nil, errorsx.Wrapf(err, "unable to share %s", mnt.dir)
	}
	if device, err = vz.NewVirtioFileSystemDeviceConfiguration(mnt.tag); err != nil {
		return nil, errorsx.Wrapf(err, "unable to create the virtio-fs device %s", mnt.tag)
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
