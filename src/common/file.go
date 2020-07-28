package common

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
)

func LoadJson(filename string, conf interface{}) error {
	err := loadJson(filename, conf)
	if err != nil {
		err := loadJson(filename+".back", conf)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadJson(filename string, conf interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(conf)
	if err != nil {
		return err
	}
	return nil
}

func SaveJson(filename string, conf interface{}) error {
	err1 := saveJson(filename, conf)
	err2 := saveJson(filename+".back", conf)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func saveJson(filename string, conf interface{}) error {
	str, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}
	jsonFile, err := os.OpenFile(filename,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	jsonFile.Write(str)
	jsonFile.Close()
	return nil
}

func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func FileMd5(filename string) (string, error) {

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	md5 := GetMd5String(string(data))
	return md5, nil
}
