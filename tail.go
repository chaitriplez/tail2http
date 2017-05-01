package tail2http

import (
	"bufio"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type DataFile struct {
	Path    string
	Name    string
	ModTime int64
}

type MonitorFile struct {
	Path    string
	Name    string
	ModTime int64
	Size    int64

	position int64
	file     *os.File
	sc       *bufio.Scanner
	open     bool
}

func ListDataFiles(path string) (map[string]*DataFile, error) {
	fs, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*DataFile)
	for _, f := range fs {
		d, err := toDataFile(path, f)
		if err != nil {
			return nil, err
		}
		result[d.Name] = d
	}
	return result, nil
}

func ListMonitorFiles(monitorPath, filePattern string) (map[string]*MonitorFile, error) {
	reg, err1 := regexp.Compile(filePattern)
	if err1 != nil {
		return nil, err1
	}

	fs, err := ioutil.ReadDir(monitorPath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*MonitorFile)
	for _, f := range fs {
		if !f.Mode().IsDir() && f.Mode().IsRegular() && reg.MatchString(f.Name()) {
			result[f.Name()] = toMonitorFile(monitorPath, f)
		}
	}
	return result, nil
}

func (d *DataFile) Position() (int64, error) {
	fullPath := filepath.Join(d.Path, d.Name)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return 0, nil
	}

	b, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return 0, err
	}

	strPos := strings.Trim(string(b), "\n")
	p, err := strconv.ParseInt(strPos, 10, 64)
	if err != nil {
		return 0, err
	}
	return p, nil
}

func (d *DataFile) SaveToFile(position int64) error {
	s := strconv.FormatInt(position, 10)
	fullPath := filepath.Join(d.Path, d.Name)
	err := ioutil.WriteFile(fullPath, []byte(s+"\n"), os.FileMode(0644))
	if err != nil {
		return err
	}

	fi, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}

	d.ModTime = fi.ModTime().Unix()
	return nil
}

func (m *MonitorFile) Position() int64 {
	return m.position
}

func (m *MonitorFile) Open(offset int64) error {
	if m.open {
		return errors.New("File already open: " + m.Name)
	}

	fullPath := filepath.Join(m.Path, m.Name)
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}

	pos, err := f.Seek(offset, os.SEEK_SET)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(f)
	sf := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanLines(data, atEOF)
		m.position += int64(advance)
		return
	}
	sc.Split(sf)

	m.file = f
	m.position = pos
	m.sc = sc
	m.open = true
	return nil
}

func (m *MonitorFile) Close() error {
	if !m.open {
		return errors.New("File already close: " + m.Name)
	}

	err := m.file.Close()
	if err != nil {
		return err
	}
	m.file = nil
	m.position = 0
	m.sc = nil
	m.open = false
	return nil
}

func (m *MonitorFile) Scan() bool {
	if !m.open {
		return false
	}
	return m.sc.Scan()
}

func (m *MonitorFile) Text() string {
	if !m.open {
		return ""
	}
	return m.sc.Text()
}

func toDataFile(path string, fi os.FileInfo) (*DataFile, error) {
	return &DataFile{
		Path:    path,
		Name:    fi.Name(),
		ModTime: fi.ModTime().Unix(),
	}, nil
}

func toMonitorFile(path string, fi os.FileInfo) *MonitorFile {
	return &MonitorFile{
		Path:    path,
		Name:    fi.Name(),
		Size:    fi.Size(),
		ModTime: fi.ModTime().Unix(),
	}
}
