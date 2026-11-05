package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 4. Cookies парсер

type Cookie struct {
	Domain string
	Name   string
	Value  string
}

func ParseCookiesFile(path string) ([]Cookie, error) {
	p := ParseUserPath(path)
	file, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cookies []Cookie
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 7 {
			cookies = append(cookies, Cookie{
				Domain: parts[0],
				Name:   parts[5],
				Value:  parts[6],
			})
		}
	}
	return cookies, scanner.Err()
}

func AppendCookie(filePath, domain, name, val string) error {
	p := ParseUserPath(filePath)
	_ = os.MkdirAll(filepath.Dir(p), 0755)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	exp := strconv.FormatInt(time.Now().Unix()+3600*24*365, 10)
	flag := "FALSE"
	if strings.HasPrefix(domain, ".") {
		flag = "TRUE"
	}
	line := fmt.Sprintf("%s\t%s\t/\tFALSE\t%s\t%s\t%s\n", domain, flag, exp, name, val)
	_, err = f.WriteString(line)
	return err
}
