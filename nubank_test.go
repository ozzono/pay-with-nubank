package nubank

import (
	"encoding/json"
	"io/ioutil"
	"testing"
)

func TestFlow(t *testing.T) {
	config, err := readFile("config_test.json")
	if err != nil {
		t.Fatal(err)
	}
	flow, err := NewFlow(Invoice{
		BarCode: config["BarCode"],
		Value:   config["Value"],
		DueDate: config["DueDate"],
	}, config["appPW"])
	if err != nil {
		t.Fatal(err)
	}

	err = flow.PayInvoice()
	if err != nil {
		t.Fatal(err)
	}
}

func readFile(filename string) (map[string]string, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return map[string]string{}, err
	}
	output := map[string]string{}
	err = json.Unmarshal(file, &output)
	if err != nil {
		return map[string]string{}, err
	}

	return output, nil
}
