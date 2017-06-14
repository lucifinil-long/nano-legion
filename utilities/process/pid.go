package process

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

func SavePid(pidFile string) error {
	dir := path.Dir(pidFile)
	os.MkdirAll(dir, 0744)

	pid := os.Getpid()
	pidString := strconv.Itoa(pid)
	if err := ioutil.WriteFile(pidFile, []byte(pidString), 0777); err != nil {
		return err
	}
	return nil
}
