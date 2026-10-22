// Package spooler lists printers and clears stuck jobs from the Windows print queue.
package spooler

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	winspool              = windows.NewLazySystemDLL("winspool.drv")
	procEnumPrinters      = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinter = winspool.NewProc("GetDefaultPrinterW")
)

type Printer struct {
	Name    string
	Jobs    int
	Default bool
}

// printerInfo2 mirrors PRINTER_INFO_2W.
type printerInfo2 struct {
	serverName, printerName, shareName, portName, driverName, comment, location *uint16
	devMode                                                                     uintptr
	sepFile, printProcessor, datatype, parameters                               *uint16
	securityDescriptor                                                          uintptr
	attributes, priority, defaultPriority, startTime, untilTime, status, jobs   uint32
	averagePPM                                                                  uint32
}

// List returns the local and connected printers with how many jobs each has waiting.
func List() ([]Printer, error) {
	const flags = 0x2 | 0x4 // PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS
	var needed, count uint32
	procEnumPrinters.Call(flags, 0, 2, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if needed == 0 {
		return nil, nil
	}
	buf := make([]byte, needed)
	r, _, err := procEnumPrinters.Call(flags, 0, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if r == 0 {
		return nil, err
	}

	def := defaultPrinter()
	list := make([]Printer, 0, count)
	for _, pi := range unsafe.Slice((*printerInfo2)(unsafe.Pointer(&buf[0])), count) {
		name := windows.UTF16PtrToString(pi.printerName)
		list = append(list, Printer{Name: name, Jobs: int(pi.jobs), Default: name == def})
	}
	return list, nil
}

func defaultPrinter() string {
	n := uint32(256)
	buf := make([]uint16, n)
	if r, _, _ := procGetDefaultPrinter.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// PrintTestPage sends the standard Windows test page to the printer.
func PrintTestPage(name string) error {
	return exec.Command("rundll32", "printui.dll,PrintUIEntry", "/k", "/n", name).Start()
}

// Clear stops the Print Spooler, deletes every queued job file and starts it again.
// This is the fix for jobs stuck at "Deleting" that block everything behind them. Needs admin.
func Clear() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService("Spooler")
	if err != nil {
		return err
	}
	defer s.Close()

	// Services like Fax block the spooler from stopping; stop them first, restart them after.
	names, err := s.ListDependentServices(svc.Active)
	if err != nil {
		return err
	}
	for _, name := range names {
		d, err := m.OpenService(name)
		if err != nil {
			return err
		}
		defer d.Close()
		defer d.Start() // deferred, so this runs after the spooler is back up
		if _, err := stop(d); err != nil {
			return err
		}
	}

	st, err := stop(s)
	if err == nil && st.State != svc.Stopped {
		// A badly stuck job can hang the spooler. spoolsv.exe hosts only this service, so killing it is safe.
		if err = kill(st.ProcessId); err == nil {
			st, err = waitFor(s, svc.Stopped, 10*time.Second)
		}
	}
	if err == nil && st.State != svc.Stopped {
		err = errors.New("Print Spooler did not stop")
	}
	if err == nil {
		err = clearDir(spoolDir())
	}

	// Always bring the spooler back, even if something above failed, or nothing can print.
	if startErr := start(s); err == nil {
		err = startErr
	}
	return err
}

func stop(s *mgr.Service) (svc.Status, error) {
	if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return svc.Status{}, err
	}
	return waitFor(s, svc.Stopped, 15*time.Second)
}

func start(s *mgr.Service) error {
	if err := s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	st, err := waitFor(s, svc.Running, 15*time.Second)
	if err == nil && st.State != svc.Running {
		err = errors.New("Print Spooler did not start")
	}
	return err
}

func waitFor(s *mgr.Service, want svc.State, timeout time.Duration) (svc.Status, error) {
	deadline := time.Now().Add(timeout)
	for {
		st, err := s.Query()
		if err != nil || st.State == want || time.Now().After(deadline) {
			return st, err
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func kill(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

func spoolDir() string {
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Print\Printers`, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if dir, _, err := k.GetStringValue("DefaultSpoolDirectory"); err == nil && dir != "" {
			return dir
		}
	}
	return filepath.Join(os.Getenv("SystemRoot"), `System32\spool\PRINTERS`)
}

// clearDir deletes the job files (.SPL/.SHD) directly inside dir and leaves subfolders alone.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			errs = append(errs, os.Remove(filepath.Join(dir, e.Name())))
		}
	}
	return errors.Join(errs...)
}
