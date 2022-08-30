/*
 * @Author: gitsrc
 * @Date: 2020-07-09 10:31:15
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-09 10:33:44
 * @FilePath: /ServiceCar/utils/filer/reader.go
 */

package filer

import (
	"errors"
	"io/ioutil"
	"os"
)

// ReadFileData is file all data reader
func ReadFileData(FileURI string) (fileData []byte, err error) {

	if _, err = os.Stat(FileURI); os.IsNotExist(err) {

		err = errors.New("The yaml configuration file does not exist")

	}

	fileData, err = ioutil.ReadFile(FileURI)

	return
}
