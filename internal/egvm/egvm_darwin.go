package egvm

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/egdaemon/eg/internal/errorsx"
)

// Run boots the machine with Virtualization.framework, copying its console to console, and waits
// for it to stop. Cancelling ctx stops it. The process needs the com.apple.security.virtualization
// entitlement.
func Run(ctx context.Context, m Machine, console io.Writer) (err error) {
	var (
		config *vz.VirtualMachineConfiguration
		vm     *vz.VirtualMachine
	)

	// the console's input is a pipe nothing writes to; its output is copied to console.
	in, stdin, err := os.Pipe()
	if err != nil {
		return err
	}
	defer in.Close()
	defer stdin.Close()
	stdout, out, err := os.Pipe()
	if err != nil {
		return err
	}
	defer stdout.Close()
	defer out.Close()

	if config, err = configure(m, in, out); err != nil {
		return err
	}
	if vm, err = vz.NewVirtualMachine(config); err != nil {
		return errorsx.Wrap(err, "unable to create the machine")
	}

	copied := make(chan struct{})
	go func() {
		defer close(copied)
		_, cause := io.Copy(console, stdout)
		errorsx.Log(errorsx.Ignore(cause, os.ErrClosed))
	}()

	if err = vm.Start(); err != nil {
		return errorsx.Wrap(err, "unable to start the machine")
	}
	err = wait(ctx, vm)

	// the framework holds its own handle on the console, so the copy may never see the end of it.
	out.Close()
	select {
	case <-copied:
	case <-time.After(time.Second):
	}
	return err
}

func configure(m Machine, in *os.File, out *os.File) (_ *vz.VirtualMachineConfiguration, err error) {
	var (
		boot     *vz.LinuxBootLoader
		config   *vz.VirtualMachineConfiguration
		platform *vz.GenericPlatformConfiguration
		entropy  *vz.VirtioEntropyDeviceConfiguration
		console  *vz.FileHandleSerialPortAttachment
		serial   *vz.VirtioConsoleDeviceSerialPortConfiguration
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

// wait returns once the machine stops, stopping it when ctx is done.
func wait(ctx context.Context, vm *vz.VirtualMachine) error {
	for {
		select {
		case <-ctx.Done():
			errorsx.Log(vm.Stop())
			return ctx.Err()
		case state := <-vm.StateChangedNotify():
			switch state {
			case vz.VirtualMachineStateStopped:
				return nil
			case vz.VirtualMachineStateError:
				return errorsx.New("the machine stopped with an error")
			}
		}
	}
}
