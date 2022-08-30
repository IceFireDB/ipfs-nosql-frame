/*
 * @Author: gitsrc
 * @Date: 2020-07-09 10:20:40
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-10 09:27:52
 * @FilePath: /ServiceCar/sc/confer/yamlParser.go
 */

package confer

import (
	"errors"

	"github.com/gitsrc/ipfs-nosql-frame/utils/filer"

	"gopkg.in/yaml.v2"
)

// parseYamlFromBytes data source is []byte
func parseYamlFromBytes(orignData []byte) (data scConfS, err error) {

	if len(orignData) == 0 {
		err = errors.New("yaml source data is empty")
		return
	}

	err = yaml.Unmarshal(orignData, &data)

	return
}

// parseYamlFromFile data source is file path.
func parseYamlFromFile(yamlFileURI string) (data scConfS, err error) {

	var fileData []byte

	fileData, err = filer.ReadFileData(yamlFileURI)

	if err != nil {
		return
	}

	data, err = parseYamlFromBytes(fileData)

	return
}
