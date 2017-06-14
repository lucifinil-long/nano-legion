package process

import (
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

/*
 * 获取二进制文件绝对目录
 @return (absolute path, nil)表示成功;否则返回("", error)
*/
func GetProcessBinaryDir() (string, error) {
	var dir, p string
	var err error
	p, err = getWindowsProcessBinaryPath()
	if err != nil {
		return "", err
	}
	dir = filepath.Dir(p)
	dir = strings.Replace(dir, "\\", "/", -1)
	return dir, nil
}

/*
 * 通常我们按照下面的结构部署项目
 * root
 *   |___bin		// bin目录存放二进制组件文件
 *   |___log		// log目录存放日志
 *   |___data		// data目录存放本地数据
 *   |___tmp		// tmp目录存放临时文件
 *   ...
 * 本函数依据此结构获取root目录
 * @return 获取到的root目录
 * @exception 如果获取二进制所在目录失败会产生panic
 */
func GetProjectRootDir() string {
	binDir, err := GetProcessBinaryDir()
	if err != nil {
		panic(err.Error())
	}
	return binDir + "/.."
}

func getWindowsProcessBinaryPath() (string, error) {
	b := make([]uint16, 300)
	n, e := getModuleFileName(uint32(len(b)), &b[0])
	if e != nil {
		return "", e
	}
	return string(utf16.Decode(b[0:n])), nil
}

func getModuleFileName(buflen uint32, buf *uint16) (n uint32, err error) {
	h, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		return 0, err
	}
	defer syscall.FreeLibrary(h)
	addr, err := syscall.GetProcAddress(h, "GetModuleFileNameW")
	if err != nil {
		return 0, err
	}
	r0, _, e1 := syscall.Syscall(addr, 3, uintptr(0), uintptr(unsafe.Pointer(buf)), uintptr(buflen))
	n = uint32(r0)
	if n == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
