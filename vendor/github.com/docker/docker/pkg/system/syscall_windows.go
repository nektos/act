package system // import "github.com/docker/docker/pkg/system"

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	OWNER_SECURITY_INFORMATION               = 0x00000001
	GROUP_SECURITY_INFORMATION               = 0x00000002
	DACL_SECURITY_INFORMATION                = 0x00000004
	SACL_SECURITY_INFORMATION                = 0x00000008
	LABEL_SECURITY_INFORMATION               = 0x00000010
	ATTRIBUTE_SECURITY_INFORMATION           = 0x00000020
	SCOPE_SECURITY_INFORMATION               = 0x00000040
	PROCESS_TRUST_LABEL_SECURITY_INFORMATION = 0x00000080
	ACCESS_FILTER_SECURITY_INFORMATION       = 0x00000100
	BACKUP_SECURITY_INFORMATION              = 0x00010000
	PROTECTED_DACL_SECURITY_INFORMATION      = 0x80000000
	PROTECTED_SACL_SECURITY_INFORMATION      = 0x40000000
	UNPROTECTED_DACL_SECURITY_INFORMATION    = 0x20000000
	UNPROTECTED_SACL_SECURITY_INFORMATION    = 0x10000000
)

const (
	SE_UNKNOWN_OBJECT_TYPE = iota
	SE_FILE_OBJECT
	SE_SERVICE
	SE_PRINTER
	SE_REGISTRY_KEY
	SE_LMSHARE
	SE_KERNEL_OBJECT
	SE_WINDOW_OBJECT
	SE_DS_OBJECT
	SE_DS_OBJECT_ALL
	SE_PROVIDER_DEFINED_OBJECT
	SE_WMIGUID_OBJECT
	SE_REGISTRY_WOW64_32KEY
)

const (
	SeTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"
)

const (
	ContainerAdministratorSidString = "S-1-5-93-2-1"
	ContainerUserSidString          = "S-1-5-93-2-2"
)

var (
	ntuserApiset                  = windows.NewLazyDLL("ext-ms-win-ntuser-window-l1-1-0")
	modadvapi32                   = windows.NewLazySystemDLL("advapi32.dll")
	procGetVersionExW             = modkernel32.NewProc("GetVersionExW")
	procGetProductInfo            = modkernel32.NewProc("GetProductInfo")
	procSetNamedSecurityInfo      = modadvapi32.NewProc("SetNamedSecurityInfoW")
	procGetSecurityDescriptorDacl = modadvapi32.NewProc("GetSecurityDescriptorDacl")
)

// OSVersion is a wrapper for Windows version information
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724439(v=vs.85).aspx
type OSVersion struct {
	Version      uint32
	MajorVersion uint8
	MinorVersion uint8
	Build        uint16
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724833(v=vs.85).aspx
type osVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserve           byte
}

// GetOSVersion gets the operating system version on Windows. Note that
// docker.exe must be manifested to get the correct version information.
func GetOSVersion() OSVersion {
	var err error
	osv := OSVersion{}
	osv.Version, err = windows.GetVersion()
	if err != nil {
		// GetVersion never fails.
		panic(err)
	}
	osv.MajorVersion = uint8(osv.Version & 0xFF)
	osv.MinorVersion = uint8(osv.Version >> 8 & 0xFF)
	osv.Build = uint16(osv.Version >> 16)
	return osv
}

func (osv OSVersion) ToString() string {
	return fmt.Sprintf("%d.%d.%d", osv.MajorVersion, osv.MinorVersion, osv.Build)
}

// IsWindowsClient returns true if the SKU is client
// @engine maintainers - this function should not be removed or modified as it
// is used to enforce licensing restrictions on Windows.
func IsWindowsClient() bool {
	osviex := &osVersionInfoEx{OSVersionInfoSize: 284}
	r1, _, err := procGetVersionExW.Call(uintptr(unsafe.Pointer(osviex)))
	if r1 == 0 {
		logrus.Warnf("GetVersionExW failed - assuming server SKU: %v", err)
		return false
	}
	const verNTWorkstation = 0x00000001
	return osviex.ProductType == verNTWorkstation
}

// IsIoTCore returns true if the currently running image is based off of
// Windows 10 IoT Core.
// @engine maintainers - this function should not be removed or modified as it
// is used to enforce licensing restrictions on Windows.
func IsIoTCore() bool {
	var returnedProductType uint32
	r1, _, err := procGetProductInfo.Call(6, 1, 0, 0, uintptr(unsafe.Pointer(&returnedProductType)))
	if r1 == 0 {
		logrus.Warnf("GetProductInfo failed - assuming this is not IoT: %v", err)
		return false
	}
	const productIoTUAP = 0x0000007B
	const productIoTUAPCommercial = 0x00000083
	return returnedProductType == productIoTUAP || returnedProductType == productIoTUAPCommercial
}

// Unmount is a platform-specific helper function to call
// the unmount syscall. Not supported on Windows
func Unmount(dest string) error {
	return nil
}

// CommandLineToArgv wraps the Windows syscall to turn a commandline into an argument array.
func CommandLineToArgv(commandLine string) ([]string, error) {
	var argc int32

	argsPtr, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return nil, err
	}

	argv, err := windows.CommandLineToArgv(argsPtr, &argc)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(argv))))

	newArgs := make([]string, argc)
	for i, v := range (*argv)[:argc] {
		newArgs[i] = string(windows.UTF16ToString((*v)[:]))
	}

	return newArgs, nil
}

// HasWin32KSupport determines whether containers that depend on win32k can
// run on this machine. Win32k is the driver used to implement windowing.
func HasWin32KSupport() bool {
	// For now, check for ntuser API support on the host. In the future, a host
	// may support win32k in containers even if the host does not support ntuser
	// APIs.
	return ntuserApiset.Load() == nil
}

func SetNamedSecurityInfo(objectName *uint16, objectType uint32, securityInformation uint32, sidOwner *windows.SID, sidGroup *windows.SID, dacl *byte, sacl *byte) (result error) {
	r0, _, _ := syscall.Syscall9(procSetNamedSecurityInfo.Addr(), 7, uintptr(unsafe.Pointer(objectName)), uintptr(objectType), uintptr(securityInformation), uintptr(unsafe.Pointer(sidOwner)), uintptr(unsafe.Pointer(sidGroup)), uintptr(unsafe.Pointer(dacl)), uintptr(unsafe.Pointer(sacl)), 0, 0)
	if r0 != 0 {
		result = syscall.Errno(r0)
	}
	return
}

func GetSecurityDescriptorDacl(securityDescriptor *byte, daclPresent *uint32, dacl **byte, daclDefaulted *uint32) (result error) {
	r1, _, e1 := syscall.Syscall6(procGetSecurityDescriptorDacl.Addr(), 4, uintptr(unsafe.Pointer(securityDescriptor)), uintptr(unsafe.Pointer(daclPresent)), uintptr(unsafe.Pointer(dacl)), uintptr(unsafe.Pointer(daclDefaulted)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			result = syscall.Errno(e1)
		} else {
			result = syscall.EINVAL
		}
	}
	return
}
