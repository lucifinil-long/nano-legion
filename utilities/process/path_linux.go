package process

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

/*
 * 获取二进制文件绝对目录
 @return (absolute path, nil)表示成功;否则返回("", error)
*/
func GetProcessBinaryDir() (string, error) {
	var dir, p string
	var err error
	pid := os.Getpid()
	lnk := "/proc/" + strconv.Itoa(pid) + "/exe"
	p, err = os.Readlink(lnk)
	if err != nil {
		return "", err
	}
	dir = filepath.Dir(p)
	dir = strings.Replace(dir, "\\", "/", -1)
	return dir, nil
}

/*
 * 获取二进制文件绝对目录
 @return (absolute path, nil)表示成功;否则返回("", error)
*/
func GetProcessBinaryDir() (string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	return dir, err
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
